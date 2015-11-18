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
//
// Sync also tracks the current generation number and the current local log
// position for each mutation of a syncgroup, created on a Database. Similar to
// the data log records, these log records are used to sync syncgroup metadata.
//
// The generations for the data mutations and mutations for each syncgroup are
// in separate spaces. Data mutations in a Database start at gen 1, and grow.
// Mutations for each syncgroup start at gen 1, and grow. Thus, for the local
// data log records, the keys are of the form y:l:d:<devid>:<gen>, and the keys
// for local syncgroup log record are of the form y:l:<sgoid>:<devid>:<gen>.

// TODO(hpucha): Should this space be separate from the data or not? If it is
// not, it can provide consistency between data and syncgroup metadata. For
// example, lets say we mutate the data in a syncgroup and soon after change the
// syncgroup ACL to prevent syncing with a device. This device may not get the
// last batch of updates since the next time it will try to sync, it will be
// rejected. However implementing consistency is not straightforward. Even if we
// had syncgroup updates in the same space as the data, we need to switch to the
// right syncgroup ACL at the responder based on the requested generations.

import (
	"fmt"
	"time"

	"v.io/v23/context"
	"v.io/v23/verror"
	"v.io/v23/vom"
	"v.io/x/lib/vlog"
	"v.io/x/ref/services/syncbase/server/interfaces"
	"v.io/x/ref/services/syncbase/server/util"
	"v.io/x/ref/services/syncbase/store"
)

// localGenInfoInMem represents the state corresponding to local generations.
type localGenInfoInMem struct {
	gen        uint64
	pos        uint64
	checkptGen uint64
}

func (in *localGenInfoInMem) deepCopy() *localGenInfoInMem {
	out := &localGenInfoInMem{
		gen:        in.gen,
		pos:        in.pos,
		checkptGen: in.checkptGen,
	}
	return out
}

// dbSyncStateInMem represents the in-memory sync state of a Database and all
// its syncgroups.
type dbSyncStateInMem struct {
	data *localGenInfoInMem // info for data.

	// Info for syncgroups. The key here is the syncgroup oid of the form
	// y:s:<groupId>. More details in syncgroup.go.
	sgs map[string]*localGenInfoInMem

	// Note: Generation vector contains state from remote devices only.
	genvec   interfaces.GenVector
	sggenvec interfaces.GenVector
}

func (in *dbSyncStateInMem) deepCopy() *dbSyncStateInMem {
	out := &dbSyncStateInMem{}
	out.data = in.data.deepCopy()

	out.sgs = make(map[string]*localGenInfoInMem)
	for oid, info := range in.sgs {
		out.sgs[oid] = info.deepCopy()
	}

	out.genvec = in.genvec.DeepCopy()
	out.sggenvec = in.sggenvec.DeepCopy()

	return out
}

// sgPublishInfo holds information on a syncgroup waiting to be published to a
// remote peer.  It is an in-memory entry in a queue of pending syncgroups.
type sgPublishInfo struct {
	sgName  string
	appName string
	dbName  string
	queued  time.Time
	lastTry time.Time
}

