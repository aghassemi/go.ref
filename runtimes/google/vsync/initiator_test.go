package vsync

// Tests for sync initiator.
import (
	"os"
	"reflect"
	"testing"
	"time"

	"veyron/services/store/raw"

	"veyron2/storage"
)

// TestGetLogRec tests getting a log record from kvdb based on object id and version.
func TestGetLogRec(t *testing.T) {
	dir, err := createTempDir()
	if err != nil {
		t.Errorf("Could not create tempdir %v", err)
	}
	// Set a large value to prevent the threads from firing.
	peerSyncInterval = 1 * time.Hour
	garbageCollectInterval = 1 * time.Hour
	s := NewSyncd("", "", "VeyronTab", dir, "", 0)

	s.lock.Lock()
	defer s.lock.Unlock()
	defer s.Close()
	defer os.RemoveAll(dir)

	// Create some data.
	objID := storage.NewID()
	expRec := &LogRec{
		DevID:   "VeyronTab",
		GNum:    50,
		LSN:     100,
		ObjID:   objID,
		CurVers: 20,
		Value:   LogValue{Mutation: raw.Mutation{Version: 20}},
	}
	if _, err := s.hdlInitiator.getLogRec(objID, expRec.CurVers); err == nil {
		t.Errorf("GetLogRec didn't fail")
	}
	logKey, err := s.log.putLogRec(expRec)
	if err != nil {
		t.Errorf("PutLogRec failed with err %v", err)
	}
	if _, err := s.hdlInitiator.getLogRec(objID, expRec.CurVers); err == nil {
		t.Errorf("GetLogRec didn't fail")
	}
	if err = s.dag.addNode(objID, expRec.CurVers, false, expRec.Parents, logKey); err != nil {
		t.Errorf("AddNode failed with err %v", err)
	}
	curRec, err := s.hdlInitiator.getLogRec(objID, expRec.CurVers)
	if err != nil {
		t.Errorf("GetLogRec failed with err %v", err)
	}
	if !reflect.DeepEqual(curRec, expRec) {
		t.Errorf("Data mismatch for %v instead of %v",
			curRec, expRec)
	}
}

// TestResolveConflictByTime tests the timestamp-based conflict resolution policy.
func TestResolveConflictByTime(t *testing.T) {
	dir, err := createTempDir()
	if err != nil {
		t.Errorf("Could not create tempdir %v", err)
	}
	// Set a large value to prevent the threads from firing.
	// Test is not thread safe.
	peerSyncInterval = 1 * time.Hour
	garbageCollectInterval = 1 * time.Hour
	s := NewSyncd("", "", "VeyronTab", dir, "", 0)

	defer s.Close()
	defer os.RemoveAll(dir)

	objID := storage.NewID()
	s.hdlInitiator.updObjects[objID] = &objConflictState{
		isConflict: true,
		oldHead:    40,
		newHead:    20,
		ancestor:   10,
	}
	versions := []storage.Version{10, 40, 20}
	for _, v := range versions {
		expRec := &LogRec{
			DevID:   "VeyronTab",
			GNum:    GenID(50 + v),
			LSN:     LSN(100 + v),
			ObjID:   objID,
			CurVers: v,
			Value:   LogValue{Mutation: raw.Mutation{Version: v, PriorVersion: 500 + v}, SyncTime: int64(v)},
		}
		logKey, err := s.log.putLogRec(expRec)
		if err != nil {
			t.Errorf("PutLogRec failed with err %v", err)
		}
		if err = s.dag.addNode(objID, expRec.CurVers, false, expRec.Parents, logKey); err != nil {
			t.Errorf("AddNode failed with err %v", err)
		}
	}

	if err := s.hdlInitiator.resolveConflictsByTime(); err != nil {
		t.Errorf("ResolveConflictsByTime failed with err %v", err)
	}
	if s.hdlInitiator.updObjects[objID].resolvVal.Mutation.PriorVersion != 540 {
		t.Errorf("Data mismatch for resolution %v", s.hdlInitiator.updObjects[objID].resolvVal)
	}
}

