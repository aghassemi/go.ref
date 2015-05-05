// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vsync

// Veyron Sync DAG (directed acyclic graph) utility functions.
// The DAG is used to track the version history of objects in order to
// detect and resolve conflicts (concurrent changes on different devices).
//
// Terminology:
// * An object is a unique value in the Veyron Store represented by its UID.
// * As an object mutates, its version number is updated by the Store.
// * Each (object, version) tuple is represented by a node in the Sync DAG.
// * The previous version of an object is its parent in the DAG, i.e. the
//   new version is derived from that parent.
// * When there are no conflicts, the node has a single reference back to
//   a parent node.
// * When a conflict between two concurrent object versions is resolved,
//   the new version has references back to each of the two parents to
//   indicate that it is derived from both nodes.
// * During a sync operation from a source device to a target device, the
//   target receives a DAG fragment from the source.  That fragment has to
//   be incorporated (grafted) into the target device's DAG.  It may be a
//   continuation of the DAG of an object, with the attachment (graft) point
//   being the current head of DAG, in which case there are no conflicts.
//   Or the graft point(s) may be older nodes, which means the new fragment
//   is a divergence in the graph causing a conflict that must be resolved
//   in order to re-converge the two DAG fragments.
//
// In the diagrams below:
// (h) represents the head node in the local device.
// (nh) represents the new head node received from the remote device.
// (g) represents a graft node, where new nodes attach to the existing DAG.
// <- represents a derived-from mutation, i.e. a child-to-parent pointer
//
// a- No-conflict example: the new nodes (v3, v4) attach to the head node (v2).
//    In this case the new head becomes the head node, the new DAG fragment
//    being a continuation of the existing DAG.
//
//    Before:
//    v0 <- v1 <- v2(h)
//
//    Sync updates applied, no conflict detected:
//    v0 <- v1 <- v2(h,g) <- v3 <- v4 (nh)
//
//    After:
//    v0 <- v1 <- v2 <- v3 <- v4 (h)
//
// b- Conflict example: the new nodes (v3, v4) attach to an old node (v1).
//    The current head node (v2) and the new head node (v4) are divergent
//    (concurrent) mutations that need to be resolved.  The conflict
//    resolution function is passed the old head (v2), new head (v4), and
//    the common ancestor (v1) and resolves the conflict with (v5) which
//    is represented in the DAG as derived from both v2 and v4 (2 parents).
//
//    Before:
//    v0 <- v1 <- v2(h)
//
//    Sync updates applied, conflict detected (v2 not a graft node):
//    v0 <- v1(g) <- v2(h)
//                <- v3 <- v4 (nh)
//
//    After, conflict resolver creates v5 having 2 parents (v2, v4):
//    v0 <- v1(g) <- v2 <------- v5(h)
//                <- v3 <- v4 <-
//
// Note: the DAG does not grow indefinitely.  During a sync operation each
// device learns what the other device already knows -- where it's at in
// the version history for the objects.  When a device determines that all
// devices that sync an object (as per the definitions of replication groups
// in the Veyron Store) have moved past some version for that object, the
// DAG for that object can be pruned, deleting all prior (ancestor) nodes.
//
// The DAG DB contains four tables persisted to disk (nodes, heads, trans,
// priv) and three in-memory (ephemeral) maps (graft, txSet, txGC):
//   * nodes: one entry per (object, version) with references to the
//            parent node(s) it is derived from, a reference to the
//            log record identifying that change, a reference to its
//            transaction set (or NoTxId if none), and a boolean to
//            indicate whether this change was a deletion of the object.
//   * heads: one entry per object pointing to its most recent version
//            in the nodes table
//   * trans: one entry per transaction ID containing the set of objects
//            that forms the transaction and their versions.
//   * priv:  one entry per object ID for objects that are private to the
//            store, not shared through any SyncGroup.
//   * graft: during a sync operation, it tracks the nodes where the new
//            DAG fragments are attached to the existing graph for each
//            mutated object.  This map is used to determine whether a
//            conflict happened for an object and, if yes, select the most
//            recent common ancestor from these graft points to use when
//            resolving the conflict.  At the end of a sync operation the
//            graft map is destroyed.
//   * txSet: used to incrementally construct the transaction sets that
//            are stored in the "trans" table once all the nodes of a
//            transaction have been added.  Multiple transaction sets
//            can be constructed to support the concurrency between the
//            Sync Initiator and Watcher threads.
//   * txGC:  used to track the transactions impacted by objects being
//            pruned.  At the end of the pruning operation the records
//            of the "trans" table are updated from the txGC information.
//
// Note: for regular (no-conflict) changes, a node has a reference to
// one parent from which it was derived.  When a conflict is resolved,
// the new node has references to the two concurrent parents that triggered
// the conflict.  The states of the parents[] array are:
//   * []            The earliest/first version of an object
//   * [XYZ]         Regular non-conflict version derived from XYZ
//   * [XYZ, ABC]    Resolution version caused by XYZ-vs-ABC conflict

