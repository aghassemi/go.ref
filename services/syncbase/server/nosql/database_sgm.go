// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package nosql

import (
	"v.io/v23/context"
	"v.io/v23/rpc"
	wire "v.io/v23/services/syncbase/nosql"
	"v.io/v23/verror"
	"v.io/x/ref/services/syncbase/vsync"
)

////////////////////////////////////////
// SyncGroup RPC methods

func (d *databaseReq) GetSyncGroupNames(ctx *context.T, call rpc.ServerCall) ([]string, error) {
	if d.batchId != nil {
		return nil, wire.NewErrBoundToBatch(ctx)
	}
	sd := vsync.NewSyncDatabase(d)
	return sd.GetSyncGroupNames(ctx, call)
}

func (d *databaseReq) CreateSyncGroup(ctx *context.T, call rpc.ServerCall, sgName string, spec wire.SyncGroupSpec, myInfo wire.SyncGroupMemberInfo) error {
	if d.batchId != nil {
		return wire.NewErrBoundToBatch(ctx)
	}
	sd := vsync.NewSyncDatabase(d)
	return sd.CreateSyncGroup(ctx, call, sgName, spec, myInfo)
}

func (d *databaseReq) JoinSyncGroup(ctx *context.T, call rpc.ServerCall, sgName string, myInfo wire.SyncGroupMemberInfo) (wire.SyncGroupSpec, error) {
	if d.batchId != nil {
		return wire.SyncGroupSpec{}, wire.NewErrBoundToBatch(ctx)
	}
	sd := vsync.NewSyncDatabase(d)
	return sd.JoinSyncGroup(ctx, call, sgName, myInfo)
}

func (d *databaseReq) LeaveSyncGroup(ctx *context.T, call rpc.ServerCall, sgName string) error {
	if d.batchId != nil {
		return wire.NewErrBoundToBatch(ctx)
	}
	return verror.NewErrNotImplemented(ctx)
}

func (d *databaseReq) DestroySyncGroup(ctx *context.T, call rpc.ServerCall, sgName string) error {
	if d.batchId != nil {
		return wire.NewErrBoundToBatch(ctx)
	}
	return verror.NewErrNotImplemented(ctx)
}

func (d *databaseReq) EjectFromSyncGroup(ctx *context.T, call rpc.ServerCall, sgName, member string) error {
	if d.batchId != nil {
		return wire.NewErrBoundToBatch(ctx)
	}
	return verror.NewErrNotImplemented(ctx)
}

func (d *databaseReq) GetSyncGroupSpec(ctx *context.T, call rpc.ServerCall, sgName string) (wire.SyncGroupSpec, string, error) {
	if d.batchId != nil {
		return wire.SyncGroupSpec{}, "", wire.NewErrBoundToBatch(ctx)
	}
	sd := vsync.NewSyncDatabase(d)
	return sd.GetSyncGroupSpec(ctx, call, sgName)
}

func (d *databaseReq) SetSyncGroupSpec(ctx *context.T, call rpc.ServerCall, sgName string, spec wire.SyncGroupSpec, version string) error {
	if d.batchId != nil {
		return wire.NewErrBoundToBatch(ctx)
	}
	sd := vsync.NewSyncDatabase(d)
	return sd.SetSyncGroupSpec(ctx, call, sgName, spec, version)
}

func (d *databaseReq) GetSyncGroupMembers(ctx *context.T, call rpc.ServerCall, sgName string) (map[string]wire.SyncGroupMemberInfo, error) {
	if d.batchId != nil {
		return nil, wire.NewErrBoundToBatch(ctx)
	}
	sd := vsync.NewSyncDatabase(d)
	return sd.GetSyncGroupMembers(ctx, call, sgName)
}