// TODO(hpucha): Add more tests around retrying failed puts in the next pass (processUpdatedObjects).
// TestLogStreamRemoteOnly tests processing of a remote log stream.
func TestLogStreamRemoteOnly(t *testing.T) {
	dir, err := createTempDir()
	if err != nil {
		t.Errorf("Could not create tempdir %v", err)
	}
	// Set a large value to prevent the threads from firing.
	// Test is not thread safe.
	peerSyncInterval = 1 * time.Hour
	garbageCollectInterval = 1 * time.Hour
	s := NewSyncd("", "", "VeyronTab", dir, "", 0)

	defer s.Close()
	defer os.RemoveAll(dir)

	stream, err := createReplayStream("remote-init-00.log.sync")
	if err != nil {
		t.Fatalf("createReplayStream failed with err %v", err)
	}
	var minGens GenVector
	if minGens, err = s.hdlInitiator.processLogStream(stream); err != nil {
		t.Fatalf("processLogStream failed with err %v", err)
	}

	// Check minGens.
	expVec := GenVector{"VeyronPhone": 1}
	if !reflect.DeepEqual(expVec, minGens) {
		t.Errorf("Data mismatch for minGens: %v instead of %v",
			minGens, expVec)
	}

	// Check generation metadata.
	curVal, err := s.log.getGenMetadata("VeyronPhone", 1)
	if err != nil || curVal == nil {
		t.Fatalf("GetGenMetadata() can not find object in log file err %v", err)
	}
	expVal := &genMetadata{Pos: 0, Count: 3, MaxLSN: 2}
	if !reflect.DeepEqual(expVal, curVal) {
		t.Errorf("Data mismatch for generation metadata: %v instead of %v",
			curVal, expVal)
	}

	objid, err := strToObjID("12345")
	if err != nil {
		t.Errorf("Could not create objid %v", err)
	}
	// Check all log records.
	for i := LSN(0); i < 3; i++ {
		curRec, err := s.log.getLogRec("VeyronPhone", GenID(1), i)
		if err != nil || curRec == nil {
			t.Fatalf("GetLogRec() can not find object %d in log file err %v",
				i, err)
		}
		if curRec.ObjID != objid {
			t.Errorf("Data mismatch in log record %v", curRec)
		}
		// Verify DAG state.
		if _, err := s.dag.getNode(objid, storage.Version(i)); err != nil {
			t.Errorf("GetNode() can not find object %d %d in DAG, err %v", objid, i, err)
		}
	}
	if err := s.hdlInitiator.detectConflicts(); err != nil {
		t.Fatalf("detectConflicts failed with err %v", err)
	}
	if len(s.hdlInitiator.updObjects) != 1 {
		t.Errorf("Unexpected number of updated objects %d", len(s.hdlInitiator.updObjects))
	}
	st := s.hdlInitiator.updObjects[objid]
	if st.isConflict {
		t.Errorf("Detected a conflict %v", st)
	}
	if st.newHead != 2 || st.oldHead != storage.NoVersion {
		t.Errorf("Conflict detection didn't succeed %v", st)
	}
	if err := s.hdlInitiator.resolveConflicts(); err != nil {
		t.Fatalf("resolveConflicts failed with err %v", err)
	}
	if err := s.hdlInitiator.updateStoreAndSync(nil, GenVector{}, minGens, GenVector{}, "VeyronPhone"); err != nil {
		t.Fatalf("updateStoreAndSync failed with err %v", err)
	}
	if st.resolvVal.Mutation.PriorVersion != storage.NoVersion || st.resolvVal.Mutation.Version != 2 {
		t.Errorf("Mutation generation is not accurate %v", st)
	}
	if s.log.head.Curgen != 1 || s.log.head.Curlsn != 0 || s.log.head.Curorder != 1 {
		t.Errorf("Data mismatch in log header %v", s.log.head)
	}
	// Verify DAG state.
	if head, err := s.dag.getHead(objid); err != nil || head != 2 {
		t.Errorf("Invalid object %d head in DAG %s, err %v", objid, head, err)
	}
}

// TestLogStreamGCedRemote tests that a remote log stream can be
// correctly applied when its generations don't start at 1 and have
// been GC'ed already. Commands are in file
// testdata/remote-init-01.log.sync.
func TestLogStreamGCedRemote(t *testing.T) {
	dir, err := createTempDir()
	if err != nil {
		t.Errorf("Could not create tempdir %v", err)
	}
	// Set a large value to prevent the threads from firing.
	// Test is not thread safe.
	peerSyncInterval = 1 * time.Hour
	garbageCollectInterval = 1 * time.Hour
	s := NewSyncd("", "", "VeyronTab", dir, "", 0)

	defer s.Close()
	defer os.RemoveAll(dir)

	stream, err := createReplayStream("remote-init-01.log.sync")
	if err != nil {
		t.Fatalf("createReplayStream failed with err %v", err)
	}

	var minGens GenVector
	if minGens, err = s.hdlInitiator.processLogStream(stream); err != nil {
		t.Fatalf("processLogStream failed with err %v", err)
	}

	// Check minGens.
	expVec := GenVector{"VeyronPhone": 5}
	if !reflect.DeepEqual(expVec, minGens) {
		t.Errorf("Data mismatch for minGens: %v instead of %v",
			minGens, expVec)
	}

	// Check generation metadata.
	curVal, err := s.log.getGenMetadata("VeyronPhone", 5)
	if err != nil || curVal == nil {
		t.Fatalf("GetGenMetadata() can not find object in log file err %v", err)
	}
	expVal := &genMetadata{Pos: 0, Count: 3, MaxLSN: 2}
	if !reflect.DeepEqual(expVal, curVal) {
		t.Errorf("Data mismatch for generation metadata: %v instead of %v",
			curVal, expVal)
	}

	objid, err := strToObjID("12345")
	if err != nil {
		t.Errorf("Could not create objid %v", err)
	}
	// Check all log records.
	for i := LSN(0); i < 3; i++ {
		curRec, err := s.log.getLogRec("VeyronPhone", GenID(5), i)
		if err != nil || curRec == nil {
			t.Fatalf("GetLogRec() can not find object %d in log file err %v",
				i, err)
		}
		if curRec.ObjID != objid {
			t.Errorf("Data mismatch in log record %v", curRec)
		}
		// Verify DAG state.
		if _, err := s.dag.getNode(objid, storage.Version(i)); err != nil {
			t.Errorf("GetNode() can not find object %d %d in DAG, err %v", objid, i, err)
		}
	}

	if err := s.hdlInitiator.detectConflicts(); err != nil {
		t.Fatalf("detectConflicts failed with err %v", err)
	}
	if len(s.hdlInitiator.updObjects) != 1 {
		t.Errorf("Unexpected number of updated objects %d", len(s.hdlInitiator.updObjects))
	}
	st := s.hdlInitiator.updObjects[objid]
	if st.isConflict {
		t.Errorf("Detected a conflict %v", st)
	}
	if st.newHead != 2 || st.oldHead != storage.NoVersion {
		t.Errorf("Conflict detection didn't succeed %v", st)
	}
	if err := s.hdlInitiator.resolveConflicts(); err != nil {
		t.Fatalf("resolveConflicts failed with err %v", err)
	}
	if err := s.hdlInitiator.updateStoreAndSync(nil, GenVector{}, minGens, GenVector{}, "VeyronPhone"); err != nil {
		t.Fatalf("updateStoreAndSync failed with err %v", err)
	}
	if st.resolvVal.Mutation.PriorVersion != storage.NoVersion || st.resolvVal.Mutation.Version != 2 {
		t.Errorf("Mutation generation is not accurate %v", st)
	}
	if s.log.head.Curgen != 1 || s.log.head.Curlsn != 0 || s.log.head.Curorder != 1 {
		t.Errorf("Data mismatch in log header %v", s.log.head)
	}
	// Verify DAG state.
	if head, err := s.dag.getHead(objid); err != nil || head != 2 {
		t.Errorf("Invalid object %d head in DAG %s, err %v", objid, head, err)
	}
}