// initSync initializes the sync module during startup. It scans all the
// databases across all apps to initialize the following:
// a) in-memory sync state of a Database and all its syncgroups consisting of
// the current generation number, log position and generation vector.
// b) watcher map of prefixes currently being synced.
// c) republish names in mount tables for all syncgroups.
// d) in-memory queue of syncgroups to be published.
func (s *syncService) initSync(ctx *context.T) error {
	vlog.VI(2).Infof("sync: initSync: begin")
	defer vlog.VI(2).Infof("sync: initSync: end")
	s.syncStateLock.Lock()
	defer s.syncStateLock.Unlock()

	var errFinal error
	s.syncState = make(map[string]*dbSyncStateInMem)
	newMembers := make(map[string]*memberInfo)

	s.forEachDatabaseStore(ctx, func(appName, dbName string, st store.Store) bool {
		// Fetch the sync state for data and syncgroups.
		ds, err := getDbSyncState(ctx, st)
		if err != nil && verror.ErrorID(err) != verror.ErrNoExist.ID {
			errFinal = err
			return false
		}

		dsInMem := &dbSyncStateInMem{
			data: &localGenInfoInMem{},
			sgs:  make(map[string]*localGenInfoInMem),
		}

		if err == nil {
			// Initialize in memory state from the persistent state.
			dsInMem.genvec = ds.GenVec
			dsInMem.sggenvec = ds.SgGenVec
		}

		vlog.VI(2).Infof("sync: initSync: initing app %v db %v, dsInMem %v", appName, dbName, dsInMem)

		sgCount := 0
		name := appDbName(appName, dbName)

		// Scan the syncgroups and init relevant metadata.
		forEachSyncgroup(st, func(sg *interfaces.Syncgroup) bool {
			sgCount++

			// Only use syncgroups that have been marked as
			// "watchable" by the sync watcher thread. This is to
			// handle the case of a syncgroup being created but
			// Syncbase restarting before the watcher processed the
			// SyncgroupOp entry in the watch queue. It should not
			// be syncing that syncgroup's data after restart, but
			// wait until the watcher processes the entry as would
			// have happened without a restart.
			state, err := getSGIdEntry(ctx, st, sg.Id)
			if err != nil {
				errFinal = err
				return false
			}
			if state.Watched {
				for _, prefix := range sg.Spec.Prefixes {
					addWatchPrefixSyncgroup(appName, dbName, toTableRowPrefixStr(prefix), sg.Id)
				}
			}

			if sg.Status == interfaces.SyncgroupStatusPublishPending {
				s.enqueuePublishSyncgroup(sg.Name, appName, dbName, false)
			}

			// Refresh membership view.
			refreshSyncgroupMembers(sg, name, newMembers)

			sgoid := sgOID(sg.Id)
			info := &localGenInfoInMem{}
			dsInMem.sgs[sgoid] = info

			// Adjust the gen and pos for the sgoid.
			info.gen, info.pos, err = s.computeCurGenAndPos(ctx, st, sgoid, dsInMem.sggenvec[sgoid])
			if err != nil {
				errFinal = err
				return false
			}
			info.checkptGen = info.gen - 1

			vlog.VI(4).Infof("sync: initSync: initing app %v db %v sg %v info %v", appName, dbName, sgoid, info)

			return false
		})

		if sgCount == 0 {
			vlog.VI(2).Infof("sync: initSync: initing app %v db %v done (no sgs found)", appName, dbName)
			return false
		}

		// Compute the max known data generation for each known device.
		maxgenvec := interfaces.PrefixGenVector{}
		for _, pgv := range dsInMem.genvec {
			for dev, gen := range pgv {
				if gen > maxgenvec[dev] {
					maxgenvec[dev] = gen
				}
			}
		}

		// Adjust the gen and pos for the data.
		dsInMem.data.gen, dsInMem.data.pos, err = s.computeCurGenAndPos(ctx, st, logDataPrefix, maxgenvec)
		if err != nil {
			errFinal = err
			return false
		}
		dsInMem.data.checkptGen = dsInMem.data.gen - 1

		s.syncState[name] = dsInMem

		vlog.VI(2).Infof("sync: initSync: initing app %v db %v done dsInMem %v (data %v)", appName, dbName, dsInMem, dsInMem.data)

		return false
	})

	s.allMembersLock.Lock()
	s.allMembers = &memberView{expiration: time.Now().Add(memberViewTTL), members: newMembers}
	s.allMembersLock.Unlock()

	return errFinal
}

// computeCurGenAndPos computes the current local generation count and local log
// position for data or a specified syncgroup.
func (s *syncService) computeCurGenAndPos(ctx *context.T, st store.Store, pfx string, genvec interfaces.PrefixGenVector) (uint64, uint64, error) {
	found := false

	// Scan the local log records to determine latest gen and its pos.
	stream := st.Scan(util.ScanPrefixArgs(logRecsPerDeviceScanPrefix(pfx, s.id), ""))
	defer stream.Cancel()

	// Get the last value.
	var val []byte
	for stream.Advance() {
		val = stream.Value(val)
		found = true
	}

	if err := stream.Err(); err != nil {
		return 0, 0, err
	}

	var maxpos, maxgen uint64
	if found {
		var lrec localLogRec
		if err := vom.Decode(val, &lrec); err != nil {
			return 0, 0, err
		}
		maxpos = lrec.Pos
		maxgen = lrec.Metadata.Gen
	}

	for id, gen := range genvec {
		if gen == 0 {
			continue
		}
		// Since log records may be filtered, we search for the last
		// available log record going backwards from the generation up
		// to which a device is caught up.
		lrec, err := getPrevLogRec(ctx, st, pfx, id, gen)
		if err != nil {
			return 0, 0, err
		}
		if lrec != nil && lrec.Pos > maxpos {
			found = true
			maxpos = lrec.Pos
		}
	}

	if found {
		maxpos++
	}

	return maxgen + 1, maxpos, nil
}