import (
	"container/list"
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"time"

	"v.io/x/lib/vlog"
	"v.io/x/ref/lib/stats"
)

const (
	NoTxId = TxId(0)
)

var (
	errBadDAG = errors.New("invalid DAG")
)

type dagTxMap map[ObjId]Version

// dagTxState tracks the state of a transaction.
type dagTxState struct {
	TxMap   dagTxMap
	TxCount uint32
}

type dag struct {
	fname string               // file pathname
	store *kvdb                // underlying K/V store
	heads *kvtable             // pointer to "heads" table in the store
	nodes *kvtable             // pointer to "nodes" table in the store
	trans *kvtable             // pointer to "trans" table in the store
	priv  *kvtable             // pointer to "priv" table in the store
	graft map[ObjId]*graftInfo // in-memory state of DAG object grafting
	txSet map[TxId]*dagTxState // in-memory construction of transaction sets
	txGC  map[TxId]dagTxMap    // in-memory tracking of transaction sets to cleanup
	txGen *rand.Rand           // transaction ID random number generator

	// DAG stats
	numObj  *stats.Integer // number of objects
	numNode *stats.Integer // number of versions across all objects
	numTx   *stats.Integer // number of transactions tracked
	numPriv *stats.Integer // number of private objects
}

type dagNode struct {
	Level   uint64    // node distance from root
	Parents []Version // references to parent versions
	Logrec  string    // reference to log record change
	TxId    TxId      // ID of a transaction set
	Deleted bool      // true if the change was a delete
}

type graftInfo struct {
	newNodes   map[Version]struct{} // set of newly added nodes during a sync
	graftNodes map[Version]uint64   // set of graft nodes and their level
	newHeads   map[Version]struct{} // set of candidate new head nodes
}

type privNode struct {
	//Mutation *raw.Mutation // most recent store mutation for a private (unshared) object
	PathIDs  []ObjId // store IDs in the path from the object to the root of the store
	SyncTime int64   // SyncTime is the timestamp of the mutation when it arrives at the Sync server.
	TxId     TxId    // ID of the transaction in which this mutation was done
	TxCount  uint32  // total number of object mutations in that transaction
}

// openDAG opens or creates a DAG for the given filename.
func openDAG(filename string) (*dag, error) {
	// Open the file and create it if it does not exist.
	// Also initialize the store and its tables.
	db, tbls, err := kvdbOpen(filename, []string{"heads", "nodes", "trans", "priv"})
	if err != nil {
		return nil, err
	}

	d := &dag{
		fname:   filename,
		store:   db,
		heads:   tbls[0],
		nodes:   tbls[1],
		trans:   tbls[2],
		priv:    tbls[3],
		txGen:   rand.New(rand.NewSource(time.Now().UTC().UnixNano())),
		txSet:   make(map[TxId]*dagTxState),
		numObj:  stats.NewInteger(statsNumDagObj),
		numNode: stats.NewInteger(statsNumDagNode),
		numTx:   stats.NewInteger(statsNumDagTx),
		numPriv: stats.NewInteger(statsNumDagPrivNode),
	}

	// Initialize the stats counters from the tables.
	d.numObj.Set(int64(d.heads.getNumKeys()))
	d.numNode.Set(int64(d.nodes.getNumKeys()))
	d.numTx.Set(int64(d.trans.getNumKeys()))
	d.numPriv.Set(int64(d.priv.getNumKeys()))

	d.clearGraft()
	d.clearTxGC()

	return d, nil
}

// close closes the DAG and invalidates its structure.
func (d *dag) close() {
	if d.store != nil {
		d.store.close() // this also closes the tables
		stats.Delete(statsNumDagObj)
		stats.Delete(statsNumDagNode)
		stats.Delete(statsNumDagTx)
		stats.Delete(statsNumDagPrivNode)
	}
	*d = dag{} // zero out the DAG struct
}

// flush flushes the DAG store to disk.
func (d *dag) flush() {
	if d.store != nil {
		d.store.flush()
	}
}

// clearGraft clears the temporary in-memory grafting maps.
func (d *dag) clearGraft() {
	if d.store != nil {
		d.graft = make(map[ObjId]*graftInfo)
	}
}

// clearTxGC clears the temporary in-memory transaction garbage collection maps.
func (d *dag) clearTxGC() {
	if d.store != nil {
		d.txGC = make(map[TxId]dagTxMap)
	}
}