// TestLogStreamNoConflict tests that a local and a remote log stream
// can be correctly applied (when there are no conflicts).  Commands
// are in files
// testdata/<local-init-00.sync,remote-noconf-00.log.sync>.
func TestLogStreamNoConflict(t *testing.T) {
	dir, err := createTempDir()
	if err != nil {
		t.Errorf("Could not create tempdir %v", err)
	}
	// Set a large value to prevent the threads from firing.
	// Test is not thread safe.
	peerSyncInterval = 1 * time.Hour
	garbageCollectInterval = 1 * time.Hour
	s := NewSyncd("", "", "VeyronTab", dir, "", 0)

	defer s.Close()
	defer os.RemoveAll(dir)

	if _, err = logReplayCommands(s.log, "local-init-00.sync"); err != nil {
		t.Error(err)
	}

	stream, err := createReplayStream("remote-noconf-00.log.sync")
	if err != nil {
		t.Fatalf("createReplayStream failed with err %v", err)
	}

	var minGens GenVector
	if minGens, err = s.hdlInitiator.processLogStream(stream); err != nil {
		t.Fatalf("processLogStream failed with err %v", err)
	}

	// Check minGens.
	expVec := GenVector{"VeyronPhone": 1}
	if !reflect.DeepEqual(expVec, minGens) {
		t.Errorf("Data mismatch for minGens: %v instead of %v",
			minGens, expVec)
	}

	// Check generation metadata.
	curVal, err := s.log.getGenMetadata("VeyronPhone", 1)
	if err != nil || curVal == nil {
		t.Fatalf("GetGenMetadata() can not find object in log file for VeyronPhone err %v", err)
	}
	expVal := &genMetadata{Pos: 0, Count: 3, MaxLSN: 2}
	if !reflect.DeepEqual(expVal, curVal) {
		t.Errorf("Data mismatch for generation metadata: %v instead of %v",
			curVal, expVal)
	}

	objid, err := strToObjID("12345")
	if err != nil {
		t.Errorf("Could not create objid %v", err)
	}
	// Check all log records.
	for _, devid := range []DeviceID{"VeyronPhone", "VeyronTab"} {
		var v storage.Version
		for i := LSN(0); i < 3; i++ {
			curRec, err := s.log.getLogRec(devid, GenID(1), i)
			if err != nil || curRec == nil {
				t.Fatalf("GetLogRec() can not find object %s:%d in log file err %v",
					devid, i, err)
			}
			if curRec.ObjID != objid {
				t.Errorf("Data mismatch in log record %v", curRec)
			}
			// Verify DAG state.
			if _, err := s.dag.getNode(objid, v); err != nil {
				t.Errorf("GetNode() can not find object %d %d in DAG, err %v", objid, i, err)
			}
			v = v + 1
		}
	}
	if err := s.hdlInitiator.detectConflicts(); err != nil {
		t.Fatalf("detectConflicts failed with err %v", err)
	}
	if len(s.hdlInitiator.updObjects) != 1 {
		t.Errorf("Unexpected number of updated objects %d", len(s.hdlInitiator.updObjects))
	}
	st := s.hdlInitiator.updObjects[objid]
	if st.isConflict {
		t.Errorf("Detected a conflict %v", st)
	}
	if st.newHead != 5 || st.oldHead != 2 {
		t.Errorf("Conflict detection didn't succeed %v", st)
	}
	if err := s.hdlInitiator.resolveConflicts(); err != nil {
		t.Fatalf("resolveConflicts failed with err %v", err)
	}
	if err := s.hdlInitiator.updateStoreAndSync(nil, GenVector{}, minGens, GenVector{}, "VeyronPhone"); err != nil {
		t.Fatalf("updateStoreAndSync failed with err %v", err)
	}
	if st.resolvVal.Mutation.PriorVersion != 2 || st.resolvVal.Mutation.Version != 5 {
		t.Errorf("Mutation generation is not accurate %v", st)
	}
	if s.log.head.Curgen != 1 || s.log.head.Curlsn != 3 || s.log.head.Curorder != 1 {
		t.Errorf("Data mismatch in log header %v", s.log.head)
	}
	// Verify DAG state.
	if head, err := s.dag.getHead(objid); err != nil || head != 5 {
		t.Errorf("Invalid object %d head in DAG %s, err %v", objid, head, err)
	}
}

