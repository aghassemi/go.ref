// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package server

import (
	"path"
	"sync"

	wire "v.io/syncbase/v23/services/syncbase"
	"v.io/syncbase/x/ref/services/syncbase/server/interfaces"
	"v.io/syncbase/x/ref/services/syncbase/server/nosql"
	"v.io/syncbase/x/ref/services/syncbase/server/util"
	"v.io/syncbase/x/ref/services/syncbase/store"
	"v.io/v23/context"
	"v.io/v23/rpc"
	"v.io/v23/security/access"
	"v.io/v23/verror"
	"v.io/x/lib/vlog"
)

// app is a per-app singleton (i.e. not per-request) that handles App RPCs.
type app struct {
	name string
	s    *service
	// The fields below are initialized iff this app exists.
	exists bool
	// Guards the fields below. Held during database Create, Delete, and
	// SetPermissions.
	mu  sync.Mutex
	dbs map[string]interfaces.Database
}

var (
	_ wire.AppServerMethods = (*app)(nil)
	_ interfaces.App        = (*app)(nil)
	_ util.Layer            = (*app)(nil)
)

////////////////////////////////////////
// RPC methods

// TODO(sadovsky): Require the app name to match the client's blessing name.
// I.e. reserve names at the app level of the hierarchy.
func (a *app) Create(ctx *context.T, call rpc.ServerCall, perms access.Permissions) error {
	if a.exists {
		return verror.New(verror.ErrExist, ctx, a.name)
	}
	// This app does not yet exist; a is just an ephemeral handle that holds
	// {name string, s *service}. a.s.createApp will create a new app handle and
	// store it in a.s.apps[a.name].
	return a.s.createApp(ctx, call, a.name, perms)
}

func (a *app) Delete(ctx *context.T, call rpc.ServerCall) error {
	return a.s.deleteApp(ctx, call, a.name)
}

func (a *app) SetPermissions(ctx *context.T, call rpc.ServerCall, perms access.Permissions, version string) error {
	if !a.exists {
		return verror.New(verror.ErrNoExist, ctx, a.name)
	}
	return a.s.setAppPerms(ctx, call, a.name, perms, version)
}

func (a *app) GetPermissions(ctx *context.T, call rpc.ServerCall) (perms access.Permissions, version string, err error) {
	if !a.exists {
		return nil, "", verror.New(verror.ErrNoExist, ctx, a.name)
	}
	data := &appData{}
	if err := util.Get(ctx, call, a.s.st, a, data); err != nil {
		return nil, "", err
	}
	return data.Perms, util.FormatVersion(data.Version), nil
}

func (a *app) GlobChildren__(ctx *context.T, call rpc.ServerCall) (<-chan string, error) {
	if !a.exists {
		return nil, verror.New(verror.ErrNoExist, ctx, a.name)
	}
	// Check perms.
	sn := a.s.st.NewSnapshot()
	closeSnapshot := func() error {
		return sn.Close()
	}
	if err := util.Get(ctx, call, sn, a, &appData{}); err != nil {
		closeSnapshot()
		return nil, err
	}
	return util.Glob(ctx, call, "*", sn, closeSnapshot, util.JoinKeyParts(util.DbInfoPrefix, a.name))
}

////////////////////////////////////////
// interfaces.App methods

func (a *app) Service() interfaces.Service {
	return a.s
}

// TODO(sadovsky): Currently, we return an error (here and elsewhere) if the
// specified db does not exist in a.dbs. But note, this will always be the case
// after a Syncbase service restart. The implementation should be updated so
// that a.dbs acts as an in-memory cache of db handles. If a database exists but
// is not present in a.dbs, a.NoSQLDatabase() should open the database and add
// its handle to a.dbs.
func (a *app) NoSQLDatabase(ctx *context.T, call rpc.ServerCall, dbName string) (interfaces.Database, error) {
	if !a.exists {
		vlog.Fatalf("app %q does not exist", a.name)
	}
	// TODO(sadovsky): Record storage engine config (e.g. LevelDB directory) in
	// dbInfo, and add API for opening and closing storage engines.
	a.mu.Lock()
	defer a.mu.Unlock()
	d, ok := a.dbs[dbName]
	if !ok {
		return nil, verror.New(verror.ErrNoExist, ctx, dbName)
	}
	return d, nil
}

func (a *app) NoSQLDatabaseNames(ctx *context.T, call rpc.ServerCall) ([]string, error) {
	if !a.exists {
		vlog.Fatalf("app %q does not exist", a.name)
	}
	// In the future this API will likely be replaced by one that streams the
	// database names.
	a.mu.Lock()
	defer a.mu.Unlock()
	dbNames := make([]string, 0, len(a.dbs))
	for n := range a.dbs {
		dbNames = append(dbNames, n)
	}
	return dbNames, nil
}

