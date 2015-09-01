// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rpc

import (
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"v.io/x/lib/netstate"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/flow"
	"v.io/v23/i18n"
	"v.io/v23/namespace"
	"v.io/v23/naming"
	"v.io/v23/options"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/v23/security/access"
	"v.io/v23/vdl"
	"v.io/v23/verror"
	"v.io/v23/vom"
	"v.io/v23/vtrace"

	"v.io/x/ref/lib/apilog"
	"v.io/x/ref/lib/pubsub"
	"v.io/x/ref/lib/stats"
	"v.io/x/ref/runtime/internal/lib/publisher"
	inaming "v.io/x/ref/runtime/internal/naming"
)

// TODO(mattr): add/removeAddresses
// TODO(mattr): dhcpLoop

type xserver struct {
	sync.Mutex
	// context used by the server to make internal RPCs, error messages etc.
	ctx               *context.T
	cancel            context.CancelFunc // function to cancel the above context.
	flowMgr           flow.Manager
	publisher         publisher.Publisher // publisher to publish mounttable mounts.
	settingsPublisher *pubsub.Publisher   // pubsub publisher for dhcp
	settingsName      string              // pubwsub stream name for dhcp
	dhcpState         *dhcpState          // dhcpState, nil if not using dhcp
	principal         security.Principal
	blessings         security.Blessings
	protoEndpoints    []*inaming.Endpoint
	chosenEndpoints   []*inaming.Endpoint

	// state of proxies keyed by the name of the proxy
	proxies map[string]proxyState

	disp               rpc.Dispatcher // dispatcher to serve RPCs
	dispReserved       rpc.Dispatcher // dispatcher for reserved methods
	active             sync.WaitGroup // active goroutines we've spawned.
	stoppedChan        chan struct{}  // closed when the server has been stopped.
	preferredProtocols []string       // protocols to use when resolving proxy name to endpoint.
	// We cache the IP networks on the device since it is not that cheap to read
	// network interfaces through os syscall.
	// TODO(jhahn): Add monitoring the network interface changes.
	ipNets           []*net.IPNet
	ns               namespace.T
	servesMountTable bool
	isLeaf           bool

	// TODO(cnicolaou): add roaming stats to rpcStats
	stats *rpcStats // stats for this server.
}

func InternalNewXServer(ctx *context.T, settingsPublisher *pubsub.Publisher, settingsName string, opts ...rpc.ServerOpt) (rpc.XServer, error) {
	ctx, cancel := context.WithRootCancel(ctx)
	flowMgr := v23.ExperimentalGetFlowManager(ctx)
	ns, principal := v23.GetNamespace(ctx), v23.GetPrincipal(ctx)
	statsPrefix := naming.Join("rpc", "server", "routing-id", flowMgr.RoutingID().String())
	s := &xserver{
		ctx:               ctx,
		cancel:            cancel,
		flowMgr:           flowMgr,
		principal:         principal,
		blessings:         principal.BlessingStore().Default(),
		publisher:         publisher.New(ctx, ns, publishPeriod),
		proxies:           make(map[string]proxyState),
		stoppedChan:       make(chan struct{}),
		ns:                ns,
		stats:             newRPCStats(statsPrefix),
		settingsPublisher: settingsPublisher,
		settingsName:      settingsName,
	}
	ipNets, err := ipNetworks()
	if err != nil {
		return nil, err
	}
	s.ipNets = ipNets

	for _, opt := range opts {
		switch opt := opt.(type) {
		case options.ServesMountTable:
			s.servesMountTable = bool(opt)
		case options.IsLeaf:
			s.isLeaf = bool(opt)
		case ReservedNameDispatcher:
			s.dispReserved = opt.Dispatcher
		case PreferredServerResolveProtocols:
			s.preferredProtocols = []string(opt)
		}
	}

	blessingsStatsName := naming.Join(statsPrefix, "security", "blessings")
	// TODO(caprita): revist printing the blessings with %s, and
	// instead expose them as a list.
	stats.NewString(blessingsStatsName).Set(fmt.Sprintf("%s", s.blessings))
	stats.NewStringFunc(blessingsStatsName, func() string {
		return fmt.Sprintf("%s (default)", s.principal.BlessingStore().Default())
	})
	return s, nil
}

func (s *xserver) Status() rpc.ServerStatus {
	return rpc.ServerStatus{}
}