// TestLogStreamConflict tests that a local and a remote log stream
// can be correctly applied (when there are conflicts). Commands are
// in files testdata/<local-init-00.sync,remote-conf-00.log.sync>.
func TestLogStreamConflict(t *testing.T) {
	dir, err := createTempDir()
	if err != nil {
		t.Errorf("Could not create tempdir %v", err)
	}
	// Set a large value to prevent the threads from firing.
	// Test is not thread safe.
	peerSyncInterval = 1 * time.Hour
	garbageCollectInterval = 1 * time.Hour
	conflictResolutionPolicy = useTime
	s := NewSyncd("", "", "VeyronTab", dir, "", 0)

	defer s.Close()
	defer os.RemoveAll(dir)

	if _, err = logReplayCommands(s.log, "local-init-00.sync"); err != nil {
		t.Error(err)
	}

	stream, err := createReplayStream("remote-conf-00.log.sync")
	if err != nil {
		t.Fatalf("createReplayStream failed with err %v", err)
	}

	var minGens GenVector
	if minGens, err = s.hdlInitiator.processLogStream(stream); err != nil {
		t.Fatalf("processLogStream failed with err %v", err)
	}

	// Check minGens.
	expVec := GenVector{"VeyronPhone": 1}
	if !reflect.DeepEqual(expVec, minGens) {
		t.Errorf("Data mismatch for minGens: %v instead of %v",
			minGens, expVec)
	}

	// Check generation metadata.
	curVal, err := s.log.getGenMetadata("VeyronPhone", 1)
	if err != nil || curVal == nil {
		t.Fatalf("GetGenMetadata() can not find object in log file for VeyronPhone err %v", err)
	}
	expVal := &genMetadata{Pos: 0, Count: 3, MaxLSN: 2}
	if !reflect.DeepEqual(expVal, curVal) {
		t.Errorf("Data mismatch for generation metadata: %v instead of %v",
			curVal, expVal)
	}
	if err := s.hdlInitiator.detectConflicts(); err != nil {
		t.Fatalf("detectConflicts failed with err %v", err)
	}
	if err := s.hdlInitiator.resolveConflicts(); err != nil {
		t.Fatalf("resolveConflicts failed with err %v", err)
	}
	if err := s.hdlInitiator.updateStoreAndSync(nil, GenVector{}, minGens, GenVector{}, "VeyronPhone"); err != nil {
		t.Fatalf("updateStoreAndSync failed with err %v", err)
	}

	objid, err := strToObjID("12345")
	if err != nil {
		t.Errorf("Could not create objid %v", err)
	}
	lcount := []LSN{3, 4}
	// Check all log records.
	for index, devid := range []DeviceID{"VeyronPhone", "VeyronTab"} {
		var v storage.Version
		for i := LSN(0); i < lcount[index]; i++ {
			curRec, err := s.log.getLogRec(devid, GenID(1), i)
			if err != nil || curRec == nil {
				t.Fatalf("GetLogRec() can not find object %s:%d in log file err %v",
					devid, i, err)
			}
			if curRec.ObjID != objid {
				t.Errorf("Data mismatch in log record %v", curRec)
			}
			if devid == "VeyronTab" && index == 3 && curRec.RecType != LinkRec {
				t.Errorf("Data mismatch in log record %v", curRec)
			}
			// Verify DAG state.
			if _, err := s.dag.getNode(objid, v); err != nil {
				t.Errorf("GetNode() can not find object %d %d in DAG, err %v", objid, i, err)
			}
			v = v + 1
		}
	}
	if len(s.hdlInitiator.updObjects) != 1 {
		t.Errorf("Unexpected number of updated objects %d", len(s.hdlInitiator.updObjects))
	}
	st := s.hdlInitiator.updObjects[objid]
	if !st.isConflict {
		t.Errorf("Didn't detect a conflict %v", st)
	}
	if st.newHead != 5 || st.oldHead != 2 {
		t.Errorf("Conflict detection didn't succeed %v", st)
	}
	if st.resolvVal.Mutation.PriorVersion != 2 || st.resolvVal.Mutation.Version != 5 {
		t.Errorf("Mutation generation is not accurate %v", st)
	}
	// Curlsn == 4 for the log record that resolves conflict.
	if s.log.head.Curgen != 1 || s.log.head.Curlsn != 4 || s.log.head.Curorder != 1 {
		t.Errorf("Data mismatch in log header %v", s.log.head)
	}
	// Verify DAG state.
	if head, err := s.dag.getHead(objid); err != nil || head != 5 {
		t.Errorf("Invalid object %d head in DAG %s, err %v", objid, head, err)
	}
}