// getObjectGraft returns the graft structure for an object ID.
// The graftInfo struct for an object is ephemeral (in-memory) and it
// tracks the following information:
// - newNodes:   the set of newly added nodes used to detect the type of
//               edges between nodes (new-node to old-node or vice versa).
// - newHeads:   the set of new candidate head nodes used to detect conflicts.
// - graftNodes: the set of nodes used to find common ancestors between
//               conflicting nodes.
//
// After the received Sync logs are applied, if there are two new heads in
// the newHeads set, there is a conflict to be resolved for this object.
// Otherwise if there is only one head, no conflict was triggered and the
// new head becomes the current version for the object.
//
// In case of conflict, the graftNodes set is used to select the common
// ancestor to pass to the conflict resolver.
//
// Note: if an object's graft structure does not exist only create it
// if the "create" parameter is set to true.
func (d *dag) getObjectGraft(oid ObjId, create bool) *graftInfo {
	graft := d.graft[oid]
	if graft == nil && create {
		graft = &graftInfo{
			newNodes:   make(map[Version]struct{}),
			graftNodes: make(map[Version]uint64),
			newHeads:   make(map[Version]struct{}),
		}

		// If a current head node exists for this object, initialize
		// the set of candidate new heads to include it.
		head, err := d.getHead(oid)
		if err == nil {
			graft.newHeads[head] = struct{}{}
		}

		d.graft[oid] = graft
	}
	return graft
}

// addNodeTxStart generates a transaction ID and returns it to the
// caller if a TxId is not specified.  This transaction ID is stored
// as part of each log record. If a TxId is specified by the caller,
// state corresponding to that TxId is instantiated. TxId is used to
// track DAG nodes that are part of the same transaction.
func (d *dag) addNodeTxStart(tid TxId) TxId {
	if d.store == nil {
		return NoTxId
	}

	// Check if "tid" already exists.
	if tid != NoTxId {
		if _, ok := d.txSet[tid]; ok {
			return tid
		}
		txSt, err := d.getTransaction(tid)
		if err == nil {
			d.txSet[tid] = txSt
			return tid
		}
	} else {
		// Generate a random 64-bit transaction ID different than NoTxId.
		// Also make sure the ID is not already being used.
		for (tid == NoTxId) || (d.txSet[tid] != nil) {
			// Generate an unsigned 64-bit random value by combining a
			// random 63-bit value and a random 1-bit value.
			tid = (TxId(d.txGen.Int63()) << 1) | TxId(d.txGen.Int63n(2))
		}
	}

	// Initialize the in-memory object/version map for that transaction ID.
	d.txSet[tid] = &dagTxState{TxMap: make(dagTxMap), TxCount: 0}
	return tid
}

// addNodeTxEnd marks the end of a given transaction.
// The DAG completes its internal tracking of the transaction information.
func (d *dag) addNodeTxEnd(tid TxId, count uint32) error {
	if d.store == nil {
		return errBadDAG
	}
	if tid == NoTxId || count == 0 {
		return fmt.Errorf("invalid TxState: %v %v", tid, count)
	}

	txSt, ok := d.txSet[tid]
	if !ok {
		return fmt.Errorf("unknown transaction ID: %v", tid)
	}

	// The first time a transaction (TxId) is ended, TxCount is
	// zero while "count" is not. Subsequently if this TxId is
	// started and ended, TxCount should be the same as the
	// incoming "count".
	if txSt.TxCount != 0 && txSt.TxCount != count {
		return fmt.Errorf("incorrect counts for transaction: %v (%v %v)", tid, txSt.TxCount, count)
	}

	// Only save non-empty transactions, i.e. those that have at least
	// one mutation on a shared (non-private) object.
	if len(txSt.TxMap) > 0 {
		txSt.TxCount = count
		if err := d.setTransaction(tid, txSt); err != nil {
			return err
		}
	}

	delete(d.txSet, tid)
	return nil
}

