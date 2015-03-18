package manager

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"

	"v.io/x/ref/profiles/internal/lib/upcqueue"
	inaming "v.io/x/ref/profiles/internal/naming"
	"v.io/x/ref/profiles/internal/rpc/stream/proxy"
	"v.io/x/ref/profiles/internal/rpc/stream/vif"

	"v.io/v23/naming"
	"v.io/v23/security"
	"v.io/v23/verror"
	"v.io/v23/vom"
	"v.io/x/lib/vlog"
	"v.io/x/ref/profiles/internal/rpc/stream"
)

var errListenerIsClosed = errors.New("Listener has been Closed")

// listener extends stream.Listener with a DebugString method.
type listener interface {
	stream.Listener
	DebugString() string
}

// netListener implements the listener interface by accepting flows (and VCs)
// over network connections accepted on an underlying net.Listener.
type netListener struct {
	q       *upcqueue.T
	netLn   net.Listener
	manager *manager
	vifs    *vif.Set

	netLoop  sync.WaitGroup
	vifLoops sync.WaitGroup
}

var _ stream.Listener = (*netListener)(nil)

// proxyListener implements the listener interface by connecting to a remote
// proxy (typically used to "listen" across network domains).
type proxyListener struct {
	q         *upcqueue.T
	proxyEP   naming.Endpoint
	manager   *manager
	principal security.Principal
	blessings security.Blessings
	opts      []stream.ListenerOpt
}

var _ stream.Listener = (*proxyListener)(nil)

func newNetListener(m *manager, netLn net.Listener, principal security.Principal, blessings security.Blessings, opts []stream.ListenerOpt) listener {
	ln := &netListener{
		q:       upcqueue.New(),
		manager: m,
		netLn:   netLn,
		vifs:    vif.NewSet(),
	}
	ln.netLoop.Add(1)
	go ln.netAcceptLoop(principal, blessings, opts)
	return ln
}

func (ln *netListener) netAcceptLoop(principal security.Principal, blessings security.Blessings, listenerOpts []stream.ListenerOpt) {
	defer ln.netLoop.Done()
	for {
		conn, err := ln.netLn.Accept()
		if err != nil {
			vlog.VI(1).Infof("Exiting netAcceptLoop: net.Listener.Accept() failed on %v with %v", ln.netLn, err)
			return
		}
		vlog.VI(1).Infof("New net.Conn accepted from %s (local address: %s)", conn.RemoteAddr(), conn.LocalAddr())
		vf, err := vif.InternalNewAcceptedVIF(conn, ln.manager.rid, principal, blessings, nil, listenerOpts...)
		if err != nil {
			vlog.Infof("Shutting down conn from %s (local address: %s) as a VIF could not be created: %v", conn.RemoteAddr(), conn.LocalAddr(), err)
			conn.Close()
			continue
		}
		ln.vifs.Insert(vf)
		ln.manager.vifs.Insert(vf)
		ln.vifLoops.Add(1)
		go ln.vifLoop(vf)
	}
}

func (ln *netListener) vifLoop(vf *vif.VIF) {
	vifLoop(vf, ln.q)
	ln.vifLoops.Done()
}

func (ln *netListener) Accept() (stream.Flow, error) {
	item, err := ln.q.Get(nil)
	switch {
	case err == upcqueue.ErrQueueIsClosed:
		return nil, errListenerIsClosed
	case err != nil:
		return nil, fmt.Errorf("Accept failed: %v", err)
	default:
		return item.(vif.ConnectorAndFlow).Flow, nil
	}
}

func (ln *netListener) Close() error {
	closeNetListener(ln.netLn)
	ln.netLoop.Wait()
	for _, vif := range ln.vifs.List() {
		// NOTE(caprita): We do not actually Close down the vifs, as
		// that would require knowing when all outstanding requests are
		// finished.  For now, do not worry about it, since we expect
		// shut down to immediately precede process exit.
		vif.StopAccepting()
	}
	ln.q.Shutdown()
	ln.manager.removeListener(ln)
	ln.vifLoops.Wait()
	vlog.VI(3).Infof("Closed stream.Listener %s", ln)
	return nil
}

func (ln *netListener) String() string {
	return fmt.Sprintf("%T: (%v, %v)", ln, ln.netLn.Addr().Network(), ln.netLn.Addr())
}

func (ln *netListener) DebugString() string {
	ret := []string{
		fmt.Sprintf("stream.Listener: net.Listener on (%q, %q)", ln.netLn.Addr().Network(), ln.netLn.Addr()),
	}
	if vifs := ln.vifs.List(); len(vifs) > 0 {
		ret = append(ret, fmt.Sprintf("===Accepted VIFs(%d)===", len(vifs)))
		for ix, vif := range vifs {
			ret = append(ret, fmt.Sprintf("%4d) %v", ix, vif))
		}
	}
	return strings.Join(ret, "\n")
}

