// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rt

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"v.io/x/lib/pubsub"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/i18n"
	"v.io/v23/namespace"
	"v.io/v23/naming"
	"v.io/v23/options"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/v23/verror"
	"v.io/v23/vtrace"
	"v.io/x/lib/metadata"
	"v.io/x/lib/vlog"
	"v.io/x/ref/lib/apilog"
	"v.io/x/ref/lib/flags"
	"v.io/x/ref/lib/stats"
	_ "v.io/x/ref/lib/stats/sysstats"
	"v.io/x/ref/runtime/internal/lib/dependency"
	inaming "v.io/x/ref/runtime/internal/naming"
	inamespace "v.io/x/ref/runtime/internal/naming/namespace"
	irpc "v.io/x/ref/runtime/internal/rpc"
	"v.io/x/ref/runtime/internal/rpc/stream"
	imanager "v.io/x/ref/runtime/internal/rpc/stream/manager"
	ivtrace "v.io/x/ref/runtime/internal/vtrace"
)

type contextKey int

const (
	streamManagerKey = contextKey(iota)
	clientKey
	namespaceKey
	principalKey
	backgroundKey
	reservedNameKey

	// initKey is used to store values that are only set at init time.
	initKey
)

type initData struct {
	appCycle          v23.AppCycle
	listenSpec        *rpc.ListenSpec
	protocols         []string
	settingsPublisher *pubsub.Publisher
	settingsName      string
}

type vtraceDependency struct{}

// Runtime implements the v23.Runtime interface.
// Please see the interface definition for documentation of the
// individiual methods.
type Runtime struct {
	deps *dependency.Graph
}

func Init(
	ctx *context.T,
	appCycle v23.AppCycle,
	protocols []string,
	listenSpec *rpc.ListenSpec,
	settingsPublisher *pubsub.Publisher,
	settingsName string,
	flags flags.RuntimeFlags,
	reservedDispatcher rpc.Dispatcher) (*Runtime, *context.T, v23.Shutdown, error) {
	r := &Runtime{deps: dependency.NewGraph()}

	ctx = context.WithValue(ctx, initKey, &initData{
		protocols:         protocols,
		listenSpec:        listenSpec,
		appCycle:          appCycle,
		settingsPublisher: settingsPublisher,
		settingsName:      settingsName,
	})

	if reservedDispatcher != nil {
		ctx = context.WithValue(ctx, reservedNameKey, reservedDispatcher)
	}

	err := vlog.ConfigureLibraryLoggerFromFlags()
	if err != nil && err != vlog.ErrConfigured {
		return nil, nil, nil, err
	}
	// We want to print out metadata only into the log files, to avoid
	// spamming stderr, see #1246.
	//
	// TODO(caprita): We should add it to the log file header information;
	// since that requires changes to the llog and vlog packages, for now we
	// condition printing of metadata on having specified an explicit
	// log_dir for the program.  It's a hack, but it gets us the metadata
	// to device manager-run apps and avoids it for command-lines, which is
	// a good enough approximation.
	if vlog.Log.LogDir() != os.TempDir() {
		vlog.Infof(metadata.ToXML())
	}

	// Setup the initial trace.
	ctx, err = ivtrace.Init(ctx, flags.Vtrace)
	if err != nil {
		return nil, nil, nil, err
	}
	ctx, _ = vtrace.WithNewTrace(ctx)
	r.addChild(ctx, vtraceDependency{}, func() {
		vtrace.FormatTraces(os.Stderr, vtrace.GetStore(ctx).TraceRecords(), nil)
	})

	// Setup i18n.
	ctx = i18n.WithLangID(ctx, i18n.LangIDFromEnv())
	if len(flags.I18nCatalogue) != 0 {
		cat := i18n.Cat()
		for _, filename := range strings.Split(flags.I18nCatalogue, ",") {
			err := cat.MergeFromFile(filename)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s: i18n: error reading i18n catalogue file %q: %s\n", os.Args[0], filename, err)
			}
		}
	}

	// Setup the program name.
	ctx = verror.WithComponentName(ctx, filepath.Base(os.Args[0]))

	// Enable signal handling.
	r.initSignalHandling(ctx)

	// Set the initial namespace.
	ctx, _, err = r.setNewNamespace(ctx, flags.NamespaceRoots...)
	if err != nil {
		return nil, nil, nil, err
	}

	// Create and set the principal
	principal, deps, shutdown, err := r.initPrincipal(ctx, flags.Credentials)
	if err != nil {
		return nil, nil, nil, err
	}
	ctx, err = r.setPrincipal(ctx, principal, shutdown, deps...)
	if err != nil {
		return nil, nil, nil, err
	}

	// Setup authenticated "networking"
	ctx, err = r.WithNewStreamManager(ctx)
	if err != nil {
		return nil, nil, nil, err
	}

	return r, r.WithBackgroundContext(ctx), r.shutdown, nil
}