// TestMultipleLogStream tests that a local and 2 remote log streams
// can be correctly applied (when there are conflicts). Commands are
// in file testdata/<local-init-00.sync,remote-conf-01.log.sync>.
func TestMultipleLogStream(t *testing.T) {
	dir, err := createTempDir()
	if err != nil {
		t.Errorf("Could not create tempdir %v", err)
	}
	// Set a large value to prevent the threads from firing.
	// Test is not thread safe.
	peerSyncInterval = 1 * time.Hour
	garbageCollectInterval = 1 * time.Hour
	conflictResolutionPolicy = useTime
	s := NewSyncd("", "", "VeyronTab", dir, "", 0)

	defer s.Close()
	defer os.RemoveAll(dir)

	if _, err = logReplayCommands(s.log, "local-init-00.sync"); err != nil {
		t.Error(err)
	}

	stream, err := createReplayStream("remote-conf-01.log.sync")
	if err != nil {
		t.Fatalf("createReplayStream failed with err %v", err)
	}

	var minGens GenVector
	if minGens, err = s.hdlInitiator.processLogStream(stream); err != nil {
		t.Fatalf("processLogStream failed with err %v", err)
	}

	// Check minGens.
	expVec := GenVector{"VeyronPhone": 1, "VeyronLaptop": 1}
	if !reflect.DeepEqual(expVec, minGens) {
		t.Errorf("Data mismatch for minGens: %v instead of %v",
			minGens, expVec)
	}

	// Check generation metadata.
	curVal, err := s.log.getGenMetadata("VeyronLaptop", 1)
	if err != nil || curVal == nil {
		t.Fatalf("GetGenMetadata() can not find object in log file for VeyronPhone err %v", err)
	}
	expVal := &genMetadata{Pos: 0, Count: 1, MaxLSN: 0}
	if !reflect.DeepEqual(expVal, curVal) {
		t.Errorf("Data mismatch for generation metadata: %v instead of %v",
			curVal, expVal)
	}

	curVal, err = s.log.getGenMetadata("VeyronPhone", 1)
	if err != nil || curVal == nil {
		t.Fatalf("GetGenMetadata() can not find object in log file for VeyronPhone err %v", err)
	}
	expVal.Pos = 1
	expVal.Count = 2
	expVal.MaxLSN = 1
	if !reflect.DeepEqual(expVal, curVal) {
		t.Errorf("Data mismatch for generation metadata: %v instead of %v",
			curVal, expVal)
	}

	if err := s.hdlInitiator.detectConflicts(); err != nil {
		t.Fatalf("detectConflicts failed with err %v", err)
	}
	if err := s.hdlInitiator.resolveConflicts(); err != nil {
		t.Fatalf("resolveConflicts failed with err %v", err)
	}
	if err := s.hdlInitiator.updateStoreAndSync(nil, GenVector{}, minGens, GenVector{}, "VeyronPhone"); err != nil {
		t.Fatalf("updateStoreAndSync failed with err %v", err)
	}

	objid, err := strToObjID("12345")
	if err != nil {
		t.Errorf("Could not create objid %v", err)
	}
	// Check all log records.
	lcount := []LSN{2, 4, 1}
	for index, devid := range []DeviceID{"VeyronPhone", "VeyronTab", "VeyronLaptop"} {
		var v storage.Version
		for i := LSN(0); i < lcount[index]; i++ {
			curRec, err := s.log.getLogRec(devid, GenID(1), i)
			if err != nil || curRec == nil {
				t.Fatalf("GetLogRec() can not find object %s:%d in log file err %v",
					devid, i, err)
			}
			if curRec.ObjID != objid {
				t.Errorf("Data mismatch in log record %v", curRec)
			}
			if devid == "VeyronTab" && index == 3 && curRec.RecType != LinkRec {
				t.Errorf("Data mismatch in log record %v", curRec)
			}
			// Verify DAG state.
			if _, err := s.dag.getNode(objid, v); err != nil {
				t.Errorf("GetNode() can not find object %d %d in DAG, err %v", objid, i, err)
			}
			v = v + 1
		}
	}
	if len(s.hdlInitiator.updObjects) != 1 {
		t.Errorf("Unexpected number of updated objects %d", len(s.hdlInitiator.updObjects))
	}
	st := s.hdlInitiator.updObjects[objid]
	if !st.isConflict {
		t.Errorf("Didn't detect a conflict %v", st)
	}
	if st.newHead != 5 || st.oldHead != 2 {
		t.Errorf("Conflict detection didn't succeed %v", st)
	}
	if st.resolvVal.Mutation.PriorVersion != 2 || st.resolvVal.Mutation.Version != 5 {
		t.Errorf("Mutation generation is not accurate %v", st)
	}
	// Curlsn == 4 for the log record that resolves conflict.
	if s.log.head.Curgen != 1 || s.log.head.Curlsn != 4 || s.log.head.Curorder != 2 {
		t.Errorf("Data mismatch in log header %v", s.log.head)
	}
	// Verify DAG state.
	if head, err := s.dag.getHead(objid); err != nil || head != 5 {
		t.Errorf("Invalid object %d head in DAG %s, err %v", objid, head, err)
	}
}