// TODO(hpucha): This can be optimized using a backwards scan or a better
// search.
func getPrevLogRec(ctx *context.T, st store.Store, pfx string, dev, gen uint64) (*localLogRec, error) {
	for i := gen; i > 0; i-- {
		rec, err := getLogRec(ctx, st, pfx, dev, i)
		if err == nil {
			return rec, nil
		}
		if verror.ErrorID(err) != verror.ErrNoExist.ID {
			return nil, err
		}
	}
	return nil, nil
}

// enqueuePublishSyncgroup appends the given syncgroup to the publish queue.
func (s *syncService) enqueuePublishSyncgroup(sgName, appName, dbName string, attempted bool) {
	s.sgPublishQueueLock.Lock()
	defer s.sgPublishQueueLock.Unlock()

	entry := &sgPublishInfo{
		sgName:  sgName,
		appName: appName,
		dbName:  dbName,
		queued:  time.Now(),
	}
	if attempted {
		entry.lastTry = entry.queued
	}
	s.sgPublishQueue.PushBack(entry)
}

// Note: For all the utilities below, if the sgid parameter is non-nil, the
// operation is performed in the syncgroup space. If nil, it is performed in the
// data space for the Database.

// reserveGenAndPosInDbLog reserves a chunk of generation numbers and log
// positions in a Database's log. Used when local updates result in log
// entries.
func (s *syncService) reserveGenAndPosInDbLog(ctx *context.T, appName, dbName, sgoid string, count uint64) (uint64, uint64) {
	return s.reserveGenAndPosInternal(appName, dbName, sgoid, count, count)
}

// reservePosInDbLog reserves a chunk of log positions in a Database's log. Used
// when remote log records are received.
func (s *syncService) reservePosInDbLog(ctx *context.T, appName, dbName, sgoid string, count uint64) uint64 {
	_, pos := s.reserveGenAndPosInternal(appName, dbName, sgoid, 0, count)
	return pos
}

func (s *syncService) reserveGenAndPosInternal(appName, dbName, sgoid string, genCount, posCount uint64) (uint64, uint64) {
	s.syncStateLock.Lock()
	defer s.syncStateLock.Unlock()

	name := appDbName(appName, dbName)
	ds, ok := s.syncState[name]
	if !ok {
		ds = &dbSyncStateInMem{
			data: &localGenInfoInMem{gen: 1},
			sgs:  make(map[string]*localGenInfoInMem),
		}
		s.syncState[name] = ds
	}

	var info *localGenInfoInMem
	if sgoid != "" {
		var ok bool
		info, ok = ds.sgs[sgoid]
		if !ok {
			info = &localGenInfoInMem{gen: 1}
			ds.sgs[sgoid] = info
		}
	} else {
		info = ds.data
	}
	gen := info.gen
	pos := info.pos

	info.gen += genCount
	info.pos += posCount

	return gen, pos
}

// checkptLocalGen freezes the local generation number for the responder's use.
func (s *syncService) checkptLocalGen(ctx *context.T, appName, dbName string, sgs sgSet) error {
	s.syncStateLock.Lock()
	defer s.syncStateLock.Unlock()

	name := appDbName(appName, dbName)
	ds, ok := s.syncState[name]
	if !ok {
		return verror.New(verror.ErrInternal, ctx, "db state not found", name)
	}

	// The frozen generation is the last generation number used, i.e. one
	// below the next available one to use.
	if len(sgs) > 0 {
		// Checkpoint requested syncgroups.
		for id := range sgs {
			info, ok := ds.sgs[sgOID(id)]
			if !ok {
				return verror.New(verror.ErrInternal, ctx, "sg state not found", name, id)
			}
			info.checkptGen = info.gen - 1
		}
	} else {
		ds.data.checkptGen = ds.data.gen - 1
	}
	return nil
}

// initSyncStateInMem initializes the in memory sync state of the
// database/syncgroup if needed.
func (s *syncService) initSyncStateInMem(ctx *context.T, appName, dbName string, sgoid string) {
	s.syncStateLock.Lock()
	defer s.syncStateLock.Unlock()

	name := appDbName(appName, dbName)
	if s.syncState[name] == nil {
		s.syncState[name] = &dbSyncStateInMem{
			data: &localGenInfoInMem{gen: 1},
			sgs:  make(map[string]*localGenInfoInMem),
		}
	}
	if sgoid != "" {
		ds := s.syncState[name]
		if _, ok := ds.sgs[sgoid]; !ok {
			ds.sgs[sgoid] = &localGenInfoInMem{gen: 1}
		}
	}
	return
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
	return ds.deepCopy(), nil
}