// addNode adds a new node for an object in the DAG, linking it to its parent nodes.
// It verifies that this node does not exist and that its parent nodes are valid.
// It also determines the DAG level of the node from its parent nodes (max() + 1).
//
// If the node is due to a local change (from the Watcher API), no need to
// update the grafting structure.  Otherwise the node is due to a remote change
// (from the Sync protocol) being grafted on the DAG:
// - If a parent node is not new, mark it as a DAG graft point.
// - Mark this version as a new node.
// - Update the new head node pointer of the grafted DAG.
//
// If the transaction ID is set to NoTxId, this node is not part of a transaction.
// Otherwise, track its membership in the given transaction ID.
func (d *dag) addNode(oid ObjId, version Version, remote, deleted bool,
	parents []Version, logrec string, tid TxId) error {
	if d.store == nil {
		return errBadDAG
	}

	if parents != nil {
		if len(parents) > 2 {
			return fmt.Errorf("cannot have more than 2 parents, not %d", len(parents))
		}
		if len(parents) == 0 {
			// Replace an empty array with a nil.
			parents = nil
		}
	}

	// The new node must not exist.
	if d.hasNode(oid, version) {
		return fmt.Errorf("node %v:%d already exists in the DAG", oid, version)
	}

	// A new root node (no parents) is allowed only for new objects.
	if parents == nil {
		_, err := d.getHead(oid)
		if err == nil {
			return fmt.Errorf("cannot add another root node %v:%d for this object in the DAG", oid, version)
		}
	}

	// For a remote change, make sure the object has a graft info entry.
	// During a sync operation, each mutated object gets new nodes added
	// in its DAG.  These new nodes are either derived from nodes that
	// were previously known on this device (i.e. their parent nodes are
	// pre-existing), or they are derived from other new DAG nodes being
	// discovered during this sync (i.e. their parent nodes were also
	// just added to the DAG).
	//
	// To detect a conflict and find the most recent common ancestor to
	// pass to the conflict resolver callback, the DAG keeps track of the
	// new nodes that have old parent nodes.  These old-to-new edges are
	// the points where new DAG fragments are attached (grafted) onto the
	// existing DAG.  The old nodes are the "graft nodes" and they form
	// the set of possible common ancestors to use in case of conflict:
	// 1- A conflict happens when the current "head node" for an object
	//    is not in the set of graft nodes.  It means the object mutations
	//    were not derived from what the device knows, but where divergent
	//    changes from a prior point (from one of the graft nodes).
	// 2- The most recent common ancestor to use in resolving the conflict
	//    is the object graft node with the deepest level (furthest from
	//    the origin root node), representing the most up-to-date common
	//    knowledge between this device and the divergent changes.
	//
	// Note: at the end of a sync operation between 2 devices, the whole
	// graft info is cleared (Syncd calls clearGraft()) to prepare it for
	// the new pairwise sync operation.
	graft := d.getObjectGraft(oid, remote)

	// Verify the parents and determine the node level.
	// Update the graft info in the DAG for this object.
	var level uint64
	for _, parent := range parents {
		node, err := d.getNode(oid, parent)
		if err != nil {
			return err
		}
		if level <= node.Level {
			level = node.Level + 1
		}
		if remote {
			// If this parent is an old node, it's a graft point in the DAG
			// and may be a common ancestor used during conflict resolution.
			if _, ok := graft.newNodes[parent]; !ok {
				graft.graftNodes[parent] = node.Level
			}

			// The parent nodes can no longer be candidates for new head versions.
			if _, ok := graft.newHeads[parent]; ok {
				delete(graft.newHeads, parent)
			}
		}
	}

	if remote {
		// This new node is a candidate for new head version.
		graft.newNodes[version] = struct{}{}
		graft.newHeads[version] = struct{}{}
	}

	// If this node is part of a transaction, add it to that set.
	if tid != NoTxId {
		txSt, ok := d.txSet[tid]
		if !ok {
			return fmt.Errorf("unknown transaction ID: %v", tid)
		}

		txSt.TxMap[oid] = version
	}

	// Insert the new node in the kvdb.
	node := &dagNode{Level: level, Parents: parents, Logrec: logrec, TxId: tid, Deleted: deleted}
	if err := d.setNode(oid, version, node); err != nil {
		return err
	}

	d.numNode.Incr(1)
	if parents == nil {
		d.numObj.Incr(1)
	}
	return nil
}

// hasNode returns true if the node (oid, version) exists in the DAG DB.
func (d *dag) hasNode(oid ObjId, version Version) bool {
	if d.store == nil {
		return false
	}
	key := objNodeKey(oid, version)
	return d.nodes.hasKey(key)
}

// addParent adds to the DAG node (oid, version) linkage to this parent node.
// If the parent linkage is due to a local change (from conflict resolution
// by blessing an existing version), no need to update the grafting structure.
// Otherwise a remote change (from the Sync protocol) updates the graft.
//
// TODO(rdaoud): recompute the levels of reachable child-nodes if the new
// parent's level is greater or equal to the node's current level.
func (d *dag) addParent(oid ObjId, version, parent Version, remote bool) error {
	if version == parent {
		return fmt.Errorf("addParent: object %v: node %d cannot be its own parent", oid, version)
	}

	node, err := d.getNode(oid, version)
	if err != nil {
		return err
	}

	pnode, err := d.getNode(oid, parent)
	if err != nil {
		vlog.VI(1).Infof("addParent: object %v, node %d, parent %d: parent node not found", oid, version, parent)
		return err
	}

	// Check if the parent is already linked to this node.
	found := false
	for i := range node.Parents {
		if node.Parents[i] == parent {
			found = true
			break
		}
	}

	// If the parent is not yet linked (local or remote) add it.
	if !found {
		// Make sure that adding the link does not create a cycle in the DAG.
		// This is done by verifying that the node is not an ancestor of the
		// parent that it is being linked to.
		err = d.ancestorIter(oid, pnode.Parents, func(oid ObjId, v Version, nd *dagNode) error {
			if v == version {
				return fmt.Errorf("addParent: cycle on object %v: node %d is an ancestor of parent node %d",
					oid, version, parent)
			}
			return nil
		})
		if err != nil {
			return err
		}
		node.Parents = append(node.Parents, parent)
		err = d.setNode(oid, version, node)
		if err != nil {
			return err
		}
	}

	// For local changes we are done, the grafting structure is not updated.
	if !remote {
		return nil
	}

	// If the node and its parent are new/old or old/new then add
	// the parent as a graft point (a potential common ancestor).
	graft := d.getObjectGraft(oid, true)

	_, nodeNew := graft.newNodes[version]
	_, parentNew := graft.newNodes[parent]
	if (nodeNew && !parentNew) || (!nodeNew && parentNew) {
		graft.graftNodes[parent] = pnode.Level
	}

	// The parent node can no longer be a candidate for a new head version.
	// The addParent() function only removes candidates from newHeads that
	// have become parents.  It does not add the child nodes to newHeads
	// because they are not necessarily new-head candidates.  If they are
	// new nodes, the addNode() function handles adding them to newHeads.
	// For old nodes, only the current head could be a candidate and it is
	// added to newHeads when the graft struct is initialized.
	if _, ok := graft.newHeads[parent]; ok {
		delete(graft.newHeads, parent)
	}

	return nil
}

