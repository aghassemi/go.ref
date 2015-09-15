// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vsync

import (
	"container/heap"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"v.io/v23/context"
	wire "v.io/v23/services/syncbase/nosql"
	"v.io/v23/verror"
	"v.io/x/lib/vlog"
	"v.io/x/ref/services/syncbase/server/interfaces"
	"v.io/x/ref/services/syncbase/server/watchable"
	"v.io/x/ref/services/syncbase/store"
)

// GetDeltas implements the responder side of the GetDeltas RPC.
func (s *syncService) GetDeltas(ctx *context.T, call interfaces.SyncGetDeltasServerCall, req interfaces.DeltaReq, initiator string) error {
	vlog.VI(2).Infof("sync: GetDeltas: begin: from initiator %s", initiator)
	defer vlog.VI(2).Infof("sync: GetDeltas: end: from initiator %s", initiator)

	rSt := newResponderState(ctx, call, s, req, initiator)
	return rSt.sendDeltasPerDatabase(ctx)
}

// responderState is state accumulated per Database by the responder during an
// initiation round.
type responderState struct {
	// Parameters from the request.
	appName string
	dbName  string
	sgIds   sgSet
	initVec interfaces.GenVector
	sg      bool

	call      interfaces.SyncGetDeltasServerCall // Stream handle for the GetDeltas RPC.
	initiator string
	sync      *syncService
	st        store.Store // Store handle to the Database.

	diff   genRangeVector
	outVec interfaces.GenVector
}

func newResponderState(ctx *context.T, call interfaces.SyncGetDeltasServerCall, sync *syncService, req interfaces.DeltaReq, initiator string) *responderState {
	rSt := &responderState{
		call:      call,
		sync:      sync,
		initiator: initiator,
	}

	switch v := req.(type) {
	case interfaces.DeltaReqData:
		rSt.appName = v.Value.AppName
		rSt.dbName = v.Value.DbName
		rSt.sgIds = v.Value.SgIds
		rSt.initVec = v.Value.InitVec

	case interfaces.DeltaReqSgs:
		rSt.sg = true
		rSt.appName = v.Value.AppName
		rSt.dbName = v.Value.DbName
		rSt.initVec = v.Value.InitVec
		rSt.sgIds = make(sgSet)
		// Populate the sgids from the initvec.
		for id := range rSt.initVec {
			gid, err := strconv.ParseUint(id, 10, 64)
			if err != nil {
				vlog.Fatalf("sync: newResponderState: invalid syncgroup id", gid)
			}
			rSt.sgIds[interfaces.GroupId(gid)] = struct{}{}
		}
	}
	return rSt
}

// sendDeltasPerDatabase sends to an initiator all the missing generations
// corresponding to the prefixes requested for this Database, and a genvector
// summarizing the knowledge transferred from the responder to the
// initiator. This happens in three phases:
//
// In the first phase, the initiator is checked against the SyncGroup ACLs of
// all the SyncGroups it is requesting, and only those prefixes that belong to
// allowed SyncGroups are carried forward.
//
// In the second phase, for a given set of nested prefixes from the initiator,
// the shortest prefix in that set is extracted. The initiator's prefix
// genvector for this shortest prefix represents the lower bound on its
// knowledge for the entire set of nested prefixes. This prefix genvector
// (representing the lower bound) is diffed with all the responder prefix
// genvectors corresponding to same or deeper prefixes compared to the initiator
// prefix. This diff produces a bound on the missing knowledge. For example, say
// the initiator is interested in prefixes {foo, foobar}, where each prefix is
// associated with a prefix genvector. Since the initiator strictly has as much
// or more knowledge for prefix "foobar" as it has for prefix "foo", "foo"'s
// prefix genvector is chosen as the lower bound for the initiator's
// knowledge. Similarly, say the responder has knowledge on prefixes {f,
// foobarX, foobarY, bar}. The responder diffs the prefix genvectors for
// prefixes f, foobarX and foobarY with the initiator's prefix genvector to
// compute a bound on missing generations (all responder's prefixes that match
// "foo". Note that since the responder doesn't have a prefix genvector at
// "foo", its knowledge at "f" is applicable to "foo").
//
// Since the second phase outputs an aggressive calculation of missing
// generations containing more generation entries than strictly needed by the
// initiator, in the third phase, each missing generation is sent to the
// initiator only if the initiator is eligible for it and is not aware of
// it. The generations are sent to the initiator in the same order as the
// responder learned them so that the initiator can reconstruct the DAG for the
// objects by learning older nodes first.
func (rSt *responderState) sendDeltasPerDatabase(ctx *context.T) error {
	// TODO(rdaoud): for such vlog.VI() calls where the function name is
	// embedded, consider using a helper function to auto-fill it instead
	// (see http://goo.gl/mEa4L0) but only incur that overhead when the
	// logging level specified is enabled.
	vlog.VI(3).Infof("sync: sendDeltasPerDatabase: %s, %s: sgids %v, genvec %v",
		rSt.appName, rSt.dbName, rSt.sgIds, rSt.initVec)

	// Phase 1 of sendDeltas: Authorize the initiator and respond to the
	// caller only for the SyncGroups that allow access.
	err := rSt.authorizeAndFilterSyncGroups(ctx)

	// Check error from phase 1.
	if err != nil {
		return err
	}

	if len(rSt.initVec) == 0 {
		return verror.New(verror.ErrInternal, ctx, "empty initiator generation vector")
	}

	// Phase 2 and 3 of sendDeltas: diff contains the bound on the
	// generations missing from the initiator per device.
	if rSt.sg {
		err = rSt.sendSgDeltas(ctx)
	} else {
		err = rSt.sendDataDeltas(ctx)
	}

	return err
}

