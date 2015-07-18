// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package nosql

import (
	wire "v.io/syncbase/v23/services/syncbase/nosql"
	"v.io/syncbase/x/ref/services/syncbase/vsync"
	"v.io/v23/context"
	"v.io/v23/rpc"
)

////////////////////////////////////////////////////////////////////////////////
// RPCs for managing blobs between Syncbase and its clients.

func (d *databaseReq) CreateBlob(ctx *context.T, call rpc.ServerCall) (wire.BlobRef, error) {
	if d.batchId != nil {
		return wire.NullBlobRef, wire.NewErrBoundToBatch(ctx)
	}
	sd := vsync.NewSyncDatabase(d)
	return sd.CreateBlob(ctx, call)
}

func (d *databaseReq) PutBlob(ctx *context.T, call wire.BlobManagerPutBlobServerCall, br wire.BlobRef) error {
	if d.batchId != nil {
		return wire.NewErrBoundToBatch(ctx)
	}
	sd := vsync.NewSyncDatabase(d)
	return sd.PutBlob(ctx, call, br)
}

func (d *databaseReq) CommitBlob(ctx *context.T, call rpc.ServerCall, br wire.BlobRef) error {
	if d.batchId != nil {
		return wire.NewErrBoundToBatch(ctx)
	}
	sd := vsync.NewSyncDatabase(d)
	return sd.CommitBlob(ctx, call, br)
}

func (d *databaseReq) GetBlobSize(ctx *context.T, call rpc.ServerCall, br wire.BlobRef) (int64, error) {
	if d.batchId != nil {
		return 0, wire.NewErrBoundToBatch(ctx)
	}
	sd := vsync.NewSyncDatabase(d)
	return sd.GetBlobSize(ctx, call, br)
}

func (d *databaseReq) DeleteBlob(ctx *context.T, call rpc.ServerCall, br wire.BlobRef) error {
	if d.batchId != nil {
		return wire.NewErrBoundToBatch(ctx)
	}
	sd := vsync.NewSyncDatabase(d)
	return sd.DeleteBlob(ctx, call, br)
}

func (d *databaseReq) GetBlob(ctx *context.T, call wire.BlobManagerGetBlobServerCall, br wire.BlobRef, offset int64) error {
	if d.batchId != nil {
		return wire.NewErrBoundToBatch(ctx)
	}
	sd := vsync.NewSyncDatabase(d)
	return sd.GetBlob(ctx, call, br, offset)
}

func (d *databaseReq) FetchBlob(ctx *context.T, call wire.BlobManagerFetchBlobServerCall, br wire.BlobRef, priority uint64) error {
	if d.batchId != nil {
		return wire.NewErrBoundToBatch(ctx)
	}
	sd := vsync.NewSyncDatabase(d)
	return sd.FetchBlob(ctx, call, br, priority)
}

func (d *databaseReq) PinBlob(ctx *context.T, call rpc.ServerCall, br wire.BlobRef) error {
	if d.batchId != nil {
		return wire.NewErrBoundToBatch(ctx)
	}
	sd := vsync.NewSyncDatabase(d)
	return sd.PinBlob(ctx, call, br)
}

func (d *databaseReq) UnpinBlob(ctx *context.T, call rpc.ServerCall, br wire.BlobRef) error {
	if d.batchId != nil {
		return wire.NewErrBoundToBatch(ctx)
	}
	sd := vsync.NewSyncDatabase(d)
	return sd.UnpinBlob(ctx, call, br)
}

func (d *databaseReq) KeepBlob(ctx *context.T, call rpc.ServerCall, br wire.BlobRef, rank uint64) error {
	if d.batchId != nil {
		return wire.NewErrBoundToBatch(ctx)
	}
	sd := vsync.NewSyncDatabase(d)
	return sd.KeepBlob(ctx, call, br, rank)
}