func (r *Runtime) addChild(ctx *context.T, me interface{}, stop func(), dependsOn ...interface{}) error {
	if err := r.deps.Depend(me, dependsOn...); err != nil {
		stop()
		return err
	} else if done := ctx.Done(); done != nil {
		go func() {
			<-done
			finish := r.deps.CloseAndWait(me)
			stop()
			finish()
		}()
	}
	return nil
}

func (r *Runtime) Init(ctx *context.T) error {
	defer apilog.LogCall(ctx)(ctx) // gologcop: DO NOT EDIT, MUST BE FIRST STATEMENT
	return r.initMgmt(ctx)
}

func (r *Runtime) shutdown() {
	r.deps.CloseAndWaitForAll()
	vlog.FlushLog()
}

func (r *Runtime) initSignalHandling(ctx *context.T) {
	// TODO(caprita): Given that our device manager implementation is to
	// kill all child apps when the device manager dies, we should
	// enable SIGHUP on apps by default.

	// Automatically handle SIGHUP to prevent applications started as
	// daemons from being killed.  The developer can choose to still listen
	// on SIGHUP and take a different action if desired.
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGHUP)
	go func() {
		for {
			sig, ok := <-signals
			if !ok {
				break
			}
			vlog.Infof("Received signal %v", sig)
		}
	}()
	r.addChild(ctx, signals, func() {
		signal.Stop(signals)
		close(signals)
	})
}

func (*Runtime) NewEndpoint(ep string) (naming.Endpoint, error) {
	defer apilog.LogCallf(nil, "ep=%.10s...", ep)(nil, "") // gologcop: DO NOT EDIT, MUST BE FIRST STATEMENT
	return inaming.NewEndpoint(ep)
}

func (r *Runtime) NewServer(ctx *context.T, opts ...rpc.ServerOpt) (rpc.Server, error) {
	defer apilog.LogCallf(ctx, "opts...=%v", opts)(ctx, "") // gologcop: DO NOT EDIT, MUST BE FIRST STATEMENT
	// Create a new RoutingID (and StreamManager) for each server.
	sm, err := newStreamManager()
	if err != nil {
		return nil, fmt.Errorf("failed to create rpc/stream/Manager: %v", err)
	}

	ns, _ := ctx.Value(namespaceKey).(namespace.T)
	principal, _ := ctx.Value(principalKey).(security.Principal)
	client, _ := ctx.Value(clientKey).(rpc.Client)

	otherOpts := append([]rpc.ServerOpt{}, opts...)

	if reservedDispatcher := r.GetReservedNameDispatcher(ctx); reservedDispatcher != nil {
		otherOpts = append(otherOpts, irpc.ReservedNameDispatcher{
			Dispatcher: reservedDispatcher,
		})
	}

	id, _ := ctx.Value(initKey).(*initData)
	if id.protocols != nil {
		otherOpts = append(otherOpts, irpc.PreferredServerResolveProtocols(id.protocols))
	}
	if !hasServerBlessingsOpt(opts) && principal != nil {
		otherOpts = append(otherOpts, options.ServerBlessings{
			Blessings: principal.BlessingStore().Default(),
		})
	}
	server, err := irpc.InternalNewServer(ctx, sm, ns, id.settingsPublisher, id.settingsName, r.GetClient(ctx), principal, otherOpts...)
	if err != nil {
		return nil, err
	}
	stop := func() {
		if err := server.Stop(); err != nil {
			vlog.Errorf("A server could not be stopped: %v", err)
		}
		sm.Shutdown()
	}
	deps := []interface{}{client, vtraceDependency{}}
	if principal != nil {
		deps = append(deps, principal)
	}
	if err = r.addChild(ctx, server, stop, deps...); err != nil {
		return nil, err
	}
	return server, nil
}

