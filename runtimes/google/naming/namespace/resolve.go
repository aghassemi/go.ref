package namespace

import (
	"errors"
	"fmt"
	"runtime"

	"veyron.io/veyron/veyron2/context"
	"veyron.io/veyron/veyron2/ipc"
	"veyron.io/veyron/veyron2/naming"
	"veyron.io/veyron/veyron2/options"
	verror "veyron.io/veyron/veyron2/verror2"
	"veyron.io/veyron/veyron2/vlog"
)

func (ns *namespace) resolveAgainstMountTable(ctx context.T, client ipc.Client, e *naming.MountEntry, pattern string) (*naming.MountEntry, error) {
	// Try each server till one answers.
	finalErr := errors.New("no servers to resolve query")
	for _, s := range e.Servers {
		var pattern_and_name string
		name := naming.JoinAddressName(s.Server, e.Name)
		if pattern != "" {
			pattern_and_name = naming.JoinAddressName(s.Server, fmt.Sprintf("[%s]%s", pattern, e.Name))
		} else {
			pattern_and_name = name
		}
		// First check the cache.
		if ne, err := ns.resolutionCache.lookup(name); err == nil {
			vlog.VI(2).Infof("resolveAMT %s from cache -> %v", name, convertServersToStrings(ne.Servers, ne.Name))
			return &ne, nil
		}
		// Not in cache, call the real server.
		callCtx, _ := ctx.WithTimeout(callTimeout)
		call, err := client.StartCall(callCtx, pattern_and_name, "ResolveStepX", nil, options.NoResolve(true))
		if err != nil {
			finalErr = err
			vlog.VI(2).Infof("ResolveStep.StartCall %s failed: %s", name, err)
			continue
		}
		var entry naming.VDLMountEntry
		ierr := call.Finish(&entry, &err)
		if ierr != nil {
			// Internal/system error.
			finalErr = ierr
			vlog.VI(2).Infof("ResolveStep.Finish %s failed: %s", name, ierr)
			continue
		}
		// If any replica says the name doesn't exist, return that fact.
		if err != nil {
			if verror.Is(err, naming.ErrNoSuchName.ID) || verror.Is(err, naming.ErrNoSuchNameRoot.ID) {
				return nil, err
			}
			finalErr = err
			vlog.VI(2).Infof("ResolveStep %s failed: %s", name, err)
			continue
		}
		// Add result to cache.
		ne := convertMountEntry(&entry)
		ns.resolutionCache.remember(name, ne)
		vlog.VI(2).Infof("resolveAMT %s -> %v", name, *ne)
		return ne, nil
	}
	return nil, finalErr
}

func terminal(e *naming.MountEntry) bool {
	return len(e.Name) == 0
}

// ResolveX implements veyron2/naming.Namespace.
func (ns *namespace) ResolveX(ctx context.T, name string, opts ...naming.ResolveOpt) (*naming.MountEntry, error) {
	defer vlog.LogCall()()
	e, _ := ns.rootMountEntry(name)
	if vlog.V(2) {
		_, file, line, _ := runtime.Caller(1)
		vlog.Infof("ResolveX(%s) called from %s:%d", name, file, line)
		vlog.Infof("ResolveX(%s) -> rootMountEntry %v", name, *e)
	}
	if len(e.Servers) == 0 {
		return nil, verror.Make(naming.ErrNoSuchName, ctx, name)
	}
	pattern := getRootPattern(opts)
	// Iterate walking through mount table servers.
	for remaining := ns.maxResolveDepth; remaining > 0; remaining-- {
		vlog.VI(2).Infof("ResolveX(%s) loop %v", name, *e)
		if !e.ServesMountTable() || terminal(e) {
			vlog.VI(1).Infof("ResolveX(%s) -> %v", name, *e)
			return e, nil
		}
		var err error
		curr := e
		if e, err = ns.resolveAgainstMountTable(ctx, ns.rt.Client(), curr, pattern); err != nil {
			// If the name could not be found in the mount table, return an error.
			if verror.Is(err, naming.ErrNoSuchNameRoot.ID) {
				err = verror.Make(naming.ErrNoSuchName, ctx, name)
			}
			if verror.Is(err, naming.ErrNoSuchName.ID) {
				vlog.VI(1).Infof("ResolveX(%s) -> (NoSuchName: %v)", name, curr)
				return nil, err
			}
			if verror.Is(err, verror.NotTrusted.ID) {
				vlog.VI(1).Infof("ResolveX(%s) -> (NotTrusted: %v)", name, curr)
				return nil, err

			}
			// Any other failure (server not found, no ResolveStep
			// method, etc.) are a sign that iterative resolution can
			// stop.
			vlog.VI(1).Infof("ResolveX(%s) -> %v (%s)", name, curr, err)
			return curr, nil
		}
		pattern = ""
	}
	return nil, verror.Make(naming.ErrResolutionDepthExceeded, ctx)
}

// Resolve implements veyron2/naming.Namespace.
func (ns *namespace) Resolve(ctx context.T, name string, opts ...naming.ResolveOpt) ([]string, error) {
	defer vlog.LogCall()()
	e, err := ns.ResolveX(ctx, name, opts...)
	if err != nil {
		return nil, err
	}
	return naming.ToStringSlice(e), nil
}