func (s *xserver) WatchNetwork(ch chan<- rpc.NetworkChange) {
	defer apilog.LogCallf(nil, "ch=")(nil, "") // gologcop: DO NOT EDIT, MUST BE FIRST STATEMENT
	s.Lock()
	defer s.Unlock()
	if s.dhcpState != nil {
		s.dhcpState.watchers[ch] = struct{}{}
	}
}

func (s *xserver) UnwatchNetwork(ch chan<- rpc.NetworkChange) {
	defer apilog.LogCallf(nil, "ch=")(nil, "") // gologcop: DO NOT EDIT, MUST BE FIRST STATEMENT
	s.Lock()
	defer s.Unlock()
	if s.dhcpState != nil {
		delete(s.dhcpState.watchers, ch)
	}
}

// resolveToEndpoint resolves an object name or address to an endpoint.
func (s *xserver) resolveToEndpoint(address string) (string, error) {
	var resolved *naming.MountEntry
	var err error
	if s.ns != nil {
		if resolved, err = s.ns.Resolve(s.ctx, address); err != nil {
			return "", err
		}
	} else {
		// Fake a namespace resolution
		resolved = &naming.MountEntry{Servers: []naming.MountedServer{
			{Server: address},
		}}
	}
	// An empty set of protocols means all protocols...
	if resolved.Servers, err = filterAndOrderServers(resolved.Servers, s.preferredProtocols, s.ipNets); err != nil {
		return "", err
	}
	for _, n := range resolved.Names() {
		address, suffix := naming.SplitAddressName(n)
		if suffix != "" {
			continue
		}
		if ep, err := inaming.NewEndpoint(address); err == nil {
			return ep.String(), nil
		}
	}
	return "", verror.New(errFailedToResolveToEndpoint, s.ctx, address)
}

// createEndpoints creates appropriate inaming.Endpoint instances for
// all of the externally accessible network addresses that can be used
// to reach this server.
func (s *xserver) createEndpoints(lep naming.Endpoint, chooser netstate.AddressChooser) ([]*inaming.Endpoint, string, bool, error) {
	iep, ok := lep.(*inaming.Endpoint)
	if !ok {
		return nil, "", false, verror.New(errInternalTypeConversion, nil, fmt.Sprintf("%T", lep))
	}
	if !strings.HasPrefix(iep.Protocol, "tcp") &&
		!strings.HasPrefix(iep.Protocol, "ws") {
		// If not tcp, ws, or wsh, just return the endpoint we were given.
		return []*inaming.Endpoint{iep}, "", false, nil
	}
	host, port, err := net.SplitHostPort(iep.Address)
	if err != nil {
		return nil, "", false, err
	}
	addrs, unspecified, err := netstate.PossibleAddresses(iep.Protocol, host, chooser)
	if err != nil {
		return nil, port, false, err
	}

	ieps := make([]*inaming.Endpoint, 0, len(addrs))
	for _, addr := range addrs {
		n, err := inaming.NewEndpoint(lep.String())
		if err != nil {
			return nil, port, false, err
		}
		n.IsMountTable = s.servesMountTable
		n.Address = net.JoinHostPort(addr.String(), port)
		ieps = append(ieps, n)
	}
	return ieps, port, unspecified, nil
}

func (s *xserver) listen(ctx *context.T, listenSpec rpc.ListenSpec) error {
	s.Lock()
	defer s.Unlock()

	var lastErr error
	for _, addr := range listenSpec.Addrs {
		if len(addr.Address) > 0 {
			lastErr = s.flowMgr.Listen(ctx, addr.Protocol, addr.Address)
			s.ctx.VI(2).Infof("Listen(%q, %q, ...) failed: %v", addr.Protocol, addr.Address, lastErr)
		}
	}

	leps := s.flowMgr.ListeningEndpoints()
	if len(leps) == 0 {
		return verror.New(verror.ErrBadArg, s.ctx, verror.New(errNoListeners, s.ctx, lastErr))
	}

	roaming := false
	for _, ep := range leps {
		eps, _, eproaming, eperr := s.createEndpoints(ep, listenSpec.AddressChooser)
		s.chosenEndpoints = append(s.chosenEndpoints, eps...)
		if eproaming && eperr == nil {
			s.protoEndpoints = append(s.protoEndpoints, ep.(*inaming.Endpoint))
			roaming = true
		}
	}

	if roaming && s.dhcpState == nil && s.settingsPublisher != nil {
		// TODO(mattr): Support roaming.
	}

	s.active.Add(1)
	go s.acceptLoop(ctx)
	return nil
}

