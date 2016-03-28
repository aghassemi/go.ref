// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vsync

// Sync utility functions

import (
	"time"

	"v.io/v23/context"
	"v.io/v23/rpc"
	wire "v.io/v23/services/syncbase"
	"v.io/x/lib/vlog"
	"v.io/x/ref/services/syncbase/common"
	"v.io/x/ref/services/syncbase/server/interfaces"
	"v.io/x/ref/services/syncbase/store/watchable"
)

const (
	nanoPerSec = int64(1000000000)
)

// forEachDatabaseStore iterates over all Databases in all Apps within the
// service and invokes the callback function on each database. The callback
// returns a "done" flag to make forEachDatabaseStore() stop the iteration
// earlier; otherwise the function loops across all databases of all apps.
func (s *syncService) forEachDatabaseStore(ctx *context.T, callback func(string, string, *watchable.Store) bool) {
	// Get the apps and iterate over them.
	// TODO(rdaoud): use a "privileged call" parameter instead of nil (here and
	// elsewhere).
	appNames, err := s.sv.AppNames(ctx, nil)
	if err != nil {
		vlog.Errorf("sync: forEachDatabaseStore: cannot get all app names: %v", err)
		return
	}

	for _, a := range appNames {
		// For each app, get its databases and iterate over them.
		app, err := s.sv.App(ctx, nil, a)
		if err != nil {
			vlog.Errorf("sync: forEachDatabaseStore: cannot get app %s: %v", a, err)
			continue
		}
		dbNames, err := app.DatabaseNames(ctx, nil)
		if err != nil {
			vlog.Errorf("sync: forEachDatabaseStore: cannot get all db names for app %s: %v", a, err)
			continue
		}

		for _, d := range dbNames {
			// For each database, get its Store and invoke the callback.
			db, err := app.Database(ctx, nil, d)
			if err != nil {
				vlog.Errorf("sync: forEachDatabaseStore: cannot get db %s for app %s: %v", d, a, err)
				continue
			}

			if callback(a, d, db.St()) {
				return // done, early exit
			}
		}
	}
}

// getDb gets the database handle.
func (s *syncService) getDb(ctx *context.T, call rpc.ServerCall, appName, dbName string) (interfaces.Database, error) {
	app, err := s.sv.App(ctx, call, appName)
	if err != nil {
		return nil, err
	}
	return app.Database(ctx, call, dbName)
}

// getDbStore gets the store handle to the database.
func (s *syncService) getDbStore(ctx *context.T, call rpc.ServerCall, appName, dbName string) (*watchable.Store, error) {
	db, err := s.getDb(ctx, call, appName, dbName)
	if err != nil {
		return nil, err
	}
	return db.St(), nil
}

// unixNanoToTime converts a Unix timestamp in nanoseconds to a Time object.
func unixNanoToTime(timestamp int64) time.Time {
	if timestamp < 0 {
		vlog.Fatalf("sync: unixNanoToTime: invalid timestamp %d", timestamp)
	}
	return time.Unix(timestamp/nanoPerSec, timestamp%nanoPerSec)
}

// toTableRowPrefixStr converts a TableRow (table name and row key or prefix
// pair) to a string of the form used for storing perms and row data in the
// underlying storage engine.
func toTableRowPrefixStr(p wire.TableRow) string {
	return common.JoinKeyParts(p.TableName, p.Row)
}

// toRowKey prepends RowPrefix to what is presumably a "<table>:<row>" string,
// yielding a storage engine key for a row.
// TODO(sadovsky): Only used by CR code. Should go away once CR stores table
// name and row key as separate fields in a "TableRow" struct.
func toRowKey(tableRow string) string {
	return common.JoinKeyParts(common.RowPrefix, tableRow)
}