// authorizeAndFilterSyncGroups authorizes the initiator against the requested
// SyncGroups and filters the initiator's prefixes to only include those from
// allowed SyncGroups (phase 1 of sendDeltas).
func (rSt *responderState) authorizeAndFilterSyncGroups(ctx *context.T) error {
	var err error
	rSt.st, err = rSt.sync.getDbStore(ctx, nil, rSt.appName, rSt.dbName)
	if err != nil {
		return err
	}

	allowedPfxs := make(map[string]struct{})
	for sgid := range rSt.sgIds {
		// Check permissions for the SyncGroup.
		var sg *interfaces.SyncGroup
		sg, err = getSyncGroupById(ctx, rSt.st, sgid)
		if err != nil {
			vlog.Errorf("sync: authorizeAndFilterSyncGroups: accessing SyncGroup information failed %v, err %v", sgid, err)
			continue
		}
		err = authorize(ctx, rSt.call.Security(), sg)
		if verror.ErrorID(err) == verror.ErrNoAccess.ID {
			if rSt.sg {
				id := fmt.Sprintf("%d", sgid)
				delete(rSt.initVec, id)
			}
			continue
		} else if err != nil {
			return err
		}

		for _, p := range sg.Spec.Prefixes {
			allowedPfxs[p] = struct{}{}
		}

		// Add the initiator to the SyncGroup membership if not already
		// in it.  It is a temporary solution until SyncGroup metadata
		// is synchronized peer to peer.
		// TODO(rdaoud): remove this when SyncGroups are synced.
		rSt.addInitiatorToSyncGroup(ctx, sgid)
	}

	if err != nil {
		return err
	}

	if rSt.sg {
		return nil
	}

	// Filter the initiator's prefixes to what is allowed.
	for pfx := range rSt.initVec {
		if _, ok := allowedPfxs[pfx]; ok {
			continue
		}
		allowed := false
		for p := range allowedPfxs {
			if strings.HasPrefix(pfx, p) {
				allowed = true
			}
		}

		if !allowed {
			delete(rSt.initVec, pfx)
		}
	}
	return nil
}

