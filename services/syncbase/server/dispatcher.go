// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package server

import (
	"strings"

	"v.io/v23/context"
	"v.io/v23/rpc"
	"v.io/v23/security"
	wire "v.io/v23/services/syncbase"
	pubutil "v.io/v23/syncbase/util"
	"v.io/v23/verror"
	"v.io/x/ref/services/syncbase/common"
	"v.io/x/ref/services/syncbase/server/interfaces"
)

type dispatcher struct {
	s *service
}

var _ rpc.Dispatcher = (*dispatcher)(nil)

func NewDispatcher(s *service) *dispatcher {
	return &dispatcher{s: s}
}

// We always return an AllowEveryone authorizer from Lookup(), and rely on our
// RPC method implementations to perform proper authorization.
var auth security.Authorizer = security.AllowEveryone()

func (disp *dispatcher) Lookup(ctx *context.T, suffix string) (interface{}, security.Authorizer, error) {
	if len(suffix) == 0 {
		return wire.ServiceServer(disp.s), auth, nil
	}
	// Namespace: <serviceName>/<encDbId>/<encCxId>/<encRowKey>
	parts := strings.SplitN(suffix, "/", 3)

	// If the first slash-separated component of suffix is SyncbaseSuffix,
	// dispatch to the sync module.
	if parts[0] == common.SyncbaseSuffix {
		return interfaces.SyncServer(disp.s.sync), auth, nil
	}

	// Validate all name components up front, so that we can avoid doing so in all
	// our method implementations.

	// Note, the slice returned by strings.SplitN is guaranteed to contain at
	// least one element.
	// TODO(sadovsky): If we were to continue putting the batch id in the name,
	// we'd need to switch to a BatchSep that's not allowed to appear in blessing
	// strings. However, we anyways plan to switch to passing the batch id as an
	// RPC argument, so we don't bother fixing this now.
	dbParts := strings.SplitN(parts[0], common.BatchSep, 2)
	encDbId := dbParts[0]
	dbId, err := pubutil.DecodeId(encDbId)
	if err != nil || !pubutil.ValidDatabaseId(dbId) {
		return nil, nil, wire.NewErrInvalidName(ctx, suffix)
	}

	var collectionName, rowKey string
	if len(parts) > 1 {
		collectionName, err = pubutil.Decode(parts[1])
		if err != nil || !pubutil.ValidCollectionName(collectionName) {
			return nil, nil, wire.NewErrInvalidName(ctx, suffix)
		}
	}
	if len(parts) > 2 {
		rowKey, err = pubutil.Decode(parts[2])
		if err != nil || !pubutil.ValidRowKey(rowKey) {
			return nil, nil, wire.NewErrInvalidName(ctx, suffix)
		}
	}

	dbExists := false
	var d *database
	if dInt, err := disp.s.Database(ctx, nil, dbId); err == nil {
		d = dInt.(*database) // panics on failure, as desired
		dbExists = true
	} else {
		if verror.ErrorID(err) != verror.ErrNoExist.ID {
			return nil, nil, err
		} else {
			// Database does not exist. Create a short-lived database object to
			// service this request.
			d = &database{
				id: dbId,
				s:  disp.s,
			}
		}
	}

	dReq := &databaseReq{database: d}
	if len(dbParts) == 2 {
		if err := setBatchFields(ctx, dReq, dbParts[1]); err != nil {
			return nil, nil, err
		}
	}
	// TODO(sadovsky): Is it possible to determine the RPC method from the
	// context? If so, we can check here that the database exists (unless the
	// client is calling Database.Create), and avoid the "d.exists" checks in all
	// the RPC method implementations.
	if len(parts) == 1 {
		return wire.DatabaseServer(dReq), auth, nil
	}

	// All collection and row methods require the database to exist. If it
	// doesn't, abort early.
	if !dbExists {
		return nil, nil, verror.New(verror.ErrNoExist, ctx, d.id)
	}

	// Note, it's possible for the database to be deleted concurrently with
	// downstream handling of this request. Depending on the order in which things
	// execute, the client may not get an error, but in any case ultimately the
	// store will end up in a consistent state.
	cReq := &collectionReq{
		name: collectionName,
		d:    dReq,
	}
	if len(parts) == 2 {
		return wire.CollectionServer(cReq), auth, nil
	}

	rReq := &rowReq{
		key: rowKey,
		c:   cReq,
	}
	if len(parts) == 3 {
		return wire.RowServer(rReq), auth, nil
	}

	return nil, nil, verror.NewErrNoExist(ctx)
}

// setBatchFields sets the batch-related fields in databaseReq based on the
// value of batchInfo (suffix of the database name component). It returns an
// error if batchInfo is malformed or the batch does not exist.
func setBatchFields(ctx *context.T, d *databaseReq, batchInfo string) error {
	// TODO(sadovsky): Maybe share a common keyspace between sns and txs so that
	// we can avoid including the batch type in the batchInfo string.
	batchType, batchId, err := common.SplitBatchInfo(batchInfo)
	if err != nil {
		return err
	}
	d.batchId = &batchId
	d.mu.Lock()
	defer d.mu.Unlock()
	var ok bool
	switch batchType {
	case common.BatchTypeSn:
		d.sn, ok = d.sns[batchId]
	case common.BatchTypeTx:
		d.tx, ok = d.txs[batchId]
	}
	if !ok {
		return wire.NewErrUnknownBatch(ctx)
	}
	return nil
}