func (s *xserver) acceptLoop(ctx *context.T) error {
	var calls sync.WaitGroup
	defer func() {
		calls.Wait()
		s.active.Done()
		s.ctx.VI(1).Infof("rpc: Stopped accepting")
	}()
	for {
		// TODO(mattr): We need to interrupt Accept at some point.
		// Should we interrupt it by canceling the context?
		fl, err := s.flowMgr.Accept(ctx)
		if err != nil {
			s.ctx.VI(10).Infof("rpc: Accept failed: %v", err)
			return err
		}
		calls.Add(1)
		go func(fl flow.Flow) {
			defer calls.Done()
			fs, err := newXFlowServer(fl, s)
			if err != nil {
				s.ctx.VI(1).Infof("newFlowServer on %v failed", err)
				return
			}
			if err := fs.serve(); err != nil {
				// TODO(caprita): Logging errors here is too spammy. For example, "not
				// authorized" errors shouldn't be logged as server errors.
				// TODO(cnicolaou): revisit this when verror2 transition is
				// done.
				if err != io.EOF {
					s.ctx.VI(2).Infof("Flow.serve failed: %v", err)
				}
			}
		}(fl)
	}
}

func (s *server) serve(name string, obj interface{}, authorizer security.Authorizer) error {
	if obj == nil {
		return verror.New(verror.ErrBadArg, s.ctx, "nil object")
	}
	invoker, err := objectToInvoker(obj)
	if err != nil {
		return verror.New(verror.ErrBadArg, s.ctx, fmt.Sprintf("bad object: %v", err))
	}
	// TODO(mattr): Does this really need to be locked?
	s.Lock()
	s.isLeaf = true
	s.Unlock()
	return s.ServeDispatcher(name, &leafDispatcher{invoker, authorizer})
}

func (s *xserver) serveDispatcher(name string, disp rpc.Dispatcher) error {
	if disp == nil {
		return verror.New(verror.ErrBadArg, s.ctx, "nil dispatcher")
	}
	s.Lock()
	defer s.Unlock()
	vtrace.GetSpan(s.ctx).Annotate("Serving under name: " + name)
	s.disp = disp
	if len(name) > 0 {
		for _, ep := range s.chosenEndpoints {
			s.publisher.AddServer(ep.String())
		}
		s.publisher.AddName(name, s.servesMountTable, s.isLeaf)
	}
	return nil
}

func (s *xserver) AddName(name string) error {
	defer apilog.LogCallf(nil, "name=%.10s...", name)(nil, "") // gologcop: DO NOT EDIT, MUST BE FIRST STATEMENT
	if len(name) == 0 {
		return verror.New(verror.ErrBadArg, s.ctx, "name is empty")
	}
	s.Lock()
	defer s.Unlock()
	vtrace.GetSpan(s.ctx).Annotate("Serving under name: " + name)
	s.publisher.AddName(name, s.servesMountTable, s.isLeaf)
	return nil
}

func (s *xserver) RemoveName(name string) {
	defer apilog.LogCallf(nil, "name=%.10s...", name)(nil, "") // gologcop: DO NOT EDIT, MUST BE FIRST STATEMENT
	s.Lock()
	defer s.Unlock()
	vtrace.GetSpan(s.ctx).Annotate("Removed name: " + name)
	s.publisher.RemoveName(name)
}

