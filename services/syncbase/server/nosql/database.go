// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package nosql

import (
	"math/rand"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"v.io/v23/context"
	"v.io/v23/glob"
	"v.io/v23/rpc"
	"v.io/v23/security/access"
	wire "v.io/v23/services/syncbase/nosql"
	"v.io/v23/syncbase/nosql/query_db"
	"v.io/v23/syncbase/nosql/query_exec"
	"v.io/v23/vdl"
	"v.io/v23/verror"
	"v.io/v23/vom"
	"v.io/x/lib/vlog"
	"v.io/x/ref/services/syncbase/clock"
	"v.io/x/ref/services/syncbase/server/interfaces"
	"v.io/x/ref/services/syncbase/server/util"
	"v.io/x/ref/services/syncbase/server/watchable"
	"v.io/x/ref/services/syncbase/store"
)

// database is a per-database singleton (i.e. not per-request). It does not
// directly handle RPCs.
// Note: If a database does not exist at the time of a database RPC, the
// dispatcher creates a short-lived database object to service that particular
// request.
type database struct {
	name string
	a    interfaces.App
	// The fields below are initialized iff this database exists.
	exists bool
	// TODO(sadovsky): Make st point to a store.Store wrapper that handles paging,
	// and do not actually open the store in NewDatabase.
	st store.Store // stores all data for a single database

	// Active snapshots and transactions corresponding to client batches.
	// TODO(sadovsky): Add timeouts and GC.
	mu  sync.Mutex // protects the fields below
	sns map[uint64]store.Snapshot
	txs map[uint64]store.Transaction

	// Active ConflictResolver connection from the app to this database.
	// NOTE: For now, we assume there's only one open conflict resolution stream
	// per database (typically, from the app that owns the database).
	resolver wire.ConflictManagerStartConflictResolverServerCall
}

// databaseReq is a per-request object that handles Database RPCs.
// It embeds database and tracks request-specific batch state.
type databaseReq struct {
	*database
	// If non-nil, sn or tx will be non-nil.
	batchId *uint64
	sn      store.Snapshot
	tx      store.Transaction
}

var (
	_ wire.DatabaseServerMethods = (*databaseReq)(nil)
	_ interfaces.Database        = (*database)(nil)
)

// DatabaseOptions configures a database.
type DatabaseOptions struct {
	// Database-level permissions.
	Perms access.Permissions
	// Root dir for data storage.
	RootDir string
	// Storage engine to use.
	Engine string
}

// OpenDatabase opens a database and returns a *database for it. Designed for
// use from within NewDatabase and server.NewService.
func OpenDatabase(ctx *context.T, a interfaces.App, name string, opts DatabaseOptions, openOpts util.OpenOptions) (*database, error) {
	st, err := util.OpenStore(opts.Engine, path.Join(opts.RootDir, opts.Engine), openOpts)
	if err != nil {
		return nil, err
	}
	vclock := clock.NewVClock(a.Service().St())
	st, err = watchable.Wrap(st, vclock, &watchable.Options{
		ManagedPrefixes: []string{util.RowPrefix, util.PermsPrefix},
	})
	if err != nil {
		return nil, err
	}
	return &database{
		name:   name,
		a:      a,
		exists: true,
		st:     st,
		sns:    make(map[uint64]store.Snapshot),
		txs:    make(map[uint64]store.Transaction),
	}, nil
}

// NewDatabase creates a new database instance and returns it.
// Designed for use from within App.CreateNoSQLDatabase.
func NewDatabase(ctx *context.T, a interfaces.App, name string, metadata *wire.SchemaMetadata, opts DatabaseOptions) (*database, error) {
	if opts.Perms == nil {
		return nil, verror.New(verror.ErrInternal, ctx, "perms must be specified")
	}
	d, err := OpenDatabase(ctx, a, name, opts, util.OpenOptions{CreateIfMissing: true, ErrorIfExists: true})
	if err != nil {
		return nil, err
	}
	data := &databaseData{
		Name:           d.name,
		Perms:          opts.Perms,
		SchemaMetadata: metadata,
	}
	if err := util.Put(ctx, d.st, d.stKey(), data); err != nil {
		return nil, err
	}
	return d, nil
}

////////////////////////////////////////
// RPC methods

func (d *databaseReq) Create(ctx *context.T, call rpc.ServerCall, metadata *wire.SchemaMetadata, perms access.Permissions) error {
	if d.exists {
		return verror.New(verror.ErrExist, ctx, d.name)
	}
	if d.batchId != nil {
		return wire.NewErrBoundToBatch(ctx)
	}
	// This database does not yet exist; d is just an ephemeral handle that holds
	// {name string, a *app}. d.a.CreateNoSQLDatabase will create a new database
	// handle and store it in d.a.dbs[d.name].
	return d.a.CreateNoSQLDatabase(ctx, call, d.name, perms, metadata)
}