// moveHead moves the object head node in the DAG.
func (d *dag) moveHead(oid ObjId, head Version) error {
	if d.store == nil {
		return errBadDAG
	}

	// Verify that the node exists.
	if !d.hasNode(oid, head) {
		return fmt.Errorf("node %v:%d does not exist in the DAG", oid, head)
	}

	return d.setHead(oid, head)
}

// hasConflict determines if there is a conflict for this object between its
// new and old head nodes.
// - Yes: return (true, newHead, oldHead, ancestor)
// - No:  return (false, newHead, oldHead, NoVersion)
// A conflict exists when there are two new-head nodes.  It means the newly
// added object versions are not derived in part from this device's current
// knowledge.  If there is a single new-head, the object changes were applied
// without triggering a conflict.
func (d *dag) hasConflict(oid ObjId) (isConflict bool, newHead, oldHead, ancestor Version, err error) {
	oldHead = NoVersion
	newHead = NoVersion
	ancestor = NoVersion
	if d.store == nil {
		err = errBadDAG
		return
	}

	graft := d.graft[oid]
	if graft == nil {
		err = fmt.Errorf("node %v has no DAG graft information", oid)
		return
	}

	numHeads := len(graft.newHeads)
	if numHeads < 1 || numHeads > 2 {
		err = fmt.Errorf("node %v has invalid number of new head candidates %d: %v", oid, numHeads, graft.newHeads)
		return
	}

	// Fetch the current head for this object if it exists.  The error from getHead()
	// is ignored because a newly received object is not yet known on this device and
	// will not trigger a conflict.
	oldHead, _ = d.getHead(oid)

	// If there is only one new head node there is no conflict.
	// The new head is that single one, even if it might also be the same old node.
	if numHeads == 1 {
		for k := range graft.newHeads {
			newHead = k
		}
		return
	}

	// With two candidate head nodes, the new one is the node that is
	// not the current (old) head node.
	for k := range graft.newHeads {
		if k != oldHead {
			newHead = k
			break
		}
	}

	// There is a conflict: the best choice ancestor is the graft point
	// node with the largest level (farthest from the root).  It is
	// possible in some corner cases to have multiple graft nodes at
	// the same level.  This would still be a single conflict, but the
	// multiple same-level graft points representing equivalent conflict
	// resolutions on different devices that are now merging their
	// resolutions.  In such a case it does not matter which node is
	// chosen as the ancestor because the conflict resolver function
	// is assumed to be convergent.  However it's nicer to make that
	// selection deterministic so all devices see the same choice.
	// For this the version number is used as a tie-breaker.
	isConflict = true
	var maxLevel uint64
	for node, level := range graft.graftNodes {
		if maxLevel < level ||
			(maxLevel == level && ancestor < node) {
			maxLevel = level
			ancestor = node
		}
	}
	return
}

// ancestorIter iterates over the DAG ancestor nodes for an object in a
// breadth-first traversal starting from given version node(s).  In its
// traversal it invokes the callback function once for each node, passing
// the object ID, version number and a pointer to the dagNode.
func (d *dag) ancestorIter(oid ObjId, startVersions []Version,
	cb func(ObjId, Version, *dagNode) error) error {
	visited := make(map[Version]bool)
	queue := list.New()
	for _, version := range startVersions {
		queue.PushBack(version)
		visited[version] = true
	}

	for queue.Len() > 0 {
		version := queue.Remove(queue.Front()).(Version)
		node, err := d.getNode(oid, version)
		if err != nil {
			// Ignore it, the parent was previously pruned.
			continue
		}
		for _, parent := range node.Parents {
			if !visited[parent] {
				queue.PushBack(parent)
				visited[parent] = true
			}
		}
		if err = cb(oid, version, node); err != nil {
			return err
		}
	}

	return nil
}

