package state

import (
	"fmt"

	"veyron/services/store/memstore/refs"

	"veyron2/security"
	"veyron2/storage"
)

// Iterator is used to iterate through the descendents of a value.  The order of
// iteration is breadth-first.  Each descendent is visited at most once.
type Iterator interface {
	// IsValid returns true iff the iterator refers to an element.
	IsValid() bool

	// Get returns the current value.
	Get() *storage.Entry

	// Return one possible name for this entry.
	Name() string

	// Next advances to the next element.
	Next()

	// Snapshot returns the iterator's snapshot.
	Snapshot() Snapshot
}

// iterator implements the Iterator interface.
//
// TODO(jyh): This is a simple, naive implementation.  We need to implement
// security, perform type-directed iteration, and we may need pruning (similar
// to the -prune option to the "find" command).
type iterator struct {
	snapshot Snapshot

	// Set of IDs already visited on this path.
	visited map[storage.ID]struct{}

	// Stack of actions to consider next. Actions are one of:
	// - visit a node accessible from the current path (the node may already
	//   have been visited on the current path).
	// - unvisit a node (backtrack the current path).
	next []next

	// Depth of starting path.
	initialDepth int

	// Current value.
	entry *storage.Entry
	path  *refs.FullPath

	pathFilter PathFilter

	filter IterFilter
}

type next struct {
	parent *refs.FullPath
	path   *refs.Path
	id     storage.ID
	action action
}

type action int

const (
	visit = action(iota)
	unvisit
)

var (
	_ Iterator = (*iterator)(nil)
	_ Iterator = (*errorIterator)(nil)
)

// A PathFilter automatically limits the traversal of certain paths,
type PathFilter int

const (
	// ListPaths permits any path that does not visit the same object twice.
	ListPaths = PathFilter(iota)
	// ListObjects permits any path that does not revisit any object on a
	// previously traversed path 'Q', even if Q did not satisfy it.filter.
	ListObjects
)

// An IterFilter examines entries as they are considered by the
// iterator and allows it to give two boolean inputs to the process:
// ret: True if the iterator should return this value in its iteration.
// expand: True if the iterator should consider children of this value.
type IterFilter func(*refs.FullPath, *refs.Path) (ret bool, expand bool)

// Recursive filter is an IterFilter that causes all decendents to be
// returned during iteration.  This is the default behavior.
func RecursiveFilter(*refs.FullPath, *refs.Path) (bool, bool) {
	return true, true
}

// ImmediateFilter is a filter that causes only the specified path and
// immediate decendents to be returned from the iterator.
func ImmediateFilter(_ *refs.FullPath, path *refs.Path) (bool, bool) {
	return true, path == nil
}

// NewIterator returns an Iterator that starts with the value at <path>.
// pathFilter is used to automatically limit traversal of certain paths.
// If filter is given, it is used to limit traversal beneath certain paths and
// limit the results of the iteration. If filter is nil, all decendents of the
// specified path are returned.
func (sn *snapshot) NewIterator(pid security.PublicID, path storage.PathName,
	pathFilter PathFilter, filter IterFilter) Iterator {

	cell, suffix, v := sn.resolveCell(path, nil)
	if cell == nil {
		return &errorIterator{snapshot: sn}
	}

	if filter == nil {
		filter = RecursiveFilter
	}

	it := &iterator{
		snapshot:     sn,
		visited:      make(map[storage.ID]struct{}),
		initialDepth: len(path),
		path:         refs.NewFullPathFromName(path),
		pathFilter:   pathFilter,
		filter:       filter,
	}

	ret, expand := it.filter(it.path, nil)

	var set refs.Set
	if len(suffix) != 0 {
		// We're started from a field within cell.  Calculate the refs reachable
		// from the value.  Allow self references to cell.id.
		it.entry = newSubfieldEntry(v)
		r := refs.NewBuilder()
		r.AddValue(v)
		set = r.Get()
	} else {
		it.entry = cell.GetEntry()
		it.visited[cell.ID] = struct{}{}
		it.pushUnvisit(nil, cell.ID)
		set = cell.refs
	}

	if expand {
		it.pushVisitAll(it.path, set)
	}
	if !ret {
		it.Next()
	}

	return it
}

func (it *iterator) pushUnvisit(path *refs.Path, id storage.ID) {
	switch it.pathFilter {
	case ListPaths:
		it.next = append(it.next, next{nil, path, id, unvisit})
	case ListObjects:
		// Do not unvisit the object, as it is on a path already seen by
		// it.filter.
	default:
		panic("unknown PathFilter")
	}
}

func (it *iterator) pushVisitAll(parentPath *refs.FullPath, set refs.Set) {

	set.Iter(func(x interface{}) bool {
		ref := x.(*refs.Ref)
		it.next = append(it.next, next{parentPath, ref.Path, ref.ID, visit})
		return true
	})
}

// IsValid returns true iff the iterator refers to an element.
func (it *iterator) IsValid() bool {
	return it.entry != nil
}

// Name returns a name for the curent value relative to the initial name.
func (it *iterator) Name() string {
	return it.path.Suffix(it.path.Len() - it.initialDepth)
}

// Get returns the current value.
func (it *iterator) Get() *storage.Entry {
	return it.entry
}

// Next advances to the next element.
func (it *iterator) Next() {
	var n next
	var fullPath *refs.FullPath
	var c *Cell
	for {
		topIndex := len(it.next) - 1
		if topIndex < 0 {
			it.entry, it.path = nil, nil
			return
		}
		n, it.next = it.next[topIndex], it.next[:topIndex]

		if n.action == unvisit {
			delete(it.visited, n.id)
			continue
		}

		if _, ok := it.visited[n.id]; ok {
			continue
		}

		// Mark as visited.
		it.visited[n.id] = struct{}{}
		it.pushUnvisit(n.path, n.id)

		// Fetch the cell.
		c = it.snapshot.Find(n.id)
		if c == nil {
			// The table is inconsistent.
			panic(fmt.Sprintf("Dangling reference: %s", n.id))
		}

		// Check the filter
		ret, expand := it.filter(n.parent, n.path)
		fullPath = n.parent.AppendPath(n.path)
		if expand {
			it.pushVisitAll(fullPath, c.refs)
		}
		if ret {
			// Found a value.
			break
		}
	}

	it.entry, it.path = c.GetEntry(), fullPath
}

func (it *iterator) Snapshot() Snapshot {
	return it.snapshot
}

// errorIterator is the iterator that does nothing, has no values.
type errorIterator struct {
	snapshot Snapshot
}

func (it *errorIterator) IsValid() bool {
	return false
}

func (it *errorIterator) Get() *storage.Entry {
	return nil
}

func (it *errorIterator) Name() string {
	return ""
}

func (it *errorIterator) Next() {}

func (it *errorIterator) Snapshot() Snapshot {
	return it.snapshot
}