func (a *app) CreateNoSQLDatabase(ctx *context.T, call rpc.ServerCall, dbName string, perms access.Permissions) error {
	if !a.exists {
		vlog.Fatalf("app %q does not exist", a.name)
	}
	// TODO(sadovsky): Crash if any step fails, and use WAL to ensure that if we
	// crash, upon restart we execute any remaining steps before we start handling
	// client requests.
	//
	// Steps:
	// 1. Check appData perms, create dbInfo record.
	// 2. Initialize database.
	// 3. Flip dbInfo.Initialized to true. <===== CHANGE BECOMES VISIBLE
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, ok := a.dbs[dbName]; ok {
		// TODO(sadovsky): Should this be ErrExistOrNoAccess, for privacy?
		return verror.New(verror.ErrExist, ctx, dbName)
	}

	// 1. Check appData perms, create dbInfo record.
	aData := &appData{}
	if err := store.RunInTransaction(a.s.st, func(st store.StoreReadWriter) error {
		// Check appData perms.
		if err := util.Get(ctx, call, st, a, aData); err != nil {
			return err
		}
		// Check for "database already exists".
		if _, err := a.getDbInfo(ctx, call, st, dbName); verror.ErrorID(err) != verror.ErrNoExist.ID {
			if err != nil {
				return err
			}
			// TODO(sadovsky): Should this be ErrExistOrNoAccess, for privacy?
			return verror.New(verror.ErrExist, ctx, dbName)
		}
		// Write new dbInfo.
		info := &dbInfo{
			Name: dbName,
		}
		return a.putDbInfo(ctx, call, st, dbName, info)
	}); err != nil {
		return err
	}

	// 2. Initialize database.
	if perms == nil {
		perms = aData.Perms
	}
	d, err := nosql.NewDatabase(ctx, call, a, dbName, nosql.DatabaseOptions{
		Perms:   perms,
		RootDir: a.rootDirForDb(dbName),
		Engine:  a.s.opts.Engine,
	})
	if err != nil {
		return err
	}

	// 3. Flip dbInfo.Initialized to true.
	if err := store.RunInTransaction(a.s.st, func(st store.StoreReadWriter) error {
		return a.updateDbInfo(ctx, call, st, dbName, func(info *dbInfo) error {
			info.Initialized = true
			return nil
		})
	}); err != nil {
		return err
	}

	a.dbs[dbName] = d
	return nil
}

func (a *app) DeleteNoSQLDatabase(ctx *context.T, call rpc.ServerCall, dbName string) error {
	if !a.exists {
		vlog.Fatalf("app %q does not exist", a.name)
	}
	// TODO(sadovsky): Crash if any step fails, and use WAL to ensure that if we
	// crash, upon restart we execute any remaining steps before we start handling
	// client requests.
	//
	// Steps:
	// 1. Check databaseData perms.
	// 2. Flip dbInfo.Deleted to true. <===== CHANGE BECOMES VISIBLE
	// 3. Delete database.
	// 4. Delete dbInfo record.
	a.mu.Lock()
	defer a.mu.Unlock()
	d, ok := a.dbs[dbName]
	if !ok {
		return nil // delete is idempotent
	}

	// 1. Check databaseData perms.
	if err := d.CheckPermsInternal(ctx, call, d.St()); err != nil {
		if verror.ErrorID(err) == verror.ErrNoExist.ID {
			return nil // delete is idempotent
		}
		return err
	}

	// 2. Flip dbInfo.Deleted to true.
	if err := store.RunInTransaction(a.s.st, func(st store.StoreReadWriter) error {
		return a.updateDbInfo(ctx, call, st, dbName, func(info *dbInfo) error {
			info.Deleted = true
			return nil
		})
	}); err != nil {
		return err
	}

	// 3. Delete database.
	if err := d.St().Close(); err != nil {
		return err
	}
	if err := util.DestroyStore(a.s.opts.Engine, a.rootDirForDb(dbName)); err != nil {
		return err
	}

	// 4. Delete dbInfo record.
	if err := a.delDbInfo(ctx, call, a.s.st, dbName); err != nil {
		return err
	}

	delete(a.dbs, dbName)
	return nil
}

func (a *app) SetDatabasePerms(ctx *context.T, call rpc.ServerCall, dbName string, perms access.Permissions, version string) error {
	if !a.exists {
		vlog.Fatalf("app %q does not exist", a.name)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	d, ok := a.dbs[dbName]
	if !ok {
		return verror.New(verror.ErrNoExist, ctx, dbName)
	}
	return d.SetPermsInternal(ctx, call, perms, version)
}

////////////////////////////////////////
// util.Layer methods

func (a *app) Name() string {
	return a.name
}

func (a *app) StKey() string {
	return util.JoinKeyParts(util.AppPrefix, a.stKeyPart())
}

////////////////////////////////////////
// Internal helpers

func (a *app) stKeyPart() string {
	return a.name
}

func (a *app) rootDirForDb(dbName string) string {
	return path.Join(a.s.opts.RootDir, "apps", a.name, dbName)
}