// hasDeletedDescendant returns true if the node (oid, version) exists in the
// DAG DB and one of its descendants is a deleted node (i.e. has its "Deleted"
// flag set true).  This means that at some object mutation after this version,
// the object was deleted.
func (d *dag) hasDeletedDescendant(oid ObjId, version Version) bool {
	if d.store == nil {
		return false
	}
	if !d.hasNode(oid, version) {
		return false
	}

	// Do a breadth-first traversal from the object's head node back to
	// the given version.  Along the way, track whether a deleted node is
	// traversed.  Return true only if a traversal reaches the given version
	// and had seen a deleted node along the way.

	// nodeStep tracks a step along a traversal.  It stores the node to visit
	// when taking that step and a boolean tracking whether a deleted node
	// was seen so far along that trajectory.
	head, err := d.getHead(oid)
	if err != nil {
		return false
	}

	type nodeStep struct {
		node    Version
		deleted bool
	}

	visited := make(map[nodeStep]struct{})
	queue := list.New()

	step := nodeStep{node: head, deleted: false}
	queue.PushBack(&step)
	visited[step] = struct{}{}

	for queue.Len() > 0 {
		step := queue.Remove(queue.Front()).(*nodeStep)
		if step.node == version {
			if step.deleted {
				return true
			}
			continue
		}
		node, err := d.getNode(oid, step.node)
		if err != nil {
			// Ignore it, the parent was previously pruned.
			continue
		}
		nextDel := step.deleted || node.Deleted

		for _, parent := range node.Parents {
			nextStep := nodeStep{node: parent, deleted: nextDel}
			if _, ok := visited[nextStep]; !ok {
				queue.PushBack(&nextStep)
				visited[nextStep] = struct{}{}
			}
		}
	}

	return false
}

// prune trims the DAG of an object at a given version (node) by deleting
// all its ancestor nodes, making it the new root node.  For each deleted
// node it calls the given callback function to delete its log record.
// This function should only be called when Sync determines that all devices
// that know about the object have gotten past this version.
// Also track any transaction sets affected by deleting DAG objects that
// have transaction IDs.  This is later used to do garbage collection
// on transaction sets when pruneDone() is called.
func (d *dag) prune(oid ObjId, version Version, delLogRec func(logrec string) error) error {
	if d.store == nil {
		return errBadDAG
	}

	// Get the node at the pruning point and set its parents to nil.
	// It will become the oldest DAG node (root) for the object.
	node, err := d.getNode(oid, version)
	if err != nil {
		return err
	}
	if node.Parents == nil {
		// Nothing to do, this node is already the root.
		return nil
	}

	iterVersions := node.Parents

	node.Parents = nil
	if err = d.setNode(oid, version, node); err != nil {
		return err
	}

	// Delete all ancestor nodes and their log records.
	// Delete as many as possible and track the error counts.
	// Keep track of objects deleted from transaction in order
	// to cleanup transaction sets when pruneDone() is called.
	numNodeErrs, numLogErrs := 0, 0
	err = d.ancestorIter(oid, iterVersions, func(oid ObjId, v Version, node *dagNode) error {
		nodeErrs, logErrs, err := d.removeNode(oid, v, node, delLogRec)
		numNodeErrs += nodeErrs
		numLogErrs += logErrs
		return err
	})
	if err != nil {
		return err
	}
	if numNodeErrs != 0 || numLogErrs != 0 {
		return fmt.Errorf("prune failed to delete %d nodes and %d log records", numNodeErrs, numLogErrs)
	}
	return nil
}

// removeNode removes the state associated with a DAG node.
func (d *dag) removeNode(oid ObjId, v Version, node *dagNode, delLogRec func(logrec string) error) (int, int, error) {
	numNodeErrs, numLogErrs := 0, 0
	if tid := node.TxId; tid != NoTxId {
		if d.txGC[tid] == nil {
			d.txGC[tid] = make(dagTxMap)
		}
		d.txGC[tid][oid] = v
	}

	if err := delLogRec(node.Logrec); err != nil {
		numLogErrs++
	}
	if err := d.delNode(oid, v); err != nil {
		numNodeErrs++
	}
	d.numNode.Incr(-1)
	return numNodeErrs, numLogErrs, nil
}