func (s *xserver) Stop() error {
	defer apilog.LogCall(nil)(nil) // gologcop: DO NOT EDIT, MUST BE FIRST STATEMENT

	serverDebug := fmt.Sprintf("Dispatcher: %T, Status:[%v]", s.disp, s.Status())
	s.ctx.VI(1).Infof("Stop: %s", serverDebug)
	defer s.ctx.VI(1).Infof("Stop done: %s", serverDebug)

	s.Lock()
	if s.disp == nil {
		s.Unlock()
		return nil
	}
	s.disp = nil
	close(s.stoppedChan)
	s.Unlock()

	// Delete the stats object.
	s.stats.stop()

	// Note, It's safe to Stop/WaitForStop on the publisher outside of the
	// server lock, since publisher is safe for concurrent access.
	// Stop the publisher, which triggers unmounting of published names.
	s.publisher.Stop()

	// Wait for the publisher to be done unmounting before we can proceed to
	// close the listeners (to minimize the number of mounted names pointing
	// to endpoint that are no longer serving).
	//
	// TODO(caprita): See if make sense to fail fast on rejecting
	// connections once listeners are closed, and parallelize the publisher
	// and listener shutdown.
	s.publisher.WaitForStop()

	s.Lock()

	// TODO(mattr): What should we do when we stop a server now?  We need to
	// interrupt Accept at some point, but it's weird to stop the flowmanager.
	// Close all listeners.  No new flows will be accepted, while in-flight
	// flows will continue until they terminate naturally.
	// nListeners := len(s.listeners)
	// errCh := make(chan error, nListeners)
	// for ln, _ := range s.listeners {
	// 	go func(ln stream.Listener) {
	// 		errCh <- ln.Close()
	// 	}(ln)
	// }

	if dhcp := s.dhcpState; dhcp != nil {
		// TODO(cnicolaou,caprita): investigate not having to close and drain
		// the channel here. It's a little awkward right now since we have to
		// be careful to not close the channel in two places, i.e. here and
		// and from the publisher's Shutdown method.
		if err := dhcp.publisher.CloseFork(dhcp.name, dhcp.ch); err == nil {
		drain:
			for {
				select {
				case v := <-dhcp.ch:
					if v == nil {
						break drain
					}
				default:
					close(dhcp.ch)
					break drain
				}
			}
		}
	}

	s.Unlock()

	// At this point, we are guaranteed that no new requests are going to be
	// accepted.

	// Wait for the publisher and active listener + flows to finish.
	done := make(chan struct{}, 1)
	go func() { s.active.Wait(); done <- struct{}{} }()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		s.ctx.Errorf("%s: Timedout waiting for goroutines to stop", serverDebug)
		// TODO(mattr): This doesn't make sense, shouldn't we not wait after timing out?
		<-done
		s.ctx.Infof("%s: Done waiting.", serverDebug)
	}

	s.cancel()
	return nil
}

// flowServer implements the RPC server-side protocol for a single RPC, over a
// flow that's already connected to the client.
type xflowServer struct {
	ctx    *context.T     // context associated with the RPC
	server *xserver       // rpc.Server that this flow server belongs to
	disp   rpc.Dispatcher // rpc.Dispatcher that will serve RPCs on this flow
	dec    *vom.Decoder   // to decode requests and args from the client
	enc    *vom.Encoder   // to encode responses and results to the client
	flow   flow.Flow      // underlying flow

	// Fields filled in during the server invocation.
	clientBlessings  security.Blessings
	ackBlessings     bool
	grantedBlessings security.Blessings
	method, suffix   string
	tags             []*vdl.Value
	discharges       map[string]security.Discharge
	starttime        time.Time
	endStreamArgs    bool // are the stream args at EOF?
}

var (
	_ rpc.StreamServerCall = (*xflowServer)(nil)
	_ security.Call        = (*xflowServer)(nil)
)

func newXFlowServer(flow flow.Flow, server *xserver) (*xflowServer, error) {
	server.Lock()
	disp := server.disp
	server.Unlock()

	fs := &xflowServer{
		ctx:        server.ctx,
		server:     server,
		disp:       disp,
		flow:       flow,
		enc:        vom.NewEncoder(flow),
		dec:        vom.NewDecoder(flow),
		discharges: make(map[string]security.Discharge),
	}
	// TODO(toddw): Add logic to create separate type flows!
	return fs, nil
}

// authorizeVtrace works by simulating a call to __debug/vtrace.Trace.  That
// rpc is essentially equivalent in power to the data we are attempting to
// attach here.
func (fs *xflowServer) authorizeVtrace(ctx *context.T) error {
	// Set up a context as though we were calling __debug/vtrace.
	params := &security.CallParams{}
	params.Copy(fs)
	params.Method = "Trace"
	params.MethodTags = []*vdl.Value{vdl.ValueOf(access.Debug)}
	params.Suffix = "__debug/vtrace"

	var auth security.Authorizer
	if fs.server.dispReserved != nil {
		_, auth, _ = fs.server.dispReserved.Lookup(ctx, params.Suffix)
	}
	return authorize(fs.ctx, security.NewCall(params), auth)
}

