// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package server

// This file defines internal app methods for manipulating dbInfo.
// None of these methods perform authorization checks.
//
// The fundamental reason why these methods are needed is that information about
// a database is spread across two storage engines. The source of truth for the
// existence of the database, as well as things like the database type, is the
// service-level storage engine, while database permissions are tracked in the
// database's storage engine.

import (
	"v.io/v23/context"
	"v.io/x/ref/services/syncbase/server/util"
	"v.io/x/ref/services/syncbase/store"
)

func dbInfoStKey(a *app, dbName string) string {
	return util.JoinKeyParts(util.DbInfoPrefix, a.stKeyPart(), dbName)
}

// getDbInfo reads data from the storage engine.
func (a *app) getDbInfo(ctx *context.T, sntx store.SnapshotOrTransaction, dbName string) (*dbInfo, error) {
	info := &dbInfo{}
	if err := util.Get(ctx, sntx, dbInfoStKey(a, dbName), info); err != nil {
		return nil, err
	}
	return info, nil
}

// putDbInfo writes data to the storage engine.
func (a *app) putDbInfo(ctx *context.T, tx store.Transaction, dbName string, info *dbInfo) error {
	return util.Put(ctx, tx, dbInfoStKey(a, dbName), info)
}

// delDbInfo deletes data from the storage engine.
func (a *app) delDbInfo(ctx *context.T, stw store.StoreWriter, dbName string) error {
	return util.Delete(ctx, stw, dbInfoStKey(a, dbName))
}

// updateDbInfo performs a read-modify-write. fn should "modify" v.
func (a *app) updateDbInfo(ctx *context.T, tx store.Transaction, dbName string, fn func(info *dbInfo) error) error {
	info, err := a.getDbInfo(ctx, tx, dbName)
	if err != nil {
		return err
	}
	if err := fn(info); err != nil {
		return err
	}
	return a.putDbInfo(ctx, tx, dbName, info)
}