// pruneAll prunes the entire DAG state corresponding to an object,
// including the head.
func (d *dag) pruneAll(oid ObjId, delLogRec func(logrec string) error) error {
	vers, err := d.getHead(oid)
	if err != nil {
		return err
	}
	node, err := d.getNode(oid, vers)
	if err != nil {
		return err
	}

	if err := d.prune(oid, vers, delLogRec); err != nil {
		return err
	}

	// Clean up the head.
	numNodeErrs, numLogErrs, err := d.removeNode(oid, vers, node, delLogRec)
	if err != nil {
		return err
	}
	if numNodeErrs != 0 || numLogErrs != 0 {
		return fmt.Errorf("pruneAll failed to delete %d nodes and %d log records", numNodeErrs, numLogErrs)
	}

	return d.delHead(oid)
}

// pruneDone is called when object pruning is finished within a single pass
// of the Sync garbage collector.  It updates the transaction sets affected
// by the objects deleted by the prune() calls.
func (d *dag) pruneDone() error {
	if d.store == nil {
		return errBadDAG
	}

	// Update transaction sets by removing from them the objects that
	// were pruned.  If the resulting set is empty, delete it.
	for tid, txMapGC := range d.txGC {
		txSt, err := d.getTransaction(tid)
		if err != nil {
			return err
		}

		for oid := range txMapGC {
			delete(txSt.TxMap, oid)
		}

		if len(txSt.TxMap) > 0 {
			err = d.setTransaction(tid, txSt)
		} else {
			err = d.delTransaction(tid)
		}
		if err != nil {
			return err
		}
	}

	d.clearTxGC()
	return nil
}

// getLogrec returns the log record information for a given object version.
func (d *dag) getLogrec(oid ObjId, version Version) (string, error) {
	node, err := d.getNode(oid, version)
	if err != nil {
		return "", err
	}
	return node.Logrec, nil
}

// objNodeKey returns the key used to access the object node (oid, version)
// in the DAG DB.
func objNodeKey(oid ObjId, version Version) string {
	return fmt.Sprintf("%v:%d", oid, version)
}

// setNode stores the dagNode structure for the object node (oid, version)
// in the DAG DB.
func (d *dag) setNode(oid ObjId, version Version, node *dagNode) error {
	if d.store == nil {
		return errBadDAG
	}
	key := objNodeKey(oid, version)
	return d.nodes.set(key, node)
}

// getNode retrieves the dagNode structure for the object node (oid, version)
// from the DAG DB.
func (d *dag) getNode(oid ObjId, version Version) (*dagNode, error) {
	if d.store == nil {
		return nil, errBadDAG
	}
	var node dagNode
	key := objNodeKey(oid, version)
	if err := d.nodes.get(key, &node); err != nil {
		return nil, err
	}
	return &node, nil
}

// delNode deletes the object node (oid, version) from the DAG DB.
func (d *dag) delNode(oid ObjId, version Version) error {
	if d.store == nil {
		return errBadDAG
	}
	key := objNodeKey(oid, version)
	return d.nodes.del(key)
}

// objHeadKey returns the key used to access the object head in the DAG DB.
func objHeadKey(oid ObjId) string {
	return oid.String()
}

// setHead stores version as the object head in the DAG DB.
func (d *dag) setHead(oid ObjId, version Version) error {
	if d.store == nil {
		return errBadDAG
	}
	key := objHeadKey(oid)
	return d.heads.set(key, version)
}

// getHead retrieves the object head from the DAG DB.
func (d *dag) getHead(oid ObjId) (Version, error) {
	var version Version
	if d.store == nil {
		return version, errBadDAG
	}
	key := objHeadKey(oid)
	err := d.heads.get(key, &version)
	if err != nil {
		version = NoVersion
	}
	return version, err
}

// delHead deletes the object head from the DAG DB.
func (d *dag) delHead(oid ObjId) error {
	if d.store == nil {
		return errBadDAG
	}
	key := objHeadKey(oid)
	if err := d.heads.del(key); err != nil {
		return err
	}
	d.numObj.Incr(-1)
	return nil
}

// dagTransactionKey returns the key used to access the transaction in the DAG DB.
func dagTransactionKey(tid TxId) string {
	return fmt.Sprintf("%v", tid)
}

// setTransaction stores the transaction object/version map in the DAG DB.
func (d *dag) setTransaction(tid TxId, txSt *dagTxState) error {
	if d.store == nil {
		return errBadDAG
	}
	if tid == NoTxId {
		return fmt.Errorf("invalid TxId: %v", tid)
	}
	key := dagTransactionKey(tid)
	exists := d.trans.hasKey(key)

	if err := d.trans.set(key, txSt); err != nil {
		return err
	}

	if !exists {
		d.numTx.Incr(1)
	}
	return nil
}

// getTransaction retrieves the transaction object/version map from the DAG DB.
func (d *dag) getTransaction(tid TxId) (*dagTxState, error) {
	if d.store == nil {
		return nil, errBadDAG
	}
	if tid == NoTxId {
		return nil, fmt.Errorf("invalid TxId: %v", tid)
	}
	var txSt dagTxState
	key := dagTransactionKey(tid)
	if err := d.trans.get(key, &txSt); err != nil {
		return nil, err
	}
	return &txSt, nil
}

