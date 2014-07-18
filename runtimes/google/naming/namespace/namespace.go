package namespace

import (
	"sync"
	"time"

	"veyron2"
	"veyron2/naming"
	"veyron2/verror"
)

const defaultMaxResolveDepth = 32
const defaultMaxRecursiveGlobDepth = 10

// namespace is an implementation of naming.MountTable.
type namespace struct {
	sync.RWMutex
	rt veyron2.Runtime

	// the default root servers for resolutions in this namespace.
	roots []string

	// depth limits
	maxResolveDepth       int
	maxRecursiveGlobDepth int
}

func rooted(names []string) bool {
	for _, n := range names {
		if a, _ := naming.SplitAddressName(n); len(a) == 0 {
			return false
		}
	}
	return true
}

func badRoots(roots []string) verror.E {
	return verror.BadArgf("At least one root is not a rooted name: %q", roots)
}

// Create a new namespace.
func New(rt veyron2.Runtime, roots ...string) (*namespace, error) {
	if !rooted(roots) {
		return nil, badRoots(roots)
	}
	// A namespace with no roots can still be used for lookups of rooted names.
	return &namespace{
		rt:                    rt,
		roots:                 roots,
		maxResolveDepth:       defaultMaxResolveDepth,
		maxRecursiveGlobDepth: defaultMaxRecursiveGlobDepth,
	}, nil
}

// SetRoots implements naming.MountTable.SetRoots
func (ns *namespace) SetRoots(roots ...string) error {
	if !rooted(roots) {
		return badRoots(roots)
	}
	ns.Lock()
	defer ns.Unlock()
	// TODO(cnicolaou): filter out duplicate values.
	ns.roots = roots
	return nil
}

// SetDepthLimits overrides the default limits.
func (ns *namespace) SetDepthLimits(resolve, glob int) {
	if resolve >= 0 {
		ns.maxResolveDepth = resolve
	}
	if glob >= 0 {
		ns.maxRecursiveGlobDepth = glob
	}
}

// Roots implements naming.MountTable.Roots
func (ns *namespace) Roots() []string {
	ns.RLock()
	defer ns.RUnlock()
	roots := make([]string, len(ns.roots))
	for i, r := range ns.roots {
		roots[i] = r
	}
	return roots
}

// rootName 'roots' a name: if name is not a rooted name, it prepends the root
// mounttable's OA.
func (ns *namespace) rootName(name string) []string {
	if address, _ := naming.SplitAddressName(name); len(address) == 0 {
		var ret []string
		ns.RLock()
		defer ns.RUnlock()
		for _, r := range ns.roots {
			ret = append(ret, naming.Join(r, name))
		}
		return ret
	}
	return []string{name}
}

// notAnMT returns true if the error indicates this isn't a mounttable server.
func notAnMT(err error) bool {
	switch verror.ErrorID(err) {
	case verror.BadArg:
		// This should cover "ipc: wrong number of in-args".
		return true
	case verror.NotFound:
		// This should cover "ipc: unknown method", "ipc: dispatcher not
		// found", and "ipc: SoloDispatcher lookup on non-empty suffix".
		return true
	case verror.BadProtocol:
		// This covers "ipc: response decoding failed: EOF".
		return true
	}
	return false
}

// all operations against the mount table service use this fixed timeout for the
// time being.
const callTimeout = veyron2.CallTimeout(10 * time.Second)
