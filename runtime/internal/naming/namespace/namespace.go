// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package namespace

import (
	"sync"
	"time"

	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/rpc"
	"v.io/v23/security"
	vdltime "v.io/v23/vdlroot/time"
	"v.io/v23/verror"

	"v.io/x/ref/lib/apilog"
	inaming "v.io/x/ref/runtime/internal/naming"
)

const defaultMaxResolveDepth = 32
const defaultMaxRecursiveGlobDepth = 10

const pkgPath = "v.io/x/ref/runtime/internal/naming/namespace"

var (
	errNotRootedName = verror.Register(pkgPath+".errNotRootedName", verror.NoRetry, "{1:}{2:} At least one root is not a rooted name{:_}")
)

// namespace is an implementation of naming.Namespace.
type namespace struct {
	sync.RWMutex

	// the default root servers for resolutions in this namespace.
	roots []string

	// depth limits
	maxResolveDepth       int
	maxRecursiveGlobDepth int

	// cache for name resolutions
	resolutionCache cache
}

func rooted(names []string) bool {
	for _, n := range names {
		if a, _ := naming.SplitAddressName(n); len(a) == 0 {
			return false
		}
	}
	return true
}

func badRoots(roots []string) error {
	return verror.New(errNotRootedName, nil, roots)
}

// Create a new namespace.
func New(roots ...string) (*namespace, error) {
	if !rooted(roots) {
		return nil, badRoots(roots)
	}
	// A namespace with no roots can still be used for lookups of rooted names.
	return &namespace{
		roots:                 roots,
		maxResolveDepth:       defaultMaxResolveDepth,
		maxRecursiveGlobDepth: defaultMaxRecursiveGlobDepth,
		resolutionCache:       newTTLCache(),
	}, nil
}

// SetRoots implements naming.Namespace.SetRoots
func (ns *namespace) SetRoots(roots ...string) error {
	defer apilog.LogCallf(nil, "roots...=%v", roots)(nil, "") // gologcop: DO NOT EDIT, MUST BE FIRST STATEMENT
	// Allow roots to be cleared with a call of SetRoots()
	if len(roots) > 0 && !rooted(roots) {
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

// Roots implements naming.Namespace.Roots
func (ns *namespace) Roots() []string {
	//nologcall
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
	name = naming.Clean(name)
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

// rootMountEntry 'roots' a name creating a mount entry for the name.
//
// Returns:
// (1) MountEntry
// (2) Whether "name" is a rooted name or not (if not, the namespace roots
//     configured in "ns" will be used).
func (ns *namespace) rootMountEntry(name string, opts ...naming.NamespaceOpt) (*naming.MountEntry, bool) {
	_, name = security.SplitPatternName(naming.Clean(name))
	e := new(naming.MountEntry)
	deadline := vdltime.Deadline{time.Now().Add(time.Hour)} // plenty of time for a call
	address, suffix := naming.SplitAddressName(name)
	if len(address) == 0 {
		e.ServesMountTable = true
		e.Name = name
		ns.RLock()
		defer ns.RUnlock()
		for _, r := range ns.roots {
			e.Servers = append(e.Servers, naming.MountedServer{Server: r, Deadline: deadline})
		}
		return e, false
	}
	servesMT := true
	if ep, err := inaming.NewEndpoint(address); err == nil {
		servesMT = ep.ServesMountTable()
	}
	e.ServesMountTable = servesMT
	e.Name = suffix
	e.Servers = []naming.MountedServer{{Server: naming.JoinAddressName(address, ""), Deadline: deadline}}
	return e, true
}

// notAnMT returns true if the error indicates this isn't a mounttable server.
func notAnMT(err error) bool {
	switch verror.ErrorID(err) {
	case verror.ErrBadArg.ID:
		// This should cover "rpc: wrong number of in-args".
		return true
	case verror.ErrNoExist.ID, verror.ErrUnknownMethod.ID, verror.ErrUnknownSuffix.ID:
		// This should cover "rpc: unknown method", "rpc: dispatcher not
		// found", and dispatcher Lookup not found errors.
		return true
	case verror.ErrBadProtocol.ID:
		// This covers "rpc: response decoding failed: EOF".
		return true
	}
	return false
}

// All operations against the mount table service use this fixed timeout unless overridden.
const callTimeout = 30 * time.Second

// withTimeout returns a new context if the orinal has no timeout set.
func withTimeout(ctx *context.T) *context.T {
	if _, ok := ctx.Deadline(); !ok {
		ctx, _ = context.WithTimeout(ctx, callTimeout)
	}
	return ctx
}

// withTimeoutAndCancel returns a new context with a deadline and a cancellation function.
func withTimeoutAndCancel(ctx *context.T) (nctx *context.T, cancel context.CancelFunc) {
	if _, ok := ctx.Deadline(); !ok {
		nctx, cancel = context.WithTimeout(ctx, callTimeout)
	} else {
		nctx, cancel = context.WithCancel(ctx)
	}
	return
}

// CacheCtl implements naming.Namespace.CacheCtl
func (ns *namespace) CacheCtl(ctls ...naming.CacheCtl) []naming.CacheCtl {
	defer apilog.LogCallf(nil, "ctls...=%v", ctls)(nil, "") // gologcop: DO NOT EDIT, MUST BE FIRST STATEMENT
	for _, c := range ctls {
		switch v := c.(type) {
		case naming.DisableCache:
			ns.Lock()
			if _, isDisabled := ns.resolutionCache.(nullCache); isDisabled {
				if !v {
					ns.resolutionCache = newTTLCache()
				}
			} else {
				if v {
					ns.resolutionCache = newNullCache()
				}
			}
			ns.Unlock()
		}
	}
	ns.RLock()
	defer ns.RUnlock()
	if _, isDisabled := ns.resolutionCache.(nullCache); isDisabled {
		return []naming.CacheCtl{naming.DisableCache(true)}
	}
	return nil
}

func getCallOpts(opts []naming.NamespaceOpt) []rpc.CallOpt {
	var out []rpc.CallOpt
	for _, o := range opts {
		if co, ok := o.(rpc.CallOpt); ok {
			out = append(out, co)
		}
	}
	return out
}