func newProxyListener(m *manager, proxyEP naming.Endpoint, principal security.Principal, opts []stream.ListenerOpt) (listener, *inaming.Endpoint, error) {
	ln := &proxyListener{
		q:         upcqueue.New(),
		proxyEP:   proxyEP,
		manager:   m,
		opts:      opts,
		principal: principal,
	}
	vf, ep, err := ln.connect()
	if err != nil {
		return nil, nil, err
	}
	go vifLoop(vf, ln.q)
	return ln, ep, nil
}

func (ln *proxyListener) connect() (*vif.VIF, *inaming.Endpoint, error) {
	vlog.VI(1).Infof("Connecting to proxy at %v", ln.proxyEP)
	// Requires dialing a VC to the proxy, need to extract options from ln.opts to do so.
	var dialOpts []stream.VCOpt
	for _, opt := range ln.opts {
		if dopt, ok := opt.(stream.VCOpt); ok {
			dialOpts = append(dialOpts, dopt)
		}
	}
	// TODO(cnicolaou, ashankar): probably want to set a timeout here. (is this covered by opts?)
	vf, err := ln.manager.FindOrDialVIF(ln.proxyEP, ln.principal, dialOpts...)
	if err != nil {
		return nil, nil, err
	}
	if err := vf.StartAccepting(ln.opts...); err != nil {
		return nil, nil, fmt.Errorf("already connected to proxy and accepting connections? VIF: %v, StartAccepting error: %v", vf, err)
	}
	// Proxy protocol: See v.io/x/ref/profiles/internal/rpc/stream/proxy/protocol.vdl
	vc, err := vf.Dial(ln.proxyEP, ln.principal, dialOpts...)
	if err != nil {
		vf.StopAccepting()
		if verror.ErrorID(err) == verror.ErrAborted.ID {
			ln.manager.vifs.Delete(vf)
		}
		return nil, nil, fmt.Errorf("VC establishment with proxy failed: %v", err)
	}
	flow, err := vc.Connect()
	if err != nil {
		vf.StopAccepting()
		return nil, nil, fmt.Errorf("unable to create liveness check flow to proxy: %v", err)
	}
	var request proxy.Request
	var response proxy.Response
	enc, err := vom.NewEncoder(flow)
	if err != nil {
		flow.Close()
		vf.StopAccepting()
		return nil, nil, fmt.Errorf("failed to create new Encoder: %v", err)
	}
	if err := enc.Encode(request); err != nil {
		flow.Close()
		vf.StopAccepting()
		return nil, nil, fmt.Errorf("failed to encode request to proxy: %v", err)
	}
	dec, err := vom.NewDecoder(flow)
	if err != nil {
		flow.Close()
		vf.StopAccepting()
		return nil, nil, fmt.Errorf("failed to create new Decoder: %v", err)
	}
	if err := dec.Decode(&response); err != nil {
		flow.Close()
		vf.StopAccepting()
		return nil, nil, fmt.Errorf("failed to decode response from proxy: %v", err)
	}
	if response.Error != nil {
		flow.Close()
		vf.StopAccepting()
		return nil, nil, fmt.Errorf("proxy error: %v", response.Error)
	}
	ep, err := inaming.NewEndpoint(response.Endpoint)
	if err != nil {
		flow.Close()
		vf.StopAccepting()
		return nil, nil, fmt.Errorf("proxy returned invalid endpoint(%v): %v", response.Endpoint, err)
	}
	go func(vf *vif.VIF, flow stream.Flow, q *upcqueue.T) {
		<-flow.Closed()
		vf.StopAccepting()
		q.Close()
	}(vf, flow, ln.q)
	return vf, ep, nil
}

func (ln *proxyListener) Accept() (stream.Flow, error) {
	item, err := ln.q.Get(nil)
	switch {
	case err == upcqueue.ErrQueueIsClosed:
		return nil, errListenerIsClosed
	case err != nil:
		return nil, fmt.Errorf("Accept failed: %v", err)
	default:
		return item.(vif.ConnectorAndFlow).Flow, nil
	}
}

func (ln *proxyListener) Close() error {
	ln.q.Shutdown()
	ln.manager.removeListener(ln)
	return nil
}

func (ln *proxyListener) String() string {
	return ln.DebugString()
}

func (ln *proxyListener) DebugString() string {
	return fmt.Sprintf("stream.Listener: PROXY:%v RoutingID:%v", ln.proxyEP, ln.manager.rid)
}

func vifLoop(vf *vif.VIF, q *upcqueue.T) {
	for {
		cAndf, err := vf.Accept()
		switch {
		case err != nil:
			vlog.VI(2).Infof("Shutting down listener on VIF %v: %v", vf, err)
			return
		case cAndf.Flow == nil:
			vlog.VI(1).Infof("New VC %v on VIF %v", cAndf.Connector, vf)
		default:
			if err := q.Put(cAndf); err != nil {
				vlog.VI(1).Infof("Closing new flow on VC %v (VIF %v) as Put failed in vifLoop: %v", cAndf.Connector, vf, err)
				cAndf.Flow.Close()
			}
		}
	}
}