// delTransaction deletes the transation object/version map from the DAG DB.
func (d *dag) delTransaction(tid TxId) error {
	if d.store == nil {
		return errBadDAG
	}
	if tid == NoTxId {
		return fmt.Errorf("invalid TxId: %v", tid)
	}
	key := dagTransactionKey(tid)
	if err := d.trans.del(key); err != nil {
		return err
	}
	d.numTx.Incr(-1)
	return nil
}

// objPrivNodeKey returns the key used to access a private (unshared) node in the DAG DB.
func objPrivNodeKey(oid ObjId) string {
	return oid.String()
}

// setPrivNode stores the privNode structure for a private (unshared) object in the DAG DB.
func (d *dag) setPrivNode(oid ObjId, priv *privNode) error {
	if d.store == nil {
		return errBadDAG
	}
	key := objPrivNodeKey(oid)
	exists := d.priv.hasKey(key)

	if err := d.priv.set(key, priv); err != nil {
		return err
	}

	if !exists {
		d.numPriv.Incr(1)
	}
	return nil
}

// getPrivNode retrieves the privNode structure for a private (unshared) object from the DAG DB.
func (d *dag) getPrivNode(oid ObjId) (*privNode, error) {
	if d.store == nil {
		return nil, errBadDAG
	}
	var priv privNode
	key := objPrivNodeKey(oid)
	if err := d.priv.get(key, &priv); err != nil {
		return nil, err
	}
	return &priv, nil
}

// delPrivNode deletes a private (unshared) object from the DAG DB.
func (d *dag) delPrivNode(oid ObjId) error {
	if d.store == nil {
		return errBadDAG
	}
	key := objPrivNodeKey(oid)
	if err := d.priv.del(key); err != nil {
		return err
	}
	d.numPriv.Incr(-1)
	return nil
}

// getParentMap is a testing and debug helper function that returns for
// an object a map of all the object version in the DAG and their parents.
// The map represents the graph of the object version history.
func (d *dag) getParentMap(oid ObjId) map[Version][]Version {
	parentMap := make(map[Version][]Version)
	var iterVersions []Version

	if head, err := d.getHead(oid); err == nil {
		iterVersions = append(iterVersions, head)
	}
	if graft := d.graft[oid]; graft != nil {
		for k := range graft.newHeads {
			iterVersions = append(iterVersions, k)
		}
	}

	// Breadth-first traversal starting from the object head.
	d.ancestorIter(oid, iterVersions, func(oid ObjId, v Version, node *dagNode) error {
		parentMap[v] = node.Parents
		return nil
	})

	return parentMap
}

// getGraftNodes is a testing and debug helper function that returns for
// an object the graft information built and used during a sync operation.
// The newHeads map identifies the candidate head nodes based on the data
// reported by the other device during a sync operation.  The graftNodes map
// identifies the set of old nodes where the new DAG fragments were attached
// and their depth level in the DAG.
func (d *dag) getGraftNodes(oid ObjId) (map[Version]struct{}, map[Version]uint64) {
	if d.store != nil {
		if ginfo := d.graft[oid]; ginfo != nil {
			return ginfo.newHeads, ginfo.graftNodes
		}
	}
	return nil, nil
}

// strToTxId converts from a string to a transaction ID.
func strToTxId(txStr string) (TxId, error) {
	tx, err := strconv.ParseUint(txStr, 10, 64)
	if err != nil {
		return NoTxId, err
	}
	return TxId(tx), nil
}

// dump writes to the log file information on all DAG entries.
func (d *dag) dump() {
	if d.store == nil {
		return
	}

	// Dump the head and ancestor information for DAG objects.
	d.heads.keyIter(func(oidStr string) {
		oid, err := strToObjId(oidStr)
		if err != nil {
			return
		}

		head, err := d.getHead(oid)
		if err != nil {
			return
		}

		vlog.VI(1).Infof("DUMP: DAG oid %v: head %v", oid, head)
		start := []Version{head}
		d.ancestorIter(oid, start, func(oid ObjId, v Version, node *dagNode) error {
			vlog.VI(1).Infof("DUMP: DAG node %v:%v: tx %v, del %t, logrec %s --> %v",
				oid, v, node.TxId, node.Deleted, node.Logrec, node.Parents)
			return nil
		})
	})

	// Dump the transactions.
	d.trans.keyIter(func(tidStr string) {
		tid, err := strToTxId(tidStr)
		if err != nil {
			return
		}

		txSt, err := d.getTransaction(tid)
		if err != nil {
			return
		}

		vlog.VI(1).Infof("DUMP: DAG tx %v: count %d, elem %v", tid, txSt.TxCount, txSt.TxMap)
	})
}
