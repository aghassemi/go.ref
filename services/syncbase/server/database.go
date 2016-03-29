// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package server

import (
	"math/rand"
	"path"
	"sync"
	"time"

	"v.io/v23/context"
	"v.io/v23/glob"
	"v.io/v23/query/engine"
	ds "v.io/v23/query/engine/datasource"
	"v.io/v23/query/syncql"
	"v.io/v23/rpc"
	"v.io/v23/security/access"
	wire "v.io/v23/services/syncbase"
	pubutil "v.io/v23/syncbase/util"
	"v.io/v23/verror"
	"v.io/v23/vom"
	"v.io/x/lib/vlog"
	"v.io/x/ref/services/syncbase/common"
	"v.io/x/ref/services/syncbase/server/interfaces"
	"v.io/x/ref/services/syncbase/server/util"
	"v.io/x/ref/services/syncbase/store"
	storeutil "v.io/x/ref/services/syncbase/store/util"
	"v.io/x/ref/services/syncbase/store/watchable"
	sbwatchable "v.io/x/ref/services/syncbase/watchable"
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
	st *watchable.Store // stores all data for a single database

	// Active snapshots and transactions corresponding to client batches.
	// TODO(sadovsky): Add timeouts and GC.
	mu  sync.Mutex // protects the fields below
	sns map[uint64]store.Snapshot
	txs map[uint64]*watchable.Transaction

	// Active ConflictResolver connection from the app to this database.
	// NOTE: For now, we assume there's only one open conflict resolution stream
	// per database (typically, from the app that owns the database).
	crStream wire.ConflictManagerStartConflictResolverServerCall
	// Mutex lock to protect concurrent read/write of crStream pointer
	crMu sync.Mutex
}