// TestInitiatorBlessNoConf0 tests that a local and a remote log
// record stream can be correctly applied, when the conflict is
// resolved by a blessing. In this test, local head of the object is
// unchanged at the end of replay. Commands are in files
// testdata/<local-init-00.sync,remote-noconf-link-00.log.sync>.
func TestInitiatorBlessNoConf0(t *testing.T) {
	dir, err := createTempDir()
	if err != nil {
		t.Errorf("Could not create tempdir %v", err)
	}
	// Set a large value to prevent the threads from firing.
	// Test is not thread safe.
	peerSyncInterval = 1 * time.Hour
	garbageCollectInterval = 1 * time.Hour
	s := NewSyncd("", "", "VeyronTab", dir, "", 0)

	defer s.Close()
	defer os.RemoveAll(dir)

	if _, err = logReplayCommands(s.log, "local-init-00.sync"); err != nil {
		t.Error(err)
	}
	stream, err := createReplayStream("remote-noconf-link-00.log.sync")
	if err != nil {
		t.Fatalf("createReplayStream failed with err %v", err)
	}

	var minGens GenVector
	if minGens, err = s.hdlInitiator.processLogStream(stream); err != nil {
		t.Fatalf("processLogStream failed with err %v", err)
	}
	if err := s.hdlInitiator.detectConflicts(); err != nil {
		t.Fatalf("detectConflicts failed with err %v", err)
	}
	// Check that there are no conflicts.
	if len(s.hdlInitiator.updObjects) != 1 {
		t.Errorf("Too many objects %v", len(s.hdlInitiator.updObjects))
	}
	objid, err := strToObjID("12345")
	if err != nil {
		t.Errorf("Could not create objid %v", err)
	}
	st := s.hdlInitiator.updObjects[objid]
	if st.isConflict {
		t.Errorf("Detected a conflict %v", st)
	}
	if st.newHead != 2 || st.oldHead != 2 {
		t.Errorf("Conflict detection didn't succeed %v", st)
	}

	if err := s.hdlInitiator.resolveConflicts(); err != nil {
		t.Fatalf("resolveConflicts failed with err %v", err)
	}
	if err := s.hdlInitiator.updateStoreAndSync(nil, GenVector{}, minGens, GenVector{}, "VeyronPhone"); err != nil {
		t.Fatalf("updateStoreAndSync failed with err %v", err)
	}
	if st.resolvVal.Mutation.Version != 2 {
		t.Errorf("Mutation generation is not accurate %v", st)
	}
	// No new log records should be added.
	if s.log.head.Curgen != 1 || s.log.head.Curlsn != 3 || s.log.head.Curorder != 1 {
		t.Errorf("Data mismatch in log header %v", s.log.head)
	}
	// Verify DAG state.
	if head, err := s.dag.getHead(objid); err != nil || head != 2 {
		t.Errorf("Invalid object %d head in DAG %s, err %v", objid, head, err)
	}
}