func hasServerBlessingsOpt(opts []rpc.ServerOpt) bool {
	for _, o := range opts {
		if _, ok := o.(options.ServerBlessings); ok {
			return true
		}
	}
	return false
}

func newStreamManager() (stream.Manager, error) {
	rid, err := naming.NewRoutingID()
	if err != nil {
		return nil, err
	}
	sm := imanager.InternalNew(rid)
	return sm, nil
}

func (r *Runtime) setNewStreamManager(ctx *context.T) (*context.T, error) {
	sm, err := newStreamManager()
	if err != nil {
		return nil, err
	}
	newctx := context.WithValue(ctx, streamManagerKey, sm)
	if err = r.addChild(ctx, sm, sm.Shutdown); err != nil {
		return ctx, err
	}
	return newctx, err
}

func (r *Runtime) WithNewStreamManager(ctx *context.T) (*context.T, error) {
	defer apilog.LogCall(ctx)(ctx) // gologcop: DO NOT EDIT, MUST BE FIRST STATEMENT
	newctx, err := r.setNewStreamManager(ctx)
	if err != nil {
		return ctx, err
	}

	// Create a new client since it depends on the stream manager.
	newctx, _, err = r.WithNewClient(newctx)
	if err != nil {
		return ctx, err
	}
	return newctx, nil
}

func (r *Runtime) setPrincipal(ctx *context.T, principal security.Principal, shutdown func(), deps ...interface{}) (*context.T, error) {
	if principal != nil {
		// We uniquely identify a principal with "security/principal/<publicKey>"
		principalName := "security/principal/" + principal.PublicKey().String()
		stats.NewStringFunc(principalName+"/blessingstore", principal.BlessingStore().DebugString)
		stats.NewStringFunc(principalName+"/blessingroots", principal.Roots().DebugString)
	}
	ctx = context.WithValue(ctx, principalKey, principal)
	return ctx, r.addChild(ctx, principal, shutdown, deps...)
}

func (r *Runtime) WithPrincipal(ctx *context.T, principal security.Principal) (*context.T, error) {
	defer apilog.LogCallf(ctx, "principal=")(ctx, "") // gologcop: DO NOT EDIT, MUST BE FIRST STATEMENT
	var err error
	newctx := ctx

	// TODO(mattr, suharshs): If there user gives us some principal that has dependencies
	// we don't know about, we will not honour those dependencies during shutdown.
	// For example if they create an agent principal with some client, we don't know
	// about that, so servers based of this new principal will not prevent the client
	// from terminating early.
	if newctx, err = r.setPrincipal(ctx, principal, func() {}); err != nil {
		return ctx, err
	}
	if newctx, err = r.setNewStreamManager(newctx); err != nil {
		return ctx, err
	}
	if newctx, _, err = r.setNewNamespace(newctx, r.GetNamespace(ctx).Roots()...); err != nil {
		return ctx, err
	}
	if newctx, _, err = r.WithNewClient(newctx); err != nil {
		return ctx, err
	}

	return newctx, nil
}

func (*Runtime) GetPrincipal(ctx *context.T) security.Principal {
	defer apilog.LogCall(ctx)(ctx) // gologcop: DO NOT EDIT, MUST BE FIRST STATEMENT
	p, _ := ctx.Value(principalKey).(security.Principal)
	return p
}