// databaseReq is a per-request object that handles Database RPCs.
// It embeds database and tracks request-specific batch state.
type databaseReq struct {
	*database
	// If non-nil, sn or tx will be non-nil.
	batchId *uint64
	sn      store.Snapshot
	tx      *watchable.Transaction
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
func OpenDatabase(ctx *context.T, a interfaces.App, name string, opts DatabaseOptions, openOpts storeutil.OpenOptions) (*database, error) {
	st, err := storeutil.OpenStore(opts.Engine, path.Join(opts.RootDir, opts.Engine), openOpts)
	if err != nil {
		return nil, err
	}
	wst, err := watchable.Wrap(st, a.Service().Clock(), &watchable.Options{
		ManagedPrefixes: []string{common.RowPrefix, common.PermsPrefix},
	})
	if err != nil {
		return nil, err
	}
	return &database{
		name:   name,
		a:      a,
		exists: true,
		st:     wst,
		sns:    make(map[uint64]store.Snapshot),
		txs:    make(map[uint64]*watchable.Transaction),
	}, nil
}

// NewDatabase creates a new database instance and returns it.
// Designed for use from within App.CreateDatabase.
func NewDatabase(ctx *context.T, a interfaces.App, name string, metadata *wire.SchemaMetadata, opts DatabaseOptions) (*database, error) {
	if opts.Perms == nil {
		return nil, verror.New(verror.ErrInternal, ctx, "perms must be specified")
	}
	d, err := OpenDatabase(ctx, a, name, opts, storeutil.OpenOptions{CreateIfMissing: true, ErrorIfExists: true})
	if err != nil {
		return nil, err
	}
	data := &DatabaseData{
		Name:           d.name,
		Perms:          opts.Perms,
		SchemaMetadata: metadata,
	}
	if err := store.Put(ctx, d.st, d.stKey(), data); err != nil {
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
	// {name string, a *app}. d.a.CreateDatabase will create a new database handle
	// and store it in d.a.dbs[d.name].
	return d.a.CreateDatabase(ctx, call, d.name, perms, metadata)
}

func (d *databaseReq) Destroy(ctx *context.T, call rpc.ServerCall, schemaVersion int32) error {
	if d.batchId != nil {
		return wire.NewErrBoundToBatch(ctx)
	}
	if err := d.checkSchemaVersion(ctx, schemaVersion); err != nil {
		return err
	}
	return d.a.DestroyDatabase(ctx, call, d.name)
}

func (d *databaseReq) Exists(ctx *context.T, call rpc.ServerCall, schemaVersion int32) (bool, error) {
	if !d.exists {
		return false, nil
	}
	if err := d.checkSchemaVersion(ctx, schemaVersion); err != nil {
		return false, err
	}
	return util.ErrorToExists(util.GetWithAuth(ctx, call, d.st, d.stKey(), &DatabaseData{}))
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
	var batchType common.BatchType
	for {
		id = uint64(rng.Int63())
		if bo.ReadOnly {
			if _, ok := d.sns[id]; !ok {
				d.sns[id] = d.st.NewSnapshot()
				batchType = common.BatchTypeSn
				break
			}
		} else {
			if _, ok := d.txs[id]; !ok {
				d.txs[id] = d.st.NewWatchableTransaction()
				batchType = common.BatchTypeTx
				break
			}
		}
	}
	return common.BatchSep + common.JoinBatchInfo(batchType, id), nil
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

func (d *databaseReq) Exec(ctx *context.T, call wire.DatabaseExecServerCall, schemaVersion int32, q string, params []*vom.RawBytes) error {
	// RunInTransaction() cannot be used here because we may or may not be
	// creating a transaction. qe.Exec must be called and the statement must be
	// parsed before we know if a snapshot or a transaction should be created. To
	// duplicate the semantics of RunInTransaction, we attempt the Exec up to 100
	// times and retry on ErrConcurrentTransaction.
	maxAttempts := 100
	attempt := 0
	for {
		err := d.execInternal(ctx, call, schemaVersion, q, params)
		if attempt >= maxAttempts || verror.ErrorID(err) != store.ErrConcurrentTransaction.ID {
			return err
		}
		attempt++
	}
}

func (d *databaseReq) execInternal(ctx *context.T, call wire.DatabaseExecServerCall, schemaVersion int32, q string, params []*vom.RawBytes) error {
	if !d.exists {
		return verror.New(verror.ErrNoExist, ctx, d.name)
	}
	if err := d.checkSchemaVersion(ctx, schemaVersion); err != nil {
		return err
	}
	impl := func() error {
		db := &queryDb{
			ctx:  ctx,
			call: call,
			dReq: d,
			sntx: nil, // Filled in later with existing or created sn/tx.
			tx:   nil, // Only filled in if new tx created.
		}
		st, err := engine.Create(db).PrepareStatement(q)
		if err != nil {
			return execCommitOrAbort(db, err)
		}
		headers, rs, err := st.Exec(params...)
		if err != nil {
			return execCommitOrAbort(db, err)
		}
		if rs.Err() != nil {
			return execCommitOrAbort(db, err)
		}
		sender := call.SendStream()
		// Push the headers first -- the client will retrieve them and return
		// them separately from the results.
		var resultHeaders []*vom.RawBytes
		for _, header := range headers {
			resultHeaders = append(resultHeaders, vom.RawBytesOf(header))
		}
		sender.Send(resultHeaders)
		for rs.Advance() {
			result := rs.Result()
			if err := sender.Send(result); err != nil {
				rs.Cancel()
				return execCommitOrAbort(db, err)
			}
		}
		return execCommitOrAbort(db, rs.Err())
	}
	return impl()
}

func execCommitOrAbort(qdb *queryDb, err error) error {
	if qdb.dReq.batchId != nil {
		return err // part of an enclosing sn/tx
	}
	if err != nil {
		if qdb.sntx != nil {
			qdb.sntx.Abort()
		}
		return err
	} else { // err is nil
		if qdb.tx != nil {
			return qdb.tx.Commit()
		} else if qdb.sntx != nil {
			return qdb.sntx.Abort()
		}
		return nil
	}
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
	data := &DatabaseData{}
	if err := util.GetWithAuth(ctx, call, d.st, d.stKey(), data); err != nil {
		return nil, "", err
	}
	return data.Perms, util.FormatVersion(data.Version), nil
}

func (d *databaseReq) GlobChildren__(ctx *context.T, call rpc.GlobChildrenServerCall, matcher *glob.Element) error {
	if !d.exists {
		return verror.New(verror.ErrNoExist, ctx, d.name)
	}
	impl := func(sntx store.SnapshotOrTransaction) error {
		// Check perms.
		if err := util.GetWithAuth(ctx, call, sntx, d.stKey(), &DatabaseData{}); err != nil {
			return err
		}
		return util.GlobChildren(ctx, call, matcher, sntx, common.CollectionPrefix)
	}
	if d.batchId != nil {
		return impl(d.batchReader())
	} else {
		sn := d.st.NewSnapshot()
		defer sn.Abort()
		return impl(sn)
	}
}

// See comment in v.io/v23/services/syncbase/service.vdl for why we can't
// implement ListCollections using Glob.
func (d *databaseReq) ListCollections(ctx *context.T, call rpc.ServerCall) ([]string, error) {
	if !d.exists {
		return nil, verror.New(verror.ErrNoExist, ctx, d.name)
	}
	impl := func(sntx store.SnapshotOrTransaction) ([]string, error) {
		// Check perms.
		if err := util.GetWithAuth(ctx, call, sntx, d.stKey(), &DatabaseData{}); err != nil {
			return nil, err
		}
		it := sntx.Scan(common.ScanPrefixArgs(common.CollectionPrefix, ""))
		keyBytes := []byte{}
		res := []string{}
		for it.Advance() {
			keyBytes = it.Key(keyBytes)
			parts := common.SplitNKeyParts(string(keyBytes), 2)
			// For explanation of Escape(), see comment in dispatcher.go.
			res = append(res, pubutil.Escape(parts[1]))
		}
		if err := it.Err(); err != nil {
			return nil, err
		}
		return res, nil
	}
	if d.batchId != nil {
		return impl(d.batchReader())
	} else {
		sntx := d.st.NewSnapshot()
		defer sntx.Abort()
		return impl(sntx)
	}
}

func (d *databaseReq) PauseSync(ctx *context.T, call rpc.ServerCall) error {
	if !d.exists {
		return verror.New(verror.ErrNoExist, ctx, d.name)
	}
	return watchable.RunInTransaction(d.St(), func(tx *watchable.Transaction) error {
		return sbwatchable.AddDbStateChangeRequestOp(ctx, tx, sbwatchable.StateChangePauseSync)
	})
}

func (d *databaseReq) ResumeSync(ctx *context.T, call rpc.ServerCall) error {
	if !d.exists {
		return verror.New(verror.ErrNoExist, ctx, d.name)
	}
	return watchable.RunInTransaction(d.St(), func(tx *watchable.Transaction) error {
		return sbwatchable.AddDbStateChangeRequestOp(ctx, tx, sbwatchable.StateChangeResumeSync)
	})
}

////////////////////////////////////////
// interfaces.Database methods

func (d *database) St() *watchable.Store {
	if !d.exists {
		vlog.Fatalf("database %q does not exist", d.name)
	}
	return d.st
}

func (d *database) App() interfaces.App {
	return d.a
}

func (d *database) Collection(ctx *context.T, collectionName string) interfaces.Collection {
	return &collectionReq{
		name: collectionName,
		d:    &databaseReq{database: d},
	}
}

func (d *database) CheckPermsInternal(ctx *context.T, call rpc.ServerCall, st store.StoreReader) error {
	if !d.exists {
		vlog.Fatalf("database %q does not exist", d.name)
	}
	return util.GetWithAuth(ctx, call, st, d.stKey(), &DatabaseData{})
}

func (d *database) SetPermsInternal(ctx *context.T, call rpc.ServerCall, perms access.Permissions, version string) error {
	if !d.exists {
		vlog.Fatalf("database %q does not exist", d.name)
	}
	return store.RunInTransaction(d.st, func(tx store.Transaction) error {
		data := &DatabaseData{}
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

func (d *database) CrConnectionStream() wire.ConflictManagerStartConflictResolverServerStream {
	d.crMu.Lock()
	defer d.crMu.Unlock()
	return d.crStream
}

func (d *database) ResetCrConnectionStream() {
	d.crMu.Lock()
	defer d.crMu.Unlock()
	// TODO(jlodhia): figure out a way for the connection to gracefully shutdown
	// so that the client can get an appropriate error msg.
	d.crStream = nil
}

////////////////////////////////////////
// query interface implementations

// queryDb implements ds.Database.
type queryDb struct {
	ctx  *context.T
	call rpc.ServerCall
	dReq *databaseReq
	sntx store.SnapshotOrTransaction
	tx   *watchable.Transaction // If transaction, this will be same as sntx (else nil)
}

func (qdb *queryDb) GetContext() *context.T {
	return qdb.ctx
}

func (qdb *queryDb) GetTable(name string, writeAccessReq bool) (ds.Table, error) {
	// At this point, when the query package calls GetTable with the
	// writeAccessReq arg, we know whether or not we need a [writable] transaction
	// or a snapshot. If batchId is already set, there's nothing to do; but if
	// not, the writeAccessReq arg dictates whether a snapshot or a transaction is
	// should be created.
	qt := &queryTable{
		qdb: qdb,
		cReq: &collectionReq{
			name: name,
			d:    qdb.dReq,
		},
	}
	if qt.cReq.d.batchId != nil {
		if writeAccessReq {
			// We are in a batch (could be snapshot or transaction)
			// and Write access is required.  Attempt to get a
			// transaction from the request.
			var err error
			qt.qdb.tx, err = qt.qdb.dReq.batchTransaction()
			if err != nil {
				// We are in a snapshot batch, write access cannot be provided.
				// Return NotWritable.
				return nil, syncql.NewErrNotWritable(qt.qdb.GetContext(), qt.cReq.name)
			}
			qt.qdb.sntx = qt.qdb.tx
		} else {
			qt.qdb.sntx = qt.qdb.dReq.batchReader()
		}
	} else {
		// Now that we know if write access is required, create a snapshot
		// or transaction.
		if !writeAccessReq {
			qt.qdb.sntx = qt.qdb.dReq.st.NewSnapshot()
		} else { // writeAccessReq
			qt.qdb.tx = qt.qdb.dReq.st.NewWatchableTransaction()
			qt.qdb.sntx = qt.qdb.tx
		}
	}
	// Now that we have a collection, we need to check permissions.
	if err := util.GetWithAuth(qdb.ctx, qdb.call, qdb.sntx, qt.cReq.stKey(), &CollectionData{}); err != nil {
		return nil, err
	}
	return qt, nil
}

// queryTable implements ds.Table.
type queryTable struct {
	qdb  *queryDb
	cReq *collectionReq
}

func (t *queryTable) GetIndexFields() []ds.Index {
	// TODO(jkline): If and when secondary indexes are supported, they
	// would be supplied here.
	return []ds.Index{}
}

func (t *queryTable) Delete(k string) (bool, error) {
	// Create a rowReq and call delete.  Permissions will be checked.
	rowReq := &rowReq{
		key: k,
		c:   t.cReq,
	}
	if err := rowReq.delete(t.qdb.GetContext(), t.qdb.call, t.qdb.tx); err != nil {
		return false, err
	}
	return true, nil
}

func (t *queryTable) Scan(indexRanges ...ds.IndexRanges) (ds.KeyValueStream, error) {
	streams := []store.Stream{}
	// Syncbase does not currently support secondary indexes. As such, indexRanges
	// is guaranteed to be one in size as it will only specify the key ranges;
	// hence, indexRanges[0] below.
	for _, keyRange := range *indexRanges[0].StringRanges {
		// TODO(jkline): For now, acquire all of the streams at once to minimize the
		// race condition. Need a way to Scan multiple ranges at the same state of
		// uncommitted changes.
		streams = append(streams, t.qdb.sntx.Scan(common.ScanRangeArgs(common.JoinKeyParts(common.RowPrefix, t.cReq.name), keyRange.Start, keyRange.Limit)))
	}
	return &kvs{
		t:        t,
		curr:     0,
		validRow: false,
		it:       streams,
		err:      nil,
	}, nil
}

// kvs implements ds.KeyValueStream.
type kvs struct {
	t         *queryTable
	curr      int
	validRow  bool
	currKey   string
	currValue *vom.RawBytes
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
			parts := common.SplitNKeyParts(string(keyBytes), 3)
			// TODO(rogulenko): Check access for the key.
			s.currKey = parts[2]
			// value
			valueBytes := s.it[s.curr].Value(nil)
			var currValue *vom.RawBytes
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

func (s *kvs) KeyValue() (string, *vom.RawBytes) {
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
	return common.DatabasePrefix
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

func (d *databaseReq) batchTransaction() (*watchable.Transaction, error) {
	if d.batchId == nil {
		return nil, nil
	} else if d.tx != nil {
		return d.tx, nil
	} else {
		return nil, wire.NewErrReadOnlyBatch(nil)
	}
}

// TODO(jlodhia): Schema check should happen within a transaction for each
// operation in database, collection and row. Do schema check along with
// permissions check when fully-specified permission model is implemented.
func (d *databaseReq) checkSchemaVersion(ctx *context.T, schemaVersion int32) error {
	if !d.exists {
		// database does not exist yet and hence there is no schema to check.
		// This can happen if delete is called twice on the same database.
		return nil
	}
	schemaMetadata, err := d.GetSchemaMetadataInternal(ctx)
	if err != nil {
		if verror.ErrorID(err) == verror.ErrNoExist.ID {
			return nil
		}
		return err
	}
	if schemaMetadata.Version == schemaVersion {
		return nil
	}
	return wire.NewErrSchemaVersionMismatch(ctx)
}
