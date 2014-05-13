package vsync

// Package vsync provides veyron sync daemon utility functions. Sync
// daemon serves incoming GetDeltas requests and contacts other peers
// to get deltas from them. When it receives a GetDeltas request, the
// incoming generation vector is diffed with the local generation
// vector, and missing generations are sent back. When it receives
// log records in response to a GetDeltas request, it replays those
// log records to get in sync with the sender.
import (
	"sync"

	"veyron/services/store/estore"

	"veyron2/ipc"
	"veyron2/storage"
	"veyron2/storage/vstore"
	"veyron2/vlog"
	"veyron2/vom"
)

// syncd contains the metadata for the sync daemon.
type syncd struct {
	// Pointers to metadata structures.
	log    *iLog
	devtab *devTable
	dag    *dag

	// Local device id.
	id DeviceID

	// RWlock to concurrently access log and device table data structures.
	lock sync.RWMutex
	// State to coordinate shutting down all spawned goroutines.
	pending sync.WaitGroup
	closed  chan struct{}

	// Local Veyron store.
	vstoreEndpoint string
	vstore         storage.Store

	// Handlers for goroutine procedures.
	hdlGC        *syncGC
	hdlWatcher   *syncWatcher
	hdlInitiator *syncInitiator
}

// NewSyncd creates a new syncd instance.
//
// Syncd concurrency: syncd initializes three goroutines at
// startup. The "watcher" thread is responsible for watching the store
// for changes to its objects. The "initiator" thread is responsible
// for periodically checking the neighborhood and contacting a peer to
// obtain changes from that peer. The "gc" thread is responsible for
// periodically checking if any log records and dag state can be
// pruned. All these 3 threads perform write operations to the data
// structures, and synchronize by acquiring a write lock on s.lock. In
// addition, when syncd receives an incoming RPC, it responds to the
// request by acquiring a read lock on s.lock. Thus, at any instant in
// time, either one of the watcher, initiator or gc threads is active,
// or any number of responders can be active, serving incoming
// requests. Fairness between these threads follows from
// sync.RWMutex. The spec says that the writers cannot be starved by
// the readers but it does not guarantee FIFO. We may have to revisit
// this in the future.
func NewSyncd(peerEndpoints, peerDeviceIDs, devid, storePath, vstoreEndpoint string) *syncd {
	// Connect to the local Veyron store.
	// At present this is optional to allow testing (from the command-line) w/o Veyron store running.
	// TODO: connecting to Veyron store should be mandatory.
	var st storage.Store
	if vstoreEndpoint != "" {
		vs, err := vstore.New(vstoreEndpoint)
		if err != nil {
			vlog.Fatalf("newSyncd: cannot connect to Veyron store endpoint (%s): %s", vstoreEndpoint, err)
		}
		st = vs
	}

	return newSyncdCore(peerEndpoints, peerDeviceIDs, devid, storePath, vstoreEndpoint, st)
}

// newSyncdCore is the internal function that creates the Syncd
// structure and initilizes its thread (goroutines).  It takes a
// Veyron Store parameter to separate the core of Syncd setup from the
// external dependency on Veyron Store.
func newSyncdCore(peerEndpoints, peerDeviceIDs, devid, storePath, vstoreEndpoint string, store storage.Store) *syncd {
	s := &syncd{}

	// Bootstrap my own DeviceID.
	s.id = DeviceID(devid)

	var err error
	// Log init.
	if s.log, err = openILog(storePath+"/ilog", s); err != nil {
		vlog.Fatalf("newSyncd: ILogInit failed: err %v", err)
	}

	// DevTable init.
	if s.devtab, err = openDevTable(storePath+"/dtab", s); err != nil {
		vlog.Fatalf("newSyncd: DevTableInit failed: err %v", err)
	}

	// Dag Init.
	if s.dag, err = openDAG(storePath + "/dag"); err != nil {
		vlog.Fatalf("newSyncd: OpenDag failed: err %v", err)
	}

	// Veyron Store.
	s.vstoreEndpoint = vstoreEndpoint
	s.vstore = store
	vlog.VI(1).Infof("newSyncd: Local Veyron store: %s\n", s.vstoreEndpoint)

	// Register these Watch data types with VOM.
	// TODO(tilaks): why aren't they auto-retrieved from the IDL?
	vom.Register(&estore.Mutation{})
	vom.Register(&storage.DEntry{})

	// Channel to propagate close event to all threads.
	s.closed = make(chan struct{})

	s.pending.Add(3)

	// Get deltas every peerSyncInterval.
	s.hdlInitiator = newInitiator(s, peerEndpoints, peerDeviceIDs)
	go s.hdlInitiator.contactPeers()

	// Garbage collect every garbageCollectInterval.
	s.hdlGC = newGC(s)
	go s.hdlGC.garbageCollect()

	// Start a watcher thread that will get updates from local store.
	s.hdlWatcher = newWatcher(s)
	go s.hdlWatcher.watchStore()

	return s
}