// addInitiatorToSyncGroup adds the request initiator to the membership of the
// given SyncGroup if the initiator is not already a member.  It is a temporary
// solution until SyncGroup metadata starts being synchronized, at which time
// peers will learn of new members through mutations of the SyncGroup metadata
// by the SyncGroup administrators.
// Note: the joiner metadata is fake because the responder does not have it.
func (rSt *responderState) addInitiatorToSyncGroup(ctx *context.T, gid interfaces.GroupId) {
	if rSt.initiator == "" {
		return
	}

	err := store.RunInTransaction(rSt.st, func(tx store.Transaction) error {
		version, err := getSyncGroupVersion(ctx, tx, gid)
		if err != nil {
			return err
		}
		sg, err := getSGDataEntry(ctx, tx, gid, version)
		if err != nil {
			return err
		}

		// If the initiator is already a member of the SyncGroup abort
		// the transaction with a special error code.
		if _, ok := sg.Joiners[rSt.initiator]; ok {
			return verror.New(verror.ErrExist, ctx, "member already in SyncGroup")
		}

		vlog.VI(4).Infof("sync: addInitiatorToSyncGroup: add %s to sgid %d", rSt.initiator, gid)
		sg.Joiners[rSt.initiator] = wire.SyncGroupMemberInfo{SyncPriority: 1}
		return setSGDataEntry(ctx, tx, gid, version, sg)
	})

	if err != nil && verror.ErrorID(err) != verror.ErrExist.ID {
		vlog.Errorf("sync: addInitiatorToSyncGroup: initiator %s, sgid %d: %v", rSt.initiator, gid, err)
	}
}

// sendSgDeltas computes the bound on missing generations, and sends the missing
// log records across all requested SyncGroups (phases 2 and 3 of sendDeltas).
func (rSt *responderState) sendSgDeltas(ctx *context.T) error {
	vlog.VI(3).Infof("sync: sendSgDeltas: %s, %s: sgids %v, genvec %v",
		rSt.appName, rSt.dbName, rSt.sgIds, rSt.initVec)

	respVec, _, err := rSt.sync.copyDbGenInfo(ctx, rSt.appName, rSt.dbName, rSt.sgIds)
	if err != nil {
		return err
	}

	rSt.outVec = make(interfaces.GenVector)

	for sg, initpgv := range rSt.initVec {
		respgv, ok := respVec[sg]
		if !ok {
			continue
		}
		rSt.diff = make(genRangeVector)
		rSt.diffPrefixGenVectors(respgv, initpgv)
		rSt.outVec[sg] = respgv

		if err := rSt.filterAndSendDeltas(ctx, sg); err != nil {
			return err
		}
	}
	return rSt.sendGenVec(ctx)
}

// sendDataDeltas computes the bound on missing generations across all requested
// prefixes, and sends the missing log records (phases 2 and 3 of sendDeltas).
func (rSt *responderState) sendDataDeltas(ctx *context.T) error {
	// Phase 2 of sendDeltas: Compute the missing generations.
	if err := rSt.computeDataDeltas(ctx); err != nil {
		return err
	}

	// Phase 3 of sendDeltas: Process the diff, filtering out records that
	// are not needed, and send the remainder on the wire ordered.
	if err := rSt.filterAndSendDeltas(ctx, logDataPrefix); err != nil {
		return err
	}
	return rSt.sendGenVec(ctx)
}

func (rSt *responderState) computeDataDeltas(ctx *context.T) error {
	respVec, respGen, err := rSt.sync.copyDbGenInfo(ctx, rSt.appName, rSt.dbName, nil)
	if err != nil {
		return err
	}
	respPfxs := extractAndSortPrefixes(respVec)
	initPfxs := extractAndSortPrefixes(rSt.initVec)

	rSt.outVec = make(interfaces.GenVector)
	rSt.diff = make(genRangeVector)
	pfx := initPfxs[0]

	for _, p := range initPfxs {
		if strings.HasPrefix(p, pfx) && p != pfx {
			continue
		}

		// Process this prefix as this is the start of a new set of
		// nested prefixes.
		pfx = p

		// Lower bound on initiator's knowledge for this prefix set.
		initpgv := rSt.initVec[pfx]

		// Find the relevant responder prefixes and add the corresponding knowledge.
		var respgv interfaces.PrefixGenVector
		var rpStart string
		for _, rp := range respPfxs {
			if !strings.HasPrefix(rp, pfx) && !strings.HasPrefix(pfx, rp) {
				// No relationship with pfx.
				continue
			}

			if strings.HasPrefix(pfx, rp) {
				// If rp is a prefix of pfx, remember it because
				// it may be a potential starting point for the
				// responder's knowledge. The actual starting
				// point is the deepest prefix where rp is a
				// prefix of pfx.
				//
				// Say the initiator is looking for "foo", and
				// the responder has knowledge for "f" and "fo",
				// the responder's starting point will be the
				// prefix genvector for "fo". Similarly, if the
				// responder has knowledge for "foo", the
				// starting point will be the prefix genvector
				// for "foo".
				rpStart = rp
			} else {
				// If pfx is a prefix of rp, this knowledge must
				// be definitely sent to the initiator. Diff the
				// prefix genvectors to adjust the delta bound and
				// include in outVec.
				respgv = respVec[rp]
				rSt.diffPrefixGenVectors(respgv, initpgv)
				rSt.outVec[rp] = respgv
			}
		}

		// Deal with the starting point.
		if rpStart == "" {
			// No matching prefixes for pfx were found.
			respgv = make(interfaces.PrefixGenVector)
			respgv[rSt.sync.id] = respGen
		} else {
			respgv = respVec[rpStart]
		}
		rSt.diffPrefixGenVectors(respgv, initpgv)
		rSt.outVec[pfx] = respgv
	}

	vlog.VI(3).Infof("sync: computeDeltaBound: %s, %s: diff %v, outvec %v",
		rSt.appName, rSt.dbName, rSt.diff, rSt.outVec)
	return nil
}