func (fs *xflowServer) serve() error {
	defer fs.flow.Close()

	results, err := fs.processRequest()

	vtrace.GetSpan(fs.ctx).Finish()

	var traceResponse vtrace.Response
	// Check if the caller is permitted to view vtrace data.
	if fs.authorizeVtrace(fs.ctx) == nil {
		traceResponse = vtrace.GetResponse(fs.ctx)
	}

	// Respond to the client with the response header and positional results.
	response := rpc.Response{
		Error:            err,
		EndStreamResults: true,
		NumPosResults:    uint64(len(results)),
		TraceResponse:    traceResponse,
		AckBlessings:     fs.ackBlessings,
	}
	if err := fs.enc.Encode(response); err != nil {
		if err == io.EOF {
			return err
		}
		return verror.New(errResponseEncoding, fs.ctx, fs.LocalEndpoint().String(), fs.RemoteEndpoint().String(), err)
	}
	if response.Error != nil {
		return response.Error
	}
	for ix, res := range results {
		if err := fs.enc.Encode(res); err != nil {
			if err == io.EOF {
				return err
			}
			return verror.New(errResultEncoding, fs.ctx, ix, fmt.Sprintf("%T=%v", res, res), err)
		}
	}
	// TODO(ashankar): Should unread data from the flow be drained?
	//
	// Reason to do so:
	// The common stream.Flow implementation (v.io/x/ref/runtime/internal/rpc/stream/vc/reader.go)
	// uses iobuf.Slices backed by an iobuf.Pool. If the stream is not drained, these
	// slices will not be returned to the pool leading to possibly increased memory usage.
	//
	// Reason to not do so:
	// Draining here will conflict with any Reads on the flow in a separate goroutine
	// (for example, see TestStreamReadTerminatedByServer in full_test.go).
	//
	// For now, go with the reason to not do so as having unread data in the stream
	// should be a rare case.
	return nil
}

func (fs *xflowServer) readRPCRequest() (*rpc.Request, error) {
	// TODO(toddw): How do we set the initial timeout?  It might be shorter than
	// the timeout we set later, which we learn after we've decoded the request.
	/*
		// Set a default timeout before reading from the flow. Without this timeout,
		// a client that sends no request or a partial request will retain the flow
		// indefinitely (and lock up server resources).
		initTimer := newTimer(defaultCallTimeout)
		defer initTimer.Stop()
		fs.flow.SetDeadline(initTimer.C)
	*/

	// Decode the initial request.
	var req rpc.Request
	if err := fs.dec.Decode(&req); err != nil {
		return nil, verror.New(verror.ErrBadProtocol, fs.ctx, newErrBadRequest(fs.ctx, err))
	}
	return &req, nil
}

func (fs *xflowServer) processRequest() ([]interface{}, error) {
	fs.starttime = time.Now()
	req, err := fs.readRPCRequest()
	if err != nil {
		// We don't know what the rpc call was supposed to be, but we'll create
		// a placeholder span so we can capture annotations.
		fs.ctx, _ = vtrace.WithNewSpan(fs.ctx, fmt.Sprintf("\"%s\".UNKNOWN", fs.suffix))
		return nil, err
	}
	// We must call fs.drainDecoderArgs for any error that occurs
	// after this point, and before we actually decode the arguments.
	fs.method = req.Method
	fs.suffix = strings.TrimLeft(req.Suffix, "/")

	if req.Language != "" {
		fs.ctx = i18n.WithLangID(fs.ctx, i18n.LangID(req.Language))
	}

	// TODO(mattr): Currently this allows users to trigger trace collection
	// on the server even if they will not be allowed to collect the
	// results later.  This might be considered a DOS vector.
	spanName := fmt.Sprintf("\"%s\".%s", fs.suffix, fs.method)
	fs.ctx, _ = vtrace.WithContinuedTrace(fs.ctx, spanName, req.TraceRequest)

	var cancel context.CancelFunc
	if !req.Deadline.IsZero() {
		fs.ctx, cancel = context.WithDeadline(fs.ctx, req.Deadline.Time)
	} else {
		fs.ctx, cancel = context.WithCancel(fs.ctx)
	}
	fs.flow.SetContext(fs.ctx)
	// TODO(toddw): Explicitly cancel the context when the flow is done.
	_ = cancel

	// Initialize security: blessings, discharges, etc.
	if err := fs.initSecurity(req); err != nil {
		fs.drainDecoderArgs(int(req.NumPosArgs))
		return nil, err
	}
	// Lookup the invoker.
	invoker, auth, err := fs.lookup(fs.suffix, fs.method)
	if err != nil {
		fs.drainDecoderArgs(int(req.NumPosArgs))
		return nil, err
	}

	// Note that we strip the reserved prefix when calling the invoker so
	// that __Glob will call Glob.  Note that we've already assigned a
	// special invoker so that we never call the wrong method by mistake.
	strippedMethod := naming.StripReserved(fs.method)

	// Prepare invoker and decode args.
	numArgs := int(req.NumPosArgs)
	argptrs, tags, err := invoker.Prepare(fs.ctx, strippedMethod, numArgs)
	fs.tags = tags
	if err != nil {
		fs.drainDecoderArgs(numArgs)
		return nil, err
	}
	if called, want := req.NumPosArgs, uint64(len(argptrs)); called != want {
		fs.drainDecoderArgs(numArgs)
		return nil, newErrBadNumInputArgs(fs.ctx, fs.suffix, fs.method, called, want)
	}
	for ix, argptr := range argptrs {
		if err := fs.dec.Decode(argptr); err != nil {
			return nil, newErrBadInputArg(fs.ctx, fs.suffix, fs.method, uint64(ix), err)
		}
	}

	// Check application's authorization policy.
	if err := authorize(fs.ctx, fs, auth); err != nil {
		return nil, err
	}

	// Invoke the method.
	results, err := invoker.Invoke(fs.ctx, fs, strippedMethod, argptrs)
	fs.server.stats.record(fs.method, time.Since(fs.starttime))
	return results, err
}