// Close cleans up syncd state.
func (s *syncd) Close() {
	close(s.closed)
	s.pending.Wait()

	// TODO(hpucha): close without flushing.
}

// isSyncClosing returns true if Close() was called i.e. the "closed" channel is closed.
func (s *syncd) isSyncClosing() bool {
	select {
	case <-s.closed:
		return true
	default:
		return false
	}
}

// GetDeltas responds to the incoming request from a client by sending missing generations to the client.
func (s *syncd) GetDeltas(_ ipc.Context, In GenVector, ClientID DeviceID, Stream SyncServiceGetDeltasStream) (GenVector, error) {
	vlog.VI(1).Infof("GetDeltas:: Received vector %v from client %s", In, ClientID)

	s.lock.Lock()

	// Note that the incoming client generation vector cannot be
	// used for garbage collection. We can only garbage collect
	// based on the generations we receive from other
	// devices. Receiving a set of generations assures that all
	// updates branching from those generations are also received
	// and hence generations present on all devices can be
	// GC'ed. This function sends generations to other devices and
	// hence does not use the generation vector for GC.
	//
	// TODO(hpucha): Cache the client's incoming generation vector
	// to assist in tracking missing generations and hence next
	// peer to contact.
	if !s.devtab.hasDevInfo(ClientID) {
		if err := s.devtab.addDevice(ClientID); err != nil {
			s.lock.Unlock()
			vlog.Fatalf("GetDeltas:: addDevice failed with err %v", err)
		}
	}

	// TODO(hpucha): Hack, fills fake log and dag state for testing.
	s.log.fillFakeWatchRecords()

	// Create a new local generation if there are any local updates.
	gen, err := s.log.createLocalGeneration()
	if err == nil {
		// Update local generation vector in devTable.
		if err = s.devtab.updateGeneration(s.id, s.id, gen); err != nil {
			s.lock.Unlock()
			vlog.Fatalf("GetDeltas:: UpdateGeneration failed with err %v", err)
		}
	} else if err == errNoUpdates {
		vlog.VI(1).Infof("GetDeltas:: No new updates. Local at %d", gen)
	} else {
		s.lock.Unlock()
		vlog.Fatalf("GetDeltas:: CreateLocalGeneration failed with err %v", err)
	}

	// Get local generation vector.
	out, err := s.devtab.getGenVec(s.id)
	if err != nil {
		s.lock.Unlock()
		vlog.Fatalf("GetDeltas:: GetGenVec failed with err %v", err)
	}
	s.lock.Unlock()

	s.lock.RLock()
	defer s.lock.RUnlock()

	// Diff the two generation vectors.
	gens, err := s.devtab.diffGenVectors(out, In)
	if err != nil {
		vlog.Fatalf("GetDeltas:: Diffing gen vectors failed: err %v", err)
	}

	for _, v := range gens {
		// Sending one generation at a time.
		gen, err := s.log.getGenMetadata(v.devID, v.genID)
		if err != nil || gen.Count <= 0 {
			vlog.Fatalf("GetDeltas:: getGenMetadata failed for generation %s %d %v, err %v",
				v.devID, v.genID, gen, err)
		}

		var count uint64
		for i := LSN(0); i <= gen.MaxLSN; i++ {
			count++
			rec, err := s.log.getLogRec(v.devID, v.genID, i)
			if err != nil {
				vlog.Fatalf("GetDeltas:: Couldn't get log record %s %d %d, err %v",
					v.devID, v.genID, i, err)
			}
			vlog.VI(1).Infof("Sending log record %v", rec)
			s.lock.RUnlock()
			if err := Stream.Send(*rec); err != nil {
				vlog.Errorf("GetDeltas:: Couldn't send stream err: %v", err)
				s.lock.RLock()
				return GenVector{}, err
			}
			s.lock.RLock()
		}
		if count != gen.Count {
			vlog.Fatalf("GetDeltas:: GenMetadata has incorrect log records for generation %s %d %v",
				v.devID, v.genID, gen)
		}
	}

	return out, nil
}