// filterAndSendDeltas filters the computed delta to remove records already
// known by the initiator, and sends the resulting records to the initiator
// (phase 3 of sendDeltas).
func (rSt *responderState) filterAndSendDeltas(ctx *context.T, pfx string) error {
	// TODO(hpucha): Although ok for now to call SendStream once per
	// Database, would like to make this implementation agnostic.
	sender := rSt.call.SendStream()

	// First two phases were successful. So now on to phase 3. We now visit
	// every log record in the generation range as obtained from phase 1 in
	// their log order. We use a heap to incrementally sort the log records
	// as per their position in the log.
	//
	// Init the min heap, one entry per device in the diff.
	mh := make(minHeap, 0, len(rSt.diff))
	for dev, r := range rSt.diff {
		r.cur = r.min
		rec, err := getNextLogRec(ctx, rSt.st, pfx, dev, r)
		if err != nil {
			return err
		}
		if rec != nil {
			mh = append(mh, rec)
		} else {
			delete(rSt.diff, dev)
		}
	}
	heap.Init(&mh)

	// Process the log records in order.
	var initPfxs []string
	if !rSt.sg {
		initPfxs = extractAndSortPrefixes(rSt.initVec)
	}
	for mh.Len() > 0 {
		rec := heap.Pop(&mh).(*localLogRec)

		if rSt.sg || !filterLogRec(rec, rSt.initVec, initPfxs) {
			// Send on the wire.
			wireRec, err := makeWireLogRec(ctx, rSt.st, rec)
			if err != nil {
				return err
			}
			sender.Send(interfaces.DeltaRespRec{*wireRec})
		}

		// Add a new record from the same device if not done.
		dev := rec.Metadata.Id
		rec, err := getNextLogRec(ctx, rSt.st, pfx, dev, rSt.diff[dev])
		if err != nil {
			return err
		}
		if rec != nil {
			heap.Push(&mh, rec)
		} else {
			delete(rSt.diff, dev)
		}
	}
	return nil
}

func (rSt *responderState) sendGenVec(ctx *context.T) error {
	sender := rSt.call.SendStream()
	sender.Send(interfaces.DeltaRespRespVec{rSt.outVec})
	return nil
}

// genRange represents a range of generations (min and max inclusive).
type genRange struct {
	min uint64
	max uint64
	cur uint64
}

type genRangeVector map[uint64]*genRange