// drainDecoderArgs drains the next n arguments encoded onto the flows decoder.
// This is needed to ensure that the client is able to encode all of its args
// before the server closes its flow. This guarantees that the client will
// consistently get the server's error response.
// TODO(suharshs): Figure out a better way to solve this race condition without
// unnecessarily reading all arguments.
func (fs *xflowServer) drainDecoderArgs(n int) error {
	for i := 0; i < n; i++ {
		if err := fs.dec.Ignore(); err != nil {
			return err
		}
	}
	return nil
}

// lookup returns the invoker and authorizer responsible for serving the given
// name and method.  The suffix is stripped of any leading slashes. If it begins
// with rpc.DebugKeyword, we use the internal debug dispatcher to look up the
// invoker. Otherwise, and we use the server's dispatcher. The suffix and method
// value may be modified to match the actual suffix and method to use.
func (fs *xflowServer) lookup(suffix string, method string) (rpc.Invoker, security.Authorizer, error) {
	if naming.IsReserved(method) {
		return reservedInvoker(fs.disp, fs.server.dispReserved), security.AllowEveryone(), nil
	}
	disp := fs.disp
	if naming.IsReserved(suffix) {
		disp = fs.server.dispReserved
	} else if fs.server.isLeaf && suffix != "" {
		innerErr := verror.New(errUnexpectedSuffix, fs.ctx, suffix)
		return nil, nil, verror.New(verror.ErrUnknownSuffix, fs.ctx, suffix, innerErr)
	}
	if disp != nil {
		obj, auth, err := disp.Lookup(fs.ctx, suffix)
		switch {
		case err != nil:
			return nil, nil, err
		case obj != nil:
			invoker, err := objectToInvoker(obj)
			if err != nil {
				return nil, nil, verror.New(verror.ErrInternal, fs.ctx, "invalid received object", err)
			}
			return invoker, auth, nil
		}
	}
	return nil, nil, verror.New(verror.ErrUnknownSuffix, fs.ctx, suffix)
}

