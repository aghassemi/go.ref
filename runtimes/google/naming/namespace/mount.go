package namespace

import (
	"fmt"
	"time"

	inaming "v.io/core/veyron/runtimes/google/naming"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/ipc"
	"v.io/v23/naming"
	"v.io/v23/options"
	"v.io/v23/security"
	"v.io/v23/vlog"
)

type status struct {
	id  string
	err error
}

// mountIntoMountTable mounts a single server into a single mount table.
func mountIntoMountTable(ctx *context.T, client ipc.Client, name, server string, patterns []security.BlessingPattern, ttl time.Duration, flags naming.MountFlag, id string) (s status) {
	s.id = id
	ctx, _ = context.WithTimeout(ctx, callTimeout)
	call, err := client.StartCall(ctx, name, "MountX", []interface{}{server, patterns, uint32(ttl.Seconds()), flags}, options.NoResolve{})
	s.err = err
	if err != nil {
		return
	}
	s.err = call.Finish()
	return
}

// unmountFromMountTable removes a single mounted server from a single mount table.
func unmountFromMountTable(ctx *context.T, client ipc.Client, name, server string, id string) (s status) {
	s.id = id
	ctx, _ = context.WithTimeout(ctx, callTimeout)
	call, err := client.StartCall(ctx, name, "Unmount", []interface{}{server}, options.NoResolve{})
	s.err = err
	if err != nil {
		return
	}
	s.err = call.Finish()
	return
}

// nameToRID converts a name to a routing ID string. If a routing ID can't be obtained,
// it just returns the name.
func nameToRID(name string) string {
	address, _ := naming.SplitAddressName(name)
	if ep, err := inaming.NewEndpoint(address); err == nil {
		return ep.RID.String()
	}
	return name
}

// collectStati collects n status messages from channel c and returns an error if, for
// any id, there is no successful reply.
func collectStati(c chan status, n int) error {
	// Make a map indexed by the routing id (or address if routing id not found) of
	// each mount table.  A mount table may be reachable via multiple addresses but
	// each address should have the same routing id.  We should only return an error
	// if any of the ids had no successful mounts.
	statusByID := make(map[string]error)
	// Get the status of each request.
	for i := 0; i < n; i++ {
		s := <-c
		if _, ok := statusByID[s.id]; !ok || s.err == nil {
			statusByID[s.id] = s.err
		}
	}
	// Return any error.
	for _, s := range statusByID {
		if s != nil {
			return s
		}
	}
	return nil
}

// dispatch executes f in parallel for each mount table implementing mTName.
func (ns *namespace) dispatch(ctx *context.T, mTName string, f func(*context.T, string, string) status, opts ...naming.ResolveOpt) error {
	// Resolve to all the mount tables implementing name.
	me, err := ns.ResolveToMountTable(ctx, mTName, opts...)
	if err != nil {
		return err
	}
	mts := me.Names()
	// Apply f to each of the returned mount tables.
	c := make(chan status, len(mts))
	for _, mt := range mts {
		go func(mt string) {
			c <- f(ctx, mt, nameToRID(mt))
		}(mt)
	}
	finalerr := collectStati(c, len(mts))
	// Forget any previous cached information about these names.
	ns.resolutionCache.forget(mts)
	return finalerr
}

func (ns *namespace) Mount(ctx *context.T, name, server string, ttl time.Duration, opts ...naming.MountOpt) error {
	defer vlog.LogCall()()

	var flags naming.MountFlag
	var patterns []string
	for _, o := range opts {
		// NB: used a switch since we'll be adding more options.
		switch v := o.(type) {
		case naming.ReplaceMountOpt:
			if v {
				flags |= naming.MountFlag(naming.Replace)
			}
		case naming.ServesMountTableOpt:
			if v {
				flags |= naming.MountFlag(naming.MT)
			}
		case naming.MountedServerBlessingsOpt:
			patterns = []string(v)
		}
	}
	if len(patterns) == 0 {
		// No patterns explicitly provided. Take the conservative
		// approach that the server being mounted is run by this local
		// process.
		p := v23.GetPrincipal(ctx)
		b := p.BlessingStore().Default()
		if b == nil {
			return fmt.Errorf("must provide a MountedServerBlessingsOpt")
		}
		for str, _ := range p.BlessingsInfo(b) {
			patterns = append(patterns, str)
		}
		vlog.VI(2).Infof("Mount(%s, %s): No MountedServerBlessingsOpt provided using %v", name, server, patterns)
	}

	client := v23.GetClient(ctx)
	// Mount the server in all the returned mount tables.
	f := func(ctx *context.T, mt, id string) status {
		return mountIntoMountTable(ctx, client, mt, server, str2pattern(patterns), ttl, flags, id)
	}
	err := ns.dispatch(ctx, name, f)
	vlog.VI(1).Infof("Mount(%s, %q, %v) -> %v", name, server, patterns, err)
	return err
}

func (ns *namespace) Unmount(ctx *context.T, name, server string) error {
	defer vlog.LogCall()()
	// Unmount the server from all the mount tables.
	client := v23.GetClient(ctx)
	f := func(context *context.T, mt, id string) status {
		return unmountFromMountTable(ctx, client, mt, server, id)
	}
	err := ns.dispatch(ctx, name, f)
	vlog.VI(1).Infof("Unmount(%s, %s) -> %v", name, server, err)
	return err
}

func str2pattern(strs []string) (ret []security.BlessingPattern) {
	ret = make([]security.BlessingPattern, len(strs))
	for i, s := range strs {
		ret[i] = security.BlessingPattern(s)
	}
	return
}