func (r *Runtime) WithNewClient(ctx *context.T, opts ...rpc.ClientOpt) (*context.T, rpc.Client, error) {
	defer apilog.LogCallf(ctx, "opts...=%v", opts)(ctx, "") // gologcop: DO NOT EDIT, MUST BE FIRST STATEMENT
	otherOpts := append([]rpc.ClientOpt{}, opts...)

	p, _ := ctx.Value(principalKey).(security.Principal)
	sm, _ := ctx.Value(streamManagerKey).(stream.Manager)
	ns, _ := ctx.Value(namespaceKey).(namespace.T)
	otherOpts = append(otherOpts, imanager.DialTimeout(5*time.Minute))

	if id, _ := ctx.Value(initKey).(*initData); id.protocols != nil {
		otherOpts = append(otherOpts, irpc.PreferredProtocols(id.protocols))
	}
	client, err := irpc.InternalNewClient(sm, ns, otherOpts...)
	if err != nil {
		return ctx, nil, err
	}
	newctx := context.WithValue(ctx, clientKey, client)
	deps := []interface{}{sm, vtraceDependency{}}
	if p != nil {
		deps = append(deps, p)
	}
	if err = r.addChild(ctx, client, client.Close, deps...); err != nil {
		return ctx, nil, err
	}
	return newctx, client, err
}

func (*Runtime) GetClient(ctx *context.T) rpc.Client {
	defer apilog.LogCall(ctx)(ctx) // gologcop: DO NOT EDIT, MUST BE FIRST STATEMENT
	cl, _ := ctx.Value(clientKey).(rpc.Client)
	return cl
}

func (r *Runtime) setNewNamespace(ctx *context.T, roots ...string) (*context.T, namespace.T, error) {
	ns, err := inamespace.New(roots...)
	if err != nil {
		return nil, nil, err
	}

	if oldNS := r.GetNamespace(ctx); oldNS != nil {
		ns.CacheCtl(oldNS.CacheCtl()...)
	}

	if err == nil {
		ctx = context.WithValue(ctx, namespaceKey, ns)
	}
	return ctx, ns, err
}

func (r *Runtime) WithNewNamespace(ctx *context.T, roots ...string) (*context.T, namespace.T, error) {
	defer apilog.LogCallf(ctx, "roots...=%v", roots)(ctx, "") // gologcop: DO NOT EDIT, MUST BE FIRST STATEMENT
	newctx, ns, err := r.setNewNamespace(ctx, roots...)
	if err != nil {
		return ctx, nil, err
	}

	// Replace the client since it depends on the namespace.
	newctx, _, err = r.WithNewClient(newctx)
	if err != nil {
		return ctx, nil, err
	}

	return newctx, ns, err
}

func (*Runtime) GetNamespace(ctx *context.T) namespace.T {
	// nologcall
	ns, _ := ctx.Value(namespaceKey).(namespace.T)
	return ns
}

func (*Runtime) GetAppCycle(ctx *context.T) v23.AppCycle {
	// nologcall
	id, _ := ctx.Value(initKey).(*initData)
	return id.appCycle
}

func (*Runtime) GetListenSpec(ctx *context.T) rpc.ListenSpec {
	// nologcall
	if id, _ := ctx.Value(initKey).(*initData); id.listenSpec != nil {
		return id.listenSpec.Copy()
	}
	return rpc.ListenSpec{}
}

func (*Runtime) WithBackgroundContext(ctx *context.T) *context.T {
	defer apilog.LogCall(ctx)(ctx) // gologcop: DO NOT EDIT, MUST BE FIRST STATEMENT
	// Note we add an extra context with a nil value here.
	// This prevents users from travelling back through the
	// chain of background contexts.
	ctx = context.WithValue(ctx, backgroundKey, nil)
	return context.WithValue(ctx, backgroundKey, ctx)
}

func (*Runtime) GetBackgroundContext(ctx *context.T) *context.T {
	// nologcall
	bctx, _ := ctx.Value(backgroundKey).(*context.T)
	if bctx == nil {
		// There should always be a background context.  If we don't find
		// it, that means that the user passed us the background context
		// in hopes of following the chain.  Instead we just give them
		// back what they sent in, which is correct.
		return ctx
	}
	return bctx
}

func (*Runtime) WithReservedNameDispatcher(ctx *context.T, d rpc.Dispatcher) *context.T {
	defer apilog.LogCallf(ctx, "d=")(ctx, "") // gologcop: DO NOT EDIT, MUST BE FIRST STATEMENT
	return context.WithValue(ctx, reservedNameKey, d)
}

func (*Runtime) GetReservedNameDispatcher(ctx *context.T) rpc.Dispatcher {
	// nologcall
	if d, ok := ctx.Value(reservedNameKey).(rpc.Dispatcher); ok {
		return d
	}
	return nil
}