func (fs *xflowServer) initSecurity(req *rpc.Request) error {
	// TODO(toddw): Do something with this.
	/*
		// LocalPrincipal is nil which means we are operating under
		// SecurityNone.
		if fs.LocalPrincipal() == nil {
			return nil
		}

		// If additional credentials are provided, make them available in the context
		// Detect unusable blessings now, rather then discovering they are unusable on
		// first use.
		//
		// TODO(ashankar,ataly): Potential confused deputy attack: The client provides
		// the server's identity as the blessing. Figure out what we want to do about
		// this - should servers be able to assume that a blessing is something that
		// does not have the authorizations that the server's own identity has?
		if got, want := req.GrantedBlessings.PublicKey(), fs.LocalPrincipal().PublicKey(); got != nil && !reflect.DeepEqual(got, want) {
			return verror.New(verror.ErrNoAccess, fs.ctx, fmt.Sprintf("blessing granted not bound to this server(%v vs %v)", got, want))
		}
		fs.grantedBlessings = req.GrantedBlessings

		var err error
		if fs.clientBlessings, err = serverDecodeBlessings(fs.flow.VCDataCache(), req.Blessings, fs.server.stats); err != nil {
			// When the server can't access the blessings cache, the client is not following
			// protocol, so the server closes the VCs corresponding to the client endpoint.
			// TODO(suharshs,toddw): Figure out a way to only shutdown the current VC, instead
			// of all VCs connected to the RemoteEndpoint.
			fs.server.streamMgr.ShutdownEndpoint(fs.RemoteEndpoint())
			return verror.New(verror.ErrBadProtocol, fs.ctx, newErrBadBlessingsCache(fs.ctx, err))
		}
		// Verify that the blessings sent by the client in the request have the same public
		// key as those sent by the client during VC establishment.
		if got, want := fs.clientBlessings.PublicKey(), fs.flow.RemoteBlessings().PublicKey(); got != nil && !reflect.DeepEqual(got, want) {
			return verror.New(verror.ErrNoAccess, fs.ctx, fmt.Sprintf("blessings sent with the request are bound to a different public key (%v) from the blessing used during VC establishment (%v)", got, want))
		}
		fs.ackBlessings = true

		for _, d := range req.Discharges {
			fs.discharges[d.ID()] = d
		}
	*/
	return nil
}

// Send implements the rpc.Stream method.
func (fs *xflowServer) Send(item interface{}) error {
	defer apilog.LogCallf(nil, "item=")(nil, "") // gologcop: DO NOT EDIT, MUST BE FIRST STATEMENT
	// The empty response header indicates what follows is a streaming result.
	if err := fs.enc.Encode(rpc.Response{}); err != nil {
		return err
	}
	return fs.enc.Encode(item)
}

// Recv implements the rpc.Stream method.
func (fs *xflowServer) Recv(itemptr interface{}) error {
	defer apilog.LogCallf(nil, "itemptr=")(nil, "") // gologcop: DO NOT EDIT, MUST BE FIRST STATEMENT
	var req rpc.Request
	if err := fs.dec.Decode(&req); err != nil {
		return err
	}
	if req.EndStreamArgs {
		fs.endStreamArgs = true
		return io.EOF
	}
	return fs.dec.Decode(itemptr)
}

// Implementations of rpc.ServerCall and security.Call methods.

func (fs *xflowServer) Security() security.Call {
	//nologcall
	return fs
}
func (fs *xflowServer) LocalDischarges() map[string]security.Discharge {
	//nologcall
	return fs.flow.LocalDischarges()
}
func (fs *xflowServer) RemoteDischarges() map[string]security.Discharge {
	//nologcall
	return fs.flow.RemoteDischarges()
}
func (fs *xflowServer) Server() rpc.XServer {
	//nologcall
	return nil // TODO(toddw): Change return to rpc.XServer
}
func (fs *xflowServer) Timestamp() time.Time {
	//nologcall
	return fs.starttime
}
func (fs *xflowServer) Method() string {
	//nologcall
	return fs.method
}
func (fs *xflowServer) MethodTags() []*vdl.Value {
	//nologcall
	return fs.tags
}
func (fs *xflowServer) Suffix() string {
	//nologcall
	return fs.suffix
}
func (fs *xflowServer) LocalPrincipal() security.Principal {
	//nologcall
	return fs.server.principal
}
func (fs *xflowServer) LocalBlessings() security.Blessings {
	//nologcall
	return fs.flow.LocalBlessings()
}
func (fs *xflowServer) RemoteBlessings() security.Blessings {
	//nologcall
	return fs.flow.RemoteBlessings()
}
func (fs *xflowServer) GrantedBlessings() security.Blessings {
	//nologcall
	return fs.grantedBlessings
}
func (fs *xflowServer) LocalEndpoint() naming.Endpoint {
	//nologcall
	return fs.flow.Conn().LocalEndpoint()
}
func (fs *xflowServer) RemoteEndpoint() naming.Endpoint {
	//nologcall
	return fs.flow.Conn().RemoteEndpoint()
}
