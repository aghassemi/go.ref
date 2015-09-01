// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vsync

// New log records are created when objects in the local store are created,
// updated or deleted. Local log records are also replayed to keep the
// per-object dags consistent with the local store state. Sync module assigns
// each log record created within a Database a unique sequence number, called
// the generation number. Locally on each device, the position of each log
// record is also recorded relative to other local and remote log records.
//
// When a device receives a request to send log records, it first computes the
// missing generations between itself and the incoming request on a per-prefix
// basis. It then sends all the log records belonging to the missing generations
// in the order they occur locally (using the local log position). A device that
// receives log records over the network replays all the records received from
// another device in a single batch. Each replayed log record adds a new version
// to the dag of the object contained in the log record. At the end of replaying
// all the log records, conflict detection and resolution is carried out for all
// the objects learned during this iteration. Conflict detection and resolution
// is carried out after a batch of log records are replayed, instead of
// incrementally after each record is replayed, to avoid repeating conflict
// resolution already performed by other devices.
//
// Sync module tracks the current generation number and the current local log
// position for each Database. In addition, it also tracks the current
// generation vector for a Database. Log records are indexed such that they can
// be selectively retrieved from the store for any missing generation from any
// device.

import (
	"fmt"
	"strconv"

	"v.io/v23/context"
	"v.io/v23/verror"
	"v.io/x/ref/services/syncbase/server/interfaces"
	"v.io/x/ref/services/syncbase/server/util"
	"v.io/x/ref/services/syncbase/store"
)

// dbSyncStateInMem represents the in-memory sync state of a Database.
type dbSyncStateInMem struct {
	gen uint64
	pos uint64

	checkptGen uint64
	genvec     interfaces.GenVector // Note: Generation vector contains state from remote devices only.
}

// initSync initializes the sync module during startup. It scans all the
// databases across all apps to initialize the following:
// a) in-memory sync state of a Database consisting of the current generation
//    number, log position and generation vector.
// b) watcher map of prefixes currently being synced.
// c) republish names in mount tables for all syncgroups.
//
// TODO(hpucha): This is incomplete. Flesh this out further.
func (s *syncService) initSync(ctx *context.T) error {
	s.syncStateLock.Lock()
	defer s.syncStateLock.Unlock()

	var errFinal error
	s.syncState = make(map[string]*dbSyncStateInMem)

	s.forEachDatabaseStore(ctx, func(appName, dbName string, st store.Store) bool {
		// Scan the SyncGroups, skipping those not yet being watched.
		forEachSyncGroup(st, func(sg *interfaces.SyncGroup) bool {
			// TODO(rdaoud): only use SyncGroups that have been
			// marked as "watchable" by the sync watcher thread.
			// This is to handle the case of a SyncGroup being
			// created but Syncbase restarting before the watcher
			// processed the SyncGroupOp entry in the watch queue.
			// It should not be syncing that SyncGroup's data after
			// restart, but wait until the watcher processes the
			// entry as would have happened without a restart.
			for _, prefix := range sg.Spec.Prefixes {
				incrWatchPrefix(appName, dbName, prefix)
			}
			return false
		})

		if false {
			// Fetch the sync state.
			ds, err := getDbSyncState(ctx, st)
			if err != nil && verror.ErrorID(err) != verror.ErrNoExist.ID {
				errFinal = err
				return false
			}
			var scanStart, scanLimit []byte
			// Figure out what to scan among local log records.
			if verror.ErrorID(err) == verror.ErrNoExist.ID {
				scanStart, scanLimit = util.ScanPrefixArgs(logRecsPerDeviceScanPrefix(s.id), "")
			} else {
				scanStart, scanLimit = util.ScanPrefixArgs(logRecKey(s.id, ds.Gen), "")
			}
			var maxpos uint64
			var dbName string
			// Scan local log records to find the most recent one.
			st.Scan(scanStart, scanLimit)
			// Scan remote log records using the persisted GenVector.
			s.syncState[dbName] = &dbSyncStateInMem{pos: maxpos + 1}
		}

		return false
	})

	return errFinal
}