// diffPrefixGenVectors diffs two generation vectors, belonging to the responder
// and the initiator, and updates the range of generations per device known to
// the responder but not known to the initiator. "gens" (generation range) is
// passed in as an input argument so that it can be incrementally updated as the
// range of missing generations grows when different responder prefix genvectors
// are used to compute the diff.
//
// For example: Generation vector for responder is say RVec = {A:10, B:5, C:1},
// Generation vector for initiator is say IVec = {A:5, B:10, D:2}. Diffing these
// two vectors returns: {A:[6-10], C:[1-1]}.
//
// TODO(hpucha): Add reclaimVec for GCing.
func (rSt *responderState) diffPrefixGenVectors(respPVec, initPVec interfaces.PrefixGenVector) {
	// Compute missing generations for devices that are in both initiator's and responder's vectors.
	for devid, gen := range initPVec {
		rgen, ok := respPVec[devid]
		if ok {
			updateDevRange(devid, rgen, gen, rSt.diff)
		}
	}

	// Compute missing generations for devices not in initiator's vector but in responder's vector.
	for devid, rgen := range respPVec {
		if _, ok := initPVec[devid]; !ok {
			updateDevRange(devid, rgen, 0, rSt.diff)
		}
	}
}

func updateDevRange(devid, rgen, gen uint64, gens genRangeVector) {
	if gen < rgen {
		// Need to include all generations in the interval [gen+1,rgen], gen+1 and rgen inclusive.
		if r, ok := gens[devid]; !ok {
			gens[devid] = &genRange{min: gen + 1, max: rgen}
		} else {
			if gen+1 < r.min {
				r.min = gen + 1
			}
			if rgen > r.max {
				r.max = rgen
			}
		}
	}
}

func extractAndSortPrefixes(vec interfaces.GenVector) []string {
	pfxs := make([]string, len(vec))
	i := 0
	for p := range vec {
		pfxs[i] = p
		i++
	}
	sort.Strings(pfxs)
	return pfxs
}

// TODO(hpucha): This can be optimized using a scan instead of "gets" in a for
// loop.
func getNextLogRec(ctx *context.T, st store.Store, pfx string, dev uint64, r *genRange) (*localLogRec, error) {
	for i := r.cur; i <= r.max; i++ {
		rec, err := getLogRec(ctx, st, pfx, dev, i)
		if err == nil {
			r.cur = i + 1
			return rec, nil
		}
		if verror.ErrorID(err) != verror.ErrNoExist.ID {
			return nil, err
		}
	}
	return nil, nil
}

// Note: initPfxs is sorted.
func filterLogRec(rec *localLogRec, initVec interfaces.GenVector, initPfxs []string) bool {
	// The key starts with one of the store's reserved prefixes for managed
	// namespaces (e.g. $row, $perms).  Remove that prefix before comparing
	// it with the SyncGroup prefixes which are defined by the application.
	key := extractAppKey(rec.Metadata.ObjId)

	filter := true
	var maxGen uint64
	for _, p := range initPfxs {
		if strings.HasPrefix(key, p) {
			// Do not filter. Initiator is interested in this
			// prefix.
			filter = false

			// Track if the initiator knows of this record.
			gen := initVec[p][rec.Metadata.Id]
			if maxGen < gen {
				maxGen = gen
			}
		}
	}

	// Filter this record if the initiator already has it.
	if maxGen >= rec.Metadata.Gen {
		filter = true
	}

	return filter
}

// makeWireLogRec creates a sync log record to send on the wire from a given
// local sync record.
func makeWireLogRec(ctx *context.T, st store.Store, rec *localLogRec) (*interfaces.LogRec, error) {
	// Get the object value at the required version.
	key, version := rec.Metadata.ObjId, rec.Metadata.CurVers
	var value []byte
	if !rec.Metadata.Delete {
		var err error
		value, err = watchable.GetAtVersion(ctx, st, []byte(key), nil, []byte(version))
		if err != nil {
			return nil, err
		}
	}

	wireRec := &interfaces.LogRec{Metadata: rec.Metadata, Value: value}
	return wireRec, nil
}

// A minHeap implements heap.Interface and holds local log records.
type minHeap []*localLogRec

func (mh minHeap) Len() int { return len(mh) }

func (mh minHeap) Less(i, j int) bool {
	return mh[i].Pos < mh[j].Pos
}

func (mh minHeap) Swap(i, j int) {
	mh[i], mh[j] = mh[j], mh[i]
}

func (mh *minHeap) Push(x interface{}) {
	item := x.(*localLogRec)
	*mh = append(*mh, item)
}

func (mh *minHeap) Pop() interface{} {
	old := *mh
	n := len(old)
	item := old[n-1]
	*mh = old[0 : n-1]
	return item
}