// TestInitiatorBlessNoConf1 tests that a local and a remote log
// record stream can be correctly applied, when the conflict is
// resolved by a blessing. In this test, local head of the object is
// updated at the end of the replay. Commands are in files
// testdata/<local-init-00.sync,remote-noconf-link-01.log.sync>.
func TestInitiatorBlessNoConf1(t *testing.T) {
	dir, err := createTempDir()
	if err != nil {
		t.Errorf("Could not create tempdir %v", err)
	}
	// Set a large value to prevent the threads from firing.
	// Test is not thread safe.
	peerSyncInterval = 1 * time.Hour
	garbageCollectInterval = 1 * time.Hour
	s := NewSyncd("", "", "VeyronTab", dir, "", 0)

	defer s.Close()
	defer os.RemoveAll(dir)

	if _, err = logReplayCommands(s.log, "local-init-00.sync"); err != nil {
		t.Error(err)
	}
	stream, err := createReplayStream("remote-noconf-link-01.log.sync")
	if err != nil {
		t.Fatalf("createReplayStream failed with err %v", err)
	}

	var minGens GenVector
	if minGens, err = s.hdlInitiator.processLogStream(stream); err != nil {
		t.Fatalf("processLogStream failed with err %v", err)
	}
	if err := s.hdlInitiator.detectConflicts(); err != nil {
		t.Fatalf("detectConflicts failed with err %v", err)
	}
	// Check that there are no conflicts.
	if len(s.hdlInitiator.updObjects) != 1 {
		t.Errorf("Too many objects %v", len(s.hdlInitiator.updObjects))
	}
	objid, err := strToObjID("12345")
	if err != nil {
		t.Errorf("Could not create objid %v", err)
	}
	st := s.hdlInitiator.updObjects[objid]
	if st.isConflict {
		t.Errorf("Detected a conflict %v", st)
	}
	if st.newHead != 3 || st.oldHead != 2 {
		t.Errorf("Conflict detection didn't succeed %v", st)
	}

	if err := s.hdlInitiator.resolveConflicts(); err != nil {
		t.Fatalf("resolveConflicts failed with err %v", err)
	}
	if err := s.hdlInitiator.updateStoreAndSync(nil, GenVector{}, minGens, GenVector{}, "VeyronPhone"); err != nil {
		t.Fatalf("updateStoreAndSync failed with err %v", err)
	}
	if st.resolvVal.Mutation.Version != 3 || st.resolvVal.Mutation.PriorVersion != 2 {
		t.Errorf("Mutation generation is not accurate %v", st)
	}
	// No new log records should be added.
	if s.log.head.Curgen != 1 || s.log.head.Curlsn != 3 || s.log.head.Curorder != 1 {
		t.Errorf("Data mismatch in log header %v", s.log.head)
	}
	// Verify DAG state.
	if head, err := s.dag.getHead(objid); err != nil || head != 3 {
		t.Errorf("Invalid object %d head in DAG %s, err %v", objid, head, err)
	}
}

// TestInitiatorBlessNoConf2 tests that a local and a remote log
// record stream can be correctly applied, when the conflict is
// resolved by a blessing. In this test, local head of the object is
// updated at the end of the first replay. In the second replay, a
// conflict resolved locally is rediscovered since it was also
// resolved remotely. Commands are in files
// testdata/<local-init-00.sync,remote-noconf-link-02.log.sync,
// remote-noconf-link-repeat.log.sync>.
func TestInitiatorBlessNoConf2(t *testing.T) {
	dir, err := createTempDir()
	if err != nil {
		t.Errorf("Could not create tempdir %v", err)
	}
	// Set a large value to prevent the threads from firing.
	// Test is not thread safe.
	peerSyncInterval = 1 * time.Hour
	garbageCollectInterval = 1 * time.Hour
	s := NewSyncd("", "", "VeyronTab", dir, "", 0)

	defer s.Close()
	defer os.RemoveAll(dir)

	if _, err = logReplayCommands(s.log, "local-init-00.sync"); err != nil {
		t.Error(err)
	}
	stream, err := createReplayStream("remote-noconf-link-02.log.sync")
	if err != nil {
		t.Fatalf("createReplayStream failed with err %v", err)
	}

	var minGens GenVector
	if minGens, err = s.hdlInitiator.processLogStream(stream); err != nil {
		t.Fatalf("processLogStream failed with err %v", err)
	}
	if err := s.hdlInitiator.detectConflicts(); err != nil {
		t.Fatalf("detectConflicts failed with err %v", err)
	}
	// Check that there are no conflicts.
	if len(s.hdlInitiator.updObjects) != 1 {
		t.Errorf("Too many objects %v", len(s.hdlInitiator.updObjects))
	}
	objid, err := strToObjID("12345")
	if err != nil {
		t.Errorf("Could not create objid %v", err)
	}
	st := s.hdlInitiator.updObjects[objid]
	if st.isConflict {
		t.Errorf("Detected a conflict %v", st)
	}
	if st.newHead != 4 || st.oldHead != 2 {
		t.Errorf("Conflict detection didn't succeed %v", st)
	}

	if err := s.hdlInitiator.resolveConflicts(); err != nil {
		t.Fatalf("resolveConflicts failed with err %v", err)
	}
	if err := s.hdlInitiator.updateStoreAndSync(nil, GenVector{"VeyronTab": 0}, minGens, GenVector{}, "VeyronPhone"); err != nil {
		t.Fatalf("updateStoreAndSync failed with err %v", err)
	}
	if st.resolvVal.Mutation.Version != 4 || st.resolvVal.Mutation.PriorVersion != 2 {
		t.Errorf("Mutation generation is not accurate %v", st)
	}
	// No new log records should be added.
	if s.log.head.Curgen != 1 || s.log.head.Curlsn != 3 || s.log.head.Curorder != 2 {
		t.Errorf("Data mismatch in log header %v", s.log.head)
	}
	// Verify DAG state.
	if head, err := s.dag.getHead(objid); err != nil || head != 4 {
		t.Errorf("Invalid object %d head in DAG %s, err %v", objid, head, err)
	}

	// Test simultaneous conflict resolution.
	stream, err = createReplayStream("remote-noconf-link-repeat.log.sync")
	if err != nil {
		t.Fatalf("createReplayStream failed with err %v", err)
	}

	if minGens, err = s.hdlInitiator.processLogStream(stream); err != nil {
		t.Fatalf("processLogStream failed with err %v", err)
	}
	if err := s.hdlInitiator.detectConflicts(); err != nil {
		t.Fatalf("detectConflicts failed with err %v", err)
	}
	// Check that there are no conflicts.
	if len(s.hdlInitiator.updObjects) != 1 {
		t.Errorf("Too many objects %v", len(s.hdlInitiator.updObjects))
	}
	st = s.hdlInitiator.updObjects[objid]
	if st.isConflict {
		t.Errorf("Detected a conflict %v", st)
	}
	if st.newHead != 4 || st.oldHead != 4 {
		t.Errorf("Conflict detection didn't succeed %v", st)
	}

	if err := s.hdlInitiator.resolveConflicts(); err != nil {
		t.Fatalf("resolveConflicts failed with err %v", err)
	}
	if err := s.hdlInitiator.updateStoreAndSync(nil, GenVector{}, minGens, GenVector{}, "VeyronLaptop"); err != nil {
		t.Fatalf("updateStoreAndSync failed with err %v", err)
	}
	if st.resolvVal.Mutation.Version != 4 {
		t.Errorf("Mutation generation is not accurate %v", st)
	}
	// No new log records should be added.
	if s.log.head.Curgen != 1 || s.log.head.Curlsn != 3 || s.log.head.Curorder != 3 {
		t.Errorf("Data mismatch in log header %v", s.log.head)
	}
	// Verify DAG state.
	if head, err := s.dag.getHead(objid); err != nil || head != 4 {
		t.Errorf("Invalid object %d head in DAG %s, err %v", objid, head, err)
	}
}