// reserveGenAndPosInDbLog reserves a chunk of generation numbers and log
// positions in a Database's log. Used when local updates result in log
// entries.
func (s *syncService) reserveGenAndPosInDbLog(ctx *context.T, appName, dbName string, count uint64) (uint64, uint64) {
	return s.reserveGenAndPosInternal(appName, dbName, count, count)
}

// reservePosInDbLog reserves a chunk of log positions in a Database's log. Used
// when remote log records are received.
func (s *syncService) reservePosInDbLog(ctx *context.T, appName, dbName string, count uint64) uint64 {
	_, pos := s.reserveGenAndPosInternal(appName, dbName, 0, count)
	return pos
}

func (s *syncService) reserveGenAndPosInternal(appName, dbName string, genCount, posCount uint64) (uint64, uint64) {
	s.syncStateLock.Lock()
	defer s.syncStateLock.Unlock()

	name := appDbName(appName, dbName)
	ds, ok := s.syncState[name]
	if !ok {
		ds = &dbSyncStateInMem{gen: 1}
		s.syncState[name] = ds
	}

	gen := ds.gen
	pos := ds.pos

	ds.gen += genCount
	ds.pos += posCount

	return gen, pos
}

// checkptLocalGen freezes the local generation number for the responder's use.
func (s *syncService) checkptLocalGen(ctx *context.T, appName, dbName string) error {
	s.syncStateLock.Lock()
	defer s.syncStateLock.Unlock()

	name := appDbName(appName, dbName)
	ds, ok := s.syncState[name]
	if !ok {
		return verror.New(verror.ErrInternal, ctx, "db state not found", name)
	}

	// The frozen generation is the last generation number used, i.e. one
	// below the next available one to use.
	ds.checkptGen = ds.gen - 1
	return nil
}

// initDbSyncStateInMem initializes the in memory sync state of the Database if needed.
func (s *syncService) initDbSyncStateInMem(ctx *context.T, appName, dbName string) {
	s.syncStateLock.Lock()
	defer s.syncStateLock.Unlock()

	name := appDbName(appName, dbName)
	if s.syncState[name] == nil {
		s.syncState[name] = &dbSyncStateInMem{gen: 1}
	}
}

// copyDbSyncStateInMem returns a copy of the current in memory sync state of the Database.
func (s *syncService) copyDbSyncStateInMem(ctx *context.T, appName, dbName string) (*dbSyncStateInMem, error) {
	s.syncStateLock.Lock()
	defer s.syncStateLock.Unlock()

	name := appDbName(appName, dbName)
	ds, ok := s.syncState[name]
	if !ok {
		return nil, verror.New(verror.ErrInternal, ctx, "db state not found", name)
	}

	dsCopy := &dbSyncStateInMem{
		gen:        ds.gen,
		pos:        ds.pos,
		checkptGen: ds.checkptGen,
	}

	dsCopy.genvec = copyGenVec(ds.genvec)

	return dsCopy, nil
}

// copyDbGenInfo returns a copy of the current generation information of the Database.
func (s *syncService) copyDbGenInfo(ctx *context.T, appName, dbName string) (interfaces.GenVector, uint64, error) {
	s.syncStateLock.Lock()
	defer s.syncStateLock.Unlock()

	name := appDbName(appName, dbName)
	ds, ok := s.syncState[name]
	if !ok {
		return nil, 0, verror.New(verror.ErrInternal, ctx, "db state not found", name)
	}

	genvec := copyGenVec(ds.genvec)

	// Add local generation information to the genvec.
	for _, gv := range genvec {
		gv[s.id] = ds.checkptGen
	}

	return genvec, ds.checkptGen, nil
}

// putDbGenInfoRemote puts the current remote generation information of the Database.
func (s *syncService) putDbGenInfoRemote(ctx *context.T, appName, dbName string, genvec interfaces.GenVector) error {
	s.syncStateLock.Lock()
	defer s.syncStateLock.Unlock()

	name := appDbName(appName, dbName)
	ds, ok := s.syncState[name]
	if !ok {
		return verror.New(verror.ErrInternal, ctx, "db state not found", name)
	}

	ds.genvec = copyGenVec(genvec)

	return nil
}

// appDbName combines the app and db names to return a globally unique name for
// a Database.  This relies on the fact that the app name is globally unique and
// the db name is unique within the scope of the app.
func appDbName(appName, dbName string) string {
	return util.JoinKeyParts(appName, dbName)
}