// copyDbGenInfo returns a copy of the current generation information of the Database.
func (s *syncService) copyDbGenInfo(ctx *context.T, appName, dbName string, sgs sgSet) (interfaces.GenVector, uint64, error) {
	s.syncStateLock.Lock()
	defer s.syncStateLock.Unlock()

	name := appDbName(appName, dbName)
	ds, ok := s.syncState[name]
	if !ok {
		return nil, 0, verror.New(verror.ErrInternal, ctx, "db state not found", name)
	}

	var genvec interfaces.GenVector
	var gen uint64
	if len(sgs) > 0 {
		genvec = make(interfaces.GenVector)
		for id := range sgs {
			sgoid := sgOID(id)
			gv := ds.sggenvec[sgoid]
			genvec[sgoid] = gv.DeepCopy()
			genvec[sgoid][s.id] = ds.sgs[sgoid].checkptGen
		}
	} else {
		genvec = ds.genvec.DeepCopy()

		// Add local generation information to the genvec.
		for _, gv := range genvec {
			gv[s.id] = ds.data.checkptGen
		}
		gen = ds.data.checkptGen
	}
	return genvec, gen, nil
}

// putDbGenInfoRemote puts the current remote generation information of the Database.
func (s *syncService) putDbGenInfoRemote(ctx *context.T, appName, dbName string, sg bool, genvec interfaces.GenVector) error {
	s.syncStateLock.Lock()
	defer s.syncStateLock.Unlock()

	name := appDbName(appName, dbName)
	ds, ok := s.syncState[name]
	if !ok {
		return verror.New(verror.ErrInternal, ctx, "db state not found", name)
	}

	if sg {
		ds.sggenvec = genvec.DeepCopy()
	} else {
		ds.genvec = genvec.DeepCopy()
	}

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
	parts := util.SplitNKeyParts(name, 2)
	if len(parts) != 2 {
		return "", "", verror.New(verror.ErrInternal, ctx, "invalid appDbName", name)
	}
	return parts[0], parts[1], nil
}

////////////////////////////////////////////////////////////
// Low-level utility functions to access sync state.

// putDbSyncState persists the sync state object for a given Database.
func putDbSyncState(ctx *context.T, tx store.Transaction, ds *dbSyncState) error {
	return util.Put(ctx, tx, dbssKey, ds)
}

// getDbSyncState retrieves the sync state object for a given Database.
func getDbSyncState(ctx *context.T, st store.StoreReader) (*dbSyncState, error) {
	var ds dbSyncState
	if err := util.Get(ctx, st, dbssKey, &ds); err != nil {
		return nil, err
	}
	return &ds, nil
}

////////////////////////////////////////////////////////////
// Low-level utility functions to access log records.

// logRecsPerDeviceScanPrefix returns the prefix used to scan log records for a particular device.
func logRecsPerDeviceScanPrefix(pfx string, id uint64) string {
	return util.JoinKeyParts(logPrefix, pfx, fmt.Sprintf("%d", id))
}

// logRecKey returns the key used to access a specific log record.
func logRecKey(pfx string, id, gen uint64) string {
	return util.JoinKeyParts(logPrefix, pfx, fmt.Sprintf("%d", id), fmt.Sprintf("%016x", gen))
}

// hasLogRec returns true if the log record for (devid, gen) exists.
func hasLogRec(st store.StoreReader, pfx string, id, gen uint64) (bool, error) {
	return util.Exists(nil, st, logRecKey(pfx, id, gen))
}

// putLogRec stores the log record.
func putLogRec(ctx *context.T, tx store.Transaction, pfx string, rec *localLogRec) error {
	return util.Put(ctx, tx, logRecKey(pfx, rec.Metadata.Id, rec.Metadata.Gen), rec)
}

// getLogRec retrieves the log record for a given (devid, gen).
func getLogRec(ctx *context.T, st store.StoreReader, pfx string, id, gen uint64) (*localLogRec, error) {
	return getLogRecByKey(ctx, st, logRecKey(pfx, id, gen))
}

// getLogRecByKey retrieves the log record for a given log record key.
func getLogRecByKey(ctx *context.T, st store.StoreReader, key string) (*localLogRec, error) {
	var rec localLogRec
	if err := util.Get(ctx, st, key, &rec); err != nil {
		return nil, err
	}
	return &rec, nil
}

// delLogRec deletes the log record for a given (devid, gen).
func delLogRec(ctx *context.T, tx store.Transaction, pfx string, id, gen uint64) error {
	return util.Delete(ctx, tx, logRecKey(pfx, id, gen))
}