// ResolveToMountTableX implements veyron2/naming.Namespace.
func (ns *namespace) ResolveToMountTableX(ctx context.T, name string, opts ...naming.ResolveOpt) (*naming.MountEntry, error) {
	defer vlog.LogCall()()
	e, _ := ns.rootMountEntry(name)
	if vlog.V(2) {
		_, file, line, _ := runtime.Caller(1)
		vlog.Infof("ResolveToMountTableX(%s) called from %s:%d", name, file, line)
		vlog.Infof("ResolveToMountTableX(%s) -> rootNames %v", name, e)
	}
	if len(e.Servers) == 0 {
		return nil, verror.Make(naming.ErrNoMountTable, ctx)
	}
	pattern := getRootPattern(opts)
	last := e
	for remaining := ns.maxResolveDepth; remaining > 0; remaining-- {
		vlog.VI(2).Infof("ResolveToMountTableX(%s) loop %v", name, e)
		var err error
		curr := e
		// If the next name to resolve doesn't point to a mount table, we're done.
		if !e.ServesMountTable() || terminal(e) {
			vlog.VI(1).Infof("ResolveToMountTableX(%s) -> %v", name, last)
			return last, nil
		}
		if e, err = ns.resolveAgainstMountTable(ctx, ns.rt.Client(), e, pattern); err != nil {
			if verror.Is(err, naming.ErrNoSuchNameRoot.ID) {
				vlog.VI(1).Infof("ResolveToMountTableX(%s) -> %v (NoSuchRoot: %v)", name, last, curr)
				return last, nil
			}
			if verror.Is(err, naming.ErrNoSuchName.ID) {
				vlog.VI(1).Infof("ResolveToMountTableX(%s) -> %v (NoSuchName: %v)", name, curr, curr)
				return curr, nil
			}
			// Lots of reasons why another error can happen.  We are trying
			// to single out "this isn't a mount table".
			// TODO(p); make this less of a hack, make a specific verror code
			// that means "we are up but don't implement what you are
			// asking for".
			if notAnMT(err) {
				vlog.VI(1).Infof("ResolveToMountTableX(%s) -> %v", name, last)
				return last, nil
			}
			// TODO(caprita): If the server is unreachable for
			// example, we may still want to return its parent
			// mounttable rather than an error.
			vlog.VI(1).Infof("ResolveToMountTableX(%s) -> %v", name, err)
			return nil, err
		}
		last = curr
		pattern = ""
	}
	return nil, verror.Make(naming.ErrResolutionDepthExceeded, ctx)
}

// ResolveToMountTable implements veyron2/naming.Namespace.
func (ns *namespace) ResolveToMountTable(ctx context.T, name string, opts ...naming.ResolveOpt) ([]string, error) {
	defer vlog.LogCall()()
	e, err := ns.ResolveToMountTableX(ctx, name, opts...)
	if err != nil {
		return nil, err
	}
	return naming.ToStringSlice(e), nil
}

func finishUnresolve(call ipc.Call) ([]string, error) {
	var newNames []string
	var unresolveErr error
	if err := call.Finish(&newNames, &unresolveErr); err != nil {
		return nil, err
	}
	return newNames, unresolveErr
}

// TODO(caprita): UnresolveStep no longer exists.
func unresolveAgainstServer(ctx context.T, client ipc.Client, names []string) ([]string, error) {
	finalErr := errors.New("no servers to unresolve")
	for _, name := range names {
		callCtx, _ := ctx.WithTimeout(callTimeout)
		call, err := client.StartCall(callCtx, name, "UnresolveStep", nil, options.NoResolve(true))
		if err != nil {
			finalErr = err
			vlog.VI(2).Infof("StartCall %q.UnresolveStep() failed: %s", name, err)
			continue

		}
		newNames, err := finishUnresolve(call)
		if err == nil {
			return newNames, nil
		}
		finalErr = err
		vlog.VI(2).Infof("Finish %s failed: %v", name, err)
	}
	return nil, finalErr
}

// TODO(caprita): Unresolve currently picks the first responsive server as it
// goes up the ancestry line.  This means that, if a service has several
// ancestors (each with potentially more than one replica), the first responsive
// replica of the first responsive ancestor is preferred.  In particular,
// other branches are ignored.  We need to figure out a desired policy for
// selecting the right branch (or should we return a representative of all
// branches?).

// Unesolve implements veyron2/naming.Namespace.
func (ns *namespace) Unresolve(ctx context.T, name string) ([]string, error) {
	defer vlog.LogCall()()
	vlog.VI(2).Infof("Unresolve %s", name)
	names, err := ns.Resolve(ctx, name)
	if err != nil {
		return nil, err
	}
	for remaining := ns.maxResolveDepth; remaining > 0; remaining-- {
		vlog.VI(2).Infof("Unresolve loop %s", names)
		curr := names
		if names, err = unresolveAgainstServer(ctx, ns.rt.Client(), names); err != nil {
			return nil, err
		}
		if len(names) == 0 {
			return curr, nil
		}
	}
	return nil, verror.Make(naming.ErrResolutionDepthExceeded, ctx)
}

// FlushCache flushes the most specific entry found for name.  It returns true if anything was
// actually flushed.
func (ns *namespace) FlushCacheEntry(name string) bool {
	defer vlog.LogCall()()
	flushed := false
	for _, n := range ns.rootName(name) {
		// Walk the cache as we would in a resolution.  Unlike a resolution, we have to follow
		// all branches since we want to flush all entries at which we might end up whereas in a resolution,
		// we stop with the first branch that works.
		if e, err := ns.resolutionCache.lookup(n); err == nil {
			// Recurse.
			for _, s := range e.Servers {
				flushed = flushed || ns.FlushCacheEntry(naming.Join(s.Server, e.Name))
			}
			if !flushed {
				// Forget the entry we just used.
				ns.resolutionCache.forget([]string{naming.TrimSuffix(n, e.Name)})
				flushed = true
			}
		}
	}
	return flushed
}

func getRootPattern(opts []naming.ResolveOpt) string {
	for _, opt := range opts {
		if pattern, ok := opt.(naming.RootBlessingPatternOpt); ok {
			return string(pattern)
		}
	}
	return ""
}