// splitAppDbName is the inverse of appDbName and returns app and db name from a
// globally unique name for a Database.
func splitAppDbName(ctx *context.T, name string) (string, string, error) {
	parts := util.SplitKeyParts(name)
	if len(parts) != 2 {
		return "", "", verror.New(verror.ErrInternal, ctx, "invalid appDbName", name)
	}
	return parts[0], parts[1], nil
}

func copyGenVec(in interfaces.GenVector) interfaces.GenVector {
	genvec := make(interfaces.GenVector)
	for p, inpgv := range in {
		pgv := make(interfaces.PrefixGenVector)
		for id, gen := range inpgv {
			pgv[id] = gen
		}
		genvec[p] = pgv
	}
	return genvec
}

////////////////////////////////////////////////////////////
// Low-level utility functions to access sync state.

// dbSyncStateKey returns the key used to access the sync state of a Database.
func dbSyncStateKey() string {
	return util.JoinKeyParts(util.SyncPrefix, dbssPrefix)
}

// putDbSyncState persists the sync state object for a given Database.
func putDbSyncState(ctx *context.T, tx store.Transaction, ds *dbSyncState) error {
	return util.Put(ctx, tx, dbSyncStateKey(), ds)
}

// getDbSyncState retrieves the sync state object for a given Database.
func getDbSyncState(ctx *context.T, st store.StoreReader) (*dbSyncState, error) {
	var ds dbSyncState
	if err := util.Get(ctx, st, dbSyncStateKey(), &ds); err != nil {
		return nil, err
	}
	return &ds, nil
}

////////////////////////////////////////////////////////////
// Low-level utility functions to access log records.

// logRecsPerDeviceScanPrefix returns the prefix used to scan log records for a particular device.
func logRecsPerDeviceScanPrefix(id uint64) string {
	return util.JoinKeyParts(util.SyncPrefix, logPrefix, fmt.Sprintf("%x", id))
}

// logRecKey returns the key used to access a specific log record.
func logRecKey(id, gen uint64) string {
	return util.JoinKeyParts(util.SyncPrefix, logPrefix, fmt.Sprintf("%d", id), fmt.Sprintf("%016x", gen))
}

// splitLogRecKey is the inverse of logRecKey and returns device id and generation number.
func splitLogRecKey(ctx *context.T, key string) (uint64, uint64, error) {
	parts := util.SplitKeyParts(key)
	verr := verror.New(verror.ErrInternal, ctx, "invalid logreckey", key)
	if len(parts) != 4 {
		return 0, 0, verr
	}
	if parts[0] != util.SyncPrefix || parts[1] != logPrefix {
		return 0, 0, verr
	}
	id, err := strconv.ParseUint(parts[2], 10, 64)
	if err != nil {
		return 0, 0, verr
	}
	gen, err := strconv.ParseUint(parts[3], 16, 64)
	if err != nil {
		return 0, 0, verr
	}
	return id, gen, nil
}

// hasLogRec returns true if the log record for (devid, gen) exists.
func hasLogRec(st store.StoreReader, id, gen uint64) (bool, error) {
	// TODO(hpucha): optimize to avoid the unneeded fetch/decode of the data.
	var rec localLogRec
	if err := util.Get(nil, st, logRecKey(id, gen), &rec); err != nil {
		if verror.ErrorID(err) == verror.ErrNoExist.ID {
			err = nil
		}
		return false, err
	}
	return true, nil
}

// putLogRec stores the log record.
func putLogRec(ctx *context.T, tx store.Transaction, rec *localLogRec) error {
	return util.Put(ctx, tx, logRecKey(rec.Metadata.Id, rec.Metadata.Gen), rec)
}

// getLogRec retrieves the log record for a given (devid, gen).
func getLogRec(ctx *context.T, st store.StoreReader, id, gen uint64) (*localLogRec, error) {
	var rec localLogRec
	if err := util.Get(ctx, st, logRecKey(id, gen), &rec); err != nil {
		return nil, err
	}
	return &rec, nil
}

// delLogRec deletes the log record for a given (devid, gen).
func delLogRec(ctx *context.T, tx store.Transaction, id, gen uint64) error {
	return util.Delete(ctx, tx, logRecKey(id, gen))
}