func (d *databaseReq) Destroy(ctx *context.T, call rpc.ServerCall, schemaVersion int32) error {
	if d.batchId != nil {
		return wire.NewErrBoundToBatch(ctx)
	}
	if err := d.checkSchemaVersion(ctx, schemaVersion); err != nil {
		return err
	}
	return d.a.DestroyNoSQLDatabase(ctx, call, d.name)
}

func (d *databaseReq) Exists(ctx *context.T, call rpc.ServerCall, schemaVersion int32) (bool, error) {
	if !d.exists {
		return false, nil
	}
	if err := d.checkSchemaVersion(ctx, schemaVersion); err != nil {
		return false, err
	}
	return util.ErrorToExists(util.GetWithAuth(ctx, call, d.st, d.stKey(), &databaseData{}))
}

var rng *rand.Rand = rand.New(rand.NewSource(time.Now().UTC().UnixNano()))

func (d *databaseReq) BeginBatch(ctx *context.T, call rpc.ServerCall, schemaVersion int32, bo wire.BatchOptions) (string, error) {
	if !d.exists {
		return "", verror.New(verror.ErrNoExist, ctx, d.name)
	}
	if d.batchId != nil {
		return "", wire.NewErrBoundToBatch(ctx)
	}
	if err := d.checkSchemaVersion(ctx, schemaVersion); err != nil {
		return "", err
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	var id uint64
	var batchType string
	for {
		id = uint64(rng.Int63())
		if bo.ReadOnly {
			if _, ok := d.sns[id]; !ok {
				d.sns[id] = d.st.NewSnapshot()
				batchType = "sn"
				break
			}
		} else {
			if _, ok := d.txs[id]; !ok {
				d.txs[id] = d.st.NewTransaction()
				batchType = "tx"
				break
			}
		}
	}
	return strings.Join([]string{d.name, batchType, strconv.FormatUint(id, 10)}, util.BatchSep), nil
}

func (d *databaseReq) Commit(ctx *context.T, call rpc.ServerCall, schemaVersion int32) error {
	if !d.exists {
		return verror.New(verror.ErrNoExist, ctx, d.name)
	}
	if d.batchId == nil {
		return wire.NewErrNotBoundToBatch(ctx)
	}
	if d.tx == nil {
		return wire.NewErrReadOnlyBatch(ctx)
	}
	if err := d.checkSchemaVersion(ctx, schemaVersion); err != nil {
		return err
	}
	var err error
	if err = d.tx.Commit(); err == nil {
		d.mu.Lock()
		delete(d.txs, *d.batchId)
		d.mu.Unlock()
	}
	if verror.ErrorID(err) == store.ErrConcurrentTransaction.ID {
		return verror.New(wire.ErrConcurrentBatch, ctx, err)
	}
	return err
}

func (d *databaseReq) Abort(ctx *context.T, call rpc.ServerCall, schemaVersion int32) error {
	if !d.exists {
		return verror.New(verror.ErrNoExist, ctx, d.name)
	}
	if d.batchId == nil {
		return wire.NewErrNotBoundToBatch(ctx)
	}
	if err := d.checkSchemaVersion(ctx, schemaVersion); err != nil {
		return err
	}
	var err error
	if d.tx != nil {
		if err = d.tx.Abort(); err == nil {
			d.mu.Lock()
			delete(d.txs, *d.batchId)
			d.mu.Unlock()
		}
	} else {
		if err = d.sn.Abort(); err == nil {
			d.mu.Lock()
			delete(d.sns, *d.batchId)
			d.mu.Unlock()
		}
	}
	return err
}

func (d *databaseReq) Exec(ctx *context.T, call wire.DatabaseExecServerCall, schemaVersion int32, q string) error {
	if err := d.checkSchemaVersion(ctx, schemaVersion); err != nil {
		return err
	}
	impl := func(headers []string, rs ResultStream, err error) error {
		if err != nil {
			return err
		}
		sender := call.SendStream()
		// Push the headers first -- the client will retrieve them and return
		// them separately from the results.
		var resultHeaders []*vdl.Value
		for _, header := range headers {
			resultHeaders = append(resultHeaders, vdl.ValueOf(header))
		}
		sender.Send(resultHeaders)
		for rs.Advance() {
			result := rs.Result()
			if err := sender.Send(result); err != nil {
				rs.Cancel()
				return err
			}
		}
		return rs.Err()
	}
	var sntx store.SnapshotOrTransaction
	if d.batchId != nil {
		sntx = d.batchReader()
	} else {
		sntx = d.st.NewSnapshot()
		defer sntx.Abort()
	}
	// queryDb implements query_db.Database
	// which is needed by the query package's
	// Exec function.
	db := &queryDb{
		ctx:  ctx,
		call: call,
		req:  d,
		sntx: sntx,
	}

	return impl(query_exec.Exec(db, q))
}

func (d *databaseReq) SetPermissions(ctx *context.T, call rpc.ServerCall, perms access.Permissions, version string) error {
	if !d.exists {
		return verror.New(verror.ErrNoExist, ctx, d.name)
	}
	if d.batchId != nil {
		return wire.NewErrBoundToBatch(ctx)
	}
	return d.a.SetDatabasePerms(ctx, call, d.name, perms, version)
}

func (d *databaseReq) GetPermissions(ctx *context.T, call rpc.ServerCall) (perms access.Permissions, version string, err error) {
	if !d.exists {
		return nil, "", verror.New(verror.ErrNoExist, ctx, d.name)
	}
	if d.batchId != nil {
		return nil, "", wire.NewErrBoundToBatch(ctx)
	}
	data := &databaseData{}
	if err := util.GetWithAuth(ctx, call, d.st, d.stKey(), data); err != nil {
		return nil, "", err
	}
	return data.Perms, util.FormatVersion(data.Version), nil
}

func (d *databaseReq) GlobChildren__(ctx *context.T, call rpc.GlobChildrenServerCall, matcher *glob.Element) error {
	if !d.exists {
		return verror.New(verror.ErrNoExist, ctx, d.name)
	}
	if d.batchId != nil {
		return wire.NewErrBoundToBatch(ctx)
	}
	// Check perms.
	sn := d.st.NewSnapshot()
	if err := util.GetWithAuth(ctx, call, sn, d.stKey(), &databaseData{}); err != nil {
		sn.Abort()
		return err
	}
	return util.Glob(ctx, call, matcher, sn, sn.Abort, util.TablePrefix)
}

////////////////////////////////////////
// ResultStream interface

// ResultStream is an interface for iterating through results (a.k.a, rows) returned from a
// query.  Each resulting rows are arrays of vdl objects.
type ResultStream interface {
	// Advance stages an element so the client can retrieve it with Result.
	// Advance returns true iff there is a result to retrieve. The client must
	// call Advance before calling Result. The client must call Cancel if it
	// does not iterate through all elements (i.e. until Advance returns false).
	// Advance may block if an element is not immediately available.
	Advance() bool

	// Result returns the row (i.e., array of vdl Values) that was staged by Advance.
	// Result may panic if Advance returned false or was not called at all.
	// Result does not block.
	Result() []*vdl.Value

	// Err returns a non-nil error iff the stream encountered any errors. Err does
	// not block.
	Err() error

	// Cancel notifies the ResultStream provider that it can stop producing results.
	// The client must call Cancel if it does not iterate through all results
	// (i.e. until Advance returns false). Cancel is idempotent and can be called
	// concurrently with a goroutine that is iterating via Advance/Result.
	// Cancel causes Advance to subsequently return false. Cancel does not block.
	Cancel()
}

////////////////////////////////////////
// interfaces.Database methods

func (d *database) St() store.Store {
	if !d.exists {
		vlog.Fatalf("database %q does not exist", d.name)
	}
	return d.st
}

func (d *database) App() interfaces.App {
	return d.a
}

func (d *database) CheckPermsInternal(ctx *context.T, call rpc.ServerCall, st store.StoreReader) error {
	if !d.exists {
		vlog.Fatalf("database %q does not exist", d.name)
	}
	return util.GetWithAuth(ctx, call, st, d.stKey(), &databaseData{})
}

func (d *database) SetPermsInternal(ctx *context.T, call rpc.ServerCall, perms access.Permissions, version string) error {
	if !d.exists {
		vlog.Fatalf("database %q does not exist", d.name)
	}
	return store.RunInTransaction(d.st, func(tx store.Transaction) error {
		data := &databaseData{}
		return util.UpdateWithAuth(ctx, call, tx, d.stKey(), data, func() error {
			if err := util.CheckVersion(ctx, version, data.Version); err != nil {
				return err
			}
			data.Perms = perms
			data.Version++
			return nil
		})
	})
}

func (d *database) Name() string {
	return d.name
}

////////////////////////////////////////
// query_db implementation

// Implement query_db's Database, Table and KeyValueStream interfaces.
type queryDb struct {
	ctx  *context.T
	call wire.DatabaseExecServerCall
	req  *databaseReq
	sntx store.SnapshotOrTransaction
}

func (db *queryDb) GetContext() *context.T {
	return db.ctx
}

func (db *queryDb) GetTable(name string) (query_db.Table, error) {
	tDb := &tableDb{
		qdb: db,
		req: &tableReq{
			name: name,
			d:    db.req,
		},
	}
	// Now that we have a table, we need to check permissions.
	if err := util.GetWithAuth(db.ctx, db.call, db.sntx, tDb.req.stKey(), &tableData{}); err != nil {
		return nil, err
	}
	return tDb, nil
}

type tableDb struct {
	qdb *queryDb
	req *tableReq
}

func (t *tableDb) Scan(keyRanges query_db.KeyRanges) (query_db.KeyValueStream, error) {
	streams := []store.Stream{}
	for _, keyRange := range keyRanges {
		// TODO(jkline): For now, acquire all of the streams at once to minimize the race condition.
		//               Need a way to Scan multiple ranges at the same state of uncommitted changes.
		streams = append(streams, t.qdb.sntx.Scan(util.ScanRangeArgs(util.JoinKeyParts(util.RowPrefix, t.req.name), keyRange.Start, keyRange.Limit)))
	}
	return &kvs{
		t:        t,
		curr:     0,
		validRow: false,
		it:       streams,
		err:      nil,
	}, nil
}

type kvs struct {
	t         *tableDb
	curr      int
	validRow  bool
	currKey   string
	currValue *vdl.Value
	it        []store.Stream // array of store.Streams
	err       error
}

func (s *kvs) Advance() bool {
	if s.err != nil {
		return false
	}
	for s.curr < len(s.it) {
		if s.it[s.curr].Advance() {
			// key
			keyBytes := s.it[s.curr].Key(nil)
			parts := util.SplitKeyParts(string(keyBytes))
			// TODO(rogulenko): Check access for the key.
			s.currKey = parts[len(parts)-1]
			// value
			valueBytes := s.it[s.curr].Value(nil)
			var currValue *vdl.Value
			if err := vom.Decode(valueBytes, &currValue); err != nil {
				s.validRow = false
				s.err = err
				return false
			}
			s.currValue = currValue
			s.validRow = true
			return true
		}
		// Advance returned false.  It could be an err, or it could
		// be we've reached the end.
		if err := s.it[s.curr].Err(); err != nil {
			s.validRow = false
			s.err = err
			return false
		}
		// We've reached the end of the iterator for this keyRange.
		// Jump to the next one.
		s.it[s.curr] = nil
		s.curr++
		s.validRow = false
	}
	// There are no more prefixes to scan.
	return false
}

func (s *kvs) KeyValue() (string, *vdl.Value) {
	if !s.validRow {
		return "", nil
	}
	return s.currKey, s.currValue
}

func (s *kvs) Err() error {
	return s.err
}

func (s *kvs) Cancel() {
	if s.it != nil {
		for i := s.curr; i < len(s.it); i++ {
			s.it[i].Cancel()
		}
		s.it = nil
	}
	// set curr to end of keyRanges so Advance will return false
	s.curr = len(s.it)
}

////////////////////////////////////////
// Internal helpers

func (d *database) stKey() string {
	return util.DatabasePrefix
}

func (d *databaseReq) batchReader() store.SnapshotOrTransaction {
	if d.batchId == nil {
		return nil
	} else if d.sn != nil {
		return d.sn
	} else {
		return d.tx
	}
}

func (d *databaseReq) batchTransaction() (store.Transaction, error) {
	if d.batchId == nil {
		return nil, nil
	} else if d.tx != nil {
		return d.tx, nil
	} else {
		return nil, wire.NewErrReadOnlyBatch(nil)
	}
}

// TODO(jlodhia): Schema check should happen within a transaction for each
// operation in database, table and row. Do schema check along with permissions
// check when fully-specified permission model is implemented.
func (d *databaseReq) checkSchemaVersion(ctx *context.T, schemaVersion int32) error {
	if !d.exists {
		// database does not exist yet and hence there is no schema to check.
		// This can happen if delete is called twice on the same database.
		return nil
	}
	schemaMetadata, err := d.getSchemaMetadataWithoutAuth(ctx)
	if err != nil {
		return err
	}
	if (schemaMetadata == nil) || (schemaMetadata.Version == schemaVersion) {
		return nil
	}
	return wire.NewErrSchemaVersionMismatch(ctx)
}