// TestInitiatorBlessConf tests that a local and a remote log record
// stream can be correctly applied, when the conflict is resolved by a
// blessing. Commands are in files
// testdata/<local-init-00.sync,remote-conf-link.log.sync>.
func TestInitiatorBlessConf(t *testing.T) {
	dir, err := createTempDir()
	if err != nil {
		t.Errorf("Could not create tempdir %v", err)
	}
	// Set a large value to prevent the threads from firing.
	// Test is not thread safe.
	peerSyncInterval = 1 * time.Hour
	garbageCollectInterval = 1 * time.Hour
	s := NewSyncd("", "", "VeyronTab", dir, "", 0)

	defer s.Close()
	defer os.RemoveAll(dir)

	if _, err = logReplayCommands(s.log, "local-init-00.sync"); err != nil {
		t.Error(err)
	}
	stream, err := createReplayStream("remote-conf-link.log.sync")
	if err != nil {
		t.Fatalf("createReplayStream failed with err %v", err)
	}

	var minGens GenVector
	if minGens, err = s.hdlInitiator.processLogStream(stream); err != nil {
		t.Fatalf("processLogStream failed with err %v", err)
	}
	if err := s.hdlInitiator.detectConflicts(); err != nil {
		t.Fatalf("detectConflicts failed with err %v", err)
	}
	// Check that there are no conflicts.
	if len(s.hdlInitiator.updObjects) != 1 {
		t.Errorf("Too many objects %v", len(s.hdlInitiator.updObjects))
	}
	objid, err := strToObjID("12345")
	if err != nil {
		t.Errorf("Could not create objid %v", err)
	}
	st := s.hdlInitiator.updObjects[objid]
	if !st.isConflict {
		t.Errorf("Didn't detect a conflict %v", st)
	}
	if st.newHead != 3 || st.oldHead != 2 || st.ancestor != 1 {
		t.Errorf("Conflict detection didn't succeed %v", st)
	}

	if err := s.hdlInitiator.resolveConflicts(); err != nil {
		t.Fatalf("resolveConflicts failed with err %v", err)
	}
	if st.resolvVal.Mutation.Version != 3 {
		t.Errorf("Mutation generation is not accurate %v", st)
	}

	if err := s.hdlInitiator.updateStoreAndSync(nil, GenVector{}, minGens, GenVector{}, "VeyronPhone"); err != nil {
		t.Fatalf("updateStoreAndSync failed with err %v", err)
	}
	if st.resolvVal.Mutation.Version != 3 || st.resolvVal.Mutation.PriorVersion != 2 {
		t.Errorf("Mutation generation is not accurate %v", st)
	}
	// New log records should be added.
	if s.log.head.Curgen != 1 || s.log.head.Curlsn != 4 || s.log.head.Curorder != 1 {
		t.Errorf("Data mismatch in log header %v", s.log.head)
	}
	curRec, err := s.log.getLogRec(s.id, GenID(1), LSN(3))
	if err != nil || curRec == nil {
		t.Fatalf("GetLogRec() can not find object %s:1:3 in log file err %v",
			s.id, err)
	}
	if curRec.ObjID != objid || curRec.RecType != LinkRec {
		t.Errorf("Data mismatch in log record %v", curRec)
	}
	// Verify DAG state.
	if head, err := s.dag.getHead(objid); err != nil || head != 3 {
		t.Errorf("Invalid object %d head in DAG %s, err %v", objid, head, err)
	}
}
