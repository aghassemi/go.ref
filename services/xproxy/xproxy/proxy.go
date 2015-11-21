// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package xproxy

import (
	"io"
	"sync"
	"time"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/flow"
	"v.io/v23/flow/message"
	"v.io/v23/naming"
	"v.io/v23/security"

	"v.io/x/ref/lib/publisher"
)

const (
	reconnectDelay = 50 * time.Millisecond
	maxBackoff     = time.Minute
	bidiProtocol   = "bidi"
)

type proxy struct {
	m      flow.Manager
	pub    publisher.Publisher
	closed chan struct{}
	auth   security.Authorizer
	wg     sync.WaitGroup

	mu                 sync.Mutex
	listeningEndpoints map[string]naming.Endpoint   // keyed by endpoint string
	proxyEndpoints     map[string][]naming.Endpoint // keyed by proxy address
	proxiedProxies     []flow.Flow                  // flows of proxies that are listening through us.
	proxiedServers     []flow.Flow                  // flows of servers that are listening through us.
	closing            bool
}

func New(ctx *context.T, name string, auth security.Authorizer) (*proxy, error) {
	mgr, err := v23.NewFlowManager(ctx, 0)
	if err != nil {
		return nil, err
	}
	p := &proxy{
		m:                  mgr,
		auth:               auth,
		proxyEndpoints:     make(map[string][]naming.Endpoint),
		listeningEndpoints: make(map[string]naming.Endpoint),
		pub:                publisher.New(ctx, v23.GetNamespace(ctx), time.Minute),
		closed:             make(chan struct{}),
	}
	if p.auth == nil {
		p.auth = security.DefaultAuthorizer()
	}
	if len(name) > 0 {
		p.pub.AddName(name, false, true)
	}
	lspec := v23.GetListenSpec(ctx)
	if len(lspec.Proxy) > 0 {
		p.wg.Add(1)
		go p.connectToProxy(ctx, lspec.Proxy)
	}
	for _, addr := range lspec.Addrs {
		if err := p.m.Listen(ctx, addr.Protocol, addr.Address); err != nil {
			return nil, err
		}
	}
	leps, changed := p.m.ListeningEndpoints()
	p.updateListeningEndpoints(ctx, leps)
	p.wg.Add(2)
	go p.updateEndpointsLoop(ctx, changed)
	go p.listenLoop(ctx)
	go func() {
		<-ctx.Done()
		p.mu.Lock()
		p.closing = true
		p.mu.Unlock()
		p.pub.Stop()
		p.pub.WaitForStop()
		p.wg.Wait()
		<-p.m.Closed()
		close(p.closed)
	}()
	return p, nil
}

func (p *proxy) Closed() <-chan struct{} {
	return p.closed
}

func (p *proxy) updateEndpointsLoop(ctx *context.T, changed <-chan struct{}) {
	defer p.wg.Done()
	var leps []naming.Endpoint
	for changed != nil {
		<-changed
		leps, changed = p.m.ListeningEndpoints()
		p.updateListeningEndpoints(ctx, leps)
	}
}

func (p *proxy) updateListeningEndpoints(ctx *context.T, leps []naming.Endpoint) {
	p.mu.Lock()
	endpoints := make(map[string]naming.Endpoint)
	for _, ep := range leps {
		endpoints[ep.String()] = ep
	}
	rmEps := setDiff(p.listeningEndpoints, endpoints)
	addEps := setDiff(endpoints, p.listeningEndpoints)
	for k := range rmEps {
		delete(p.listeningEndpoints, k)
	}
	for k, ep := range addEps {
		p.listeningEndpoints[k] = ep
	}

	p.sendUpdatesLocked(ctx)
	p.mu.Unlock()

	for k, ep := range rmEps {
		if ep.Addr().Network() != bidiProtocol {
			p.pub.RemoveServer(k)
		}
	}
	for k, ep := range addEps {
		if ep.Addr().Network() != bidiProtocol {
			p.pub.AddServer(k)
		}
	}
}

func (p *proxy) sendUpdatesLocked(ctx *context.T) {
	// Send updates to the proxies and servers that are listening through us.
	// TODO(suharshs): Should we send these in parallel?
	i := 0
	for _, f := range p.proxiedProxies {
		if !isClosed(f) {
			if err := p.replyToProxyLocked(ctx, f); err != nil {
				ctx.Error(err)
				continue
			}
			p.proxiedProxies[i] = f
			i++
		}
	}
	p.proxiedProxies = p.proxiedProxies[:i]
	i = 0
	for _, f := range p.proxiedServers {
		if !isClosed(f) {
			if err := p.replyToServerLocked(ctx, f); err != nil {
				ctx.Error(err)
				continue
			}
			p.proxiedServers[i] = f
			i++
		}
	}
	p.proxiedServers = p.proxiedServers[:i]
}

func isClosed(f flow.Flow) bool {
	select {
	case <-f.Closed():
		return true
	default:
	}
	return false
}

// setDiff returns the endpoints in a that are not in b.
func setDiff(a, b map[string]naming.Endpoint) map[string]naming.Endpoint {
	ret := make(map[string]naming.Endpoint)
	for k, ep := range a {
		if _, ok := b[k]; !ok {
			ret[k] = ep
		}
	}
	return ret
}

func (p *proxy) ListeningEndpoints() []naming.Endpoint {
	// TODO(suharshs): Return changed channel here as well.
	eps, _ := p.m.ListeningEndpoints()
	return eps
}

func (p *proxy) MultipleProxyEndpoints() []naming.Endpoint {
	var eps []naming.Endpoint
	p.mu.Lock()
	for _, v := range p.proxyEndpoints {
		eps = append(eps, v...)
	}
	p.mu.Unlock()
	return eps
}

func (p *proxy) listenLoop(ctx *context.T) {
	defer p.wg.Done()
	for {
		f, err := p.m.Accept(ctx)
		if err != nil {
			ctx.Infof("p.m.Accept failed: %v", err)
			break
		}
		msg, err := readMessage(ctx, f)
		if err != nil {
			ctx.Errorf("reading message failed: %v", err)
		}
		switch m := msg.(type) {
		case *message.Setup:
			err = p.startRouting(ctx, f, m)
		case *message.MultiProxyRequest:
			p.mu.Lock()
			err = p.replyToProxyLocked(ctx, f)
			if err == nil {
				p.proxiedProxies = append(p.proxiedProxies, f)
			}
			p.mu.Unlock()
		case *message.ProxyServerRequest:
			p.mu.Lock()
			err = p.replyToServerLocked(ctx, f)
			if err == nil {
				p.proxiedServers = append(p.proxiedServers, f)
			}
			p.mu.Unlock()
		default:
			continue
		}
		if err != nil {
			ctx.Errorf("failed to handle incoming connection: %v", err)
		}
	}
}

func (p *proxy) startRouting(ctx *context.T, f flow.Flow, m *message.Setup) error {
	fout, err := p.dialNextHop(ctx, f, m)
	if err != nil {
		f.Close()
		return err
	}
	p.mu.Lock()
	if p.closing {
		p.mu.Unlock()
		return NewErrProxyAlreadyClosed(ctx)
	}
	p.wg.Add(2)
	p.mu.Unlock()
	go p.forwardLoop(ctx, f, fout)
	go p.forwardLoop(ctx, fout, f)
	return nil
}

func (p *proxy) forwardLoop(ctx *context.T, fin, fout flow.Flow) {
	defer p.wg.Done()
	if err := framedCopy(fin, fout); err != nil {
		ctx.Errorf("Error forwarding: %v", err)
	}
	fin.Close()
	fout.Close()
}

func framedCopy(fin, fout flow.Flow) error {
	for {
		msg, err := fin.ReadMsg()
		if err != nil {
			if err == io.EOF {
				_, err = fout.WriteMsg(msg)
			}
			return err
		}
		if _, err = fout.WriteMsg(msg); err != nil {
			return err
		}
	}
}

func (p *proxy) dialNextHop(ctx *context.T, f flow.Flow, m *message.Setup) (flow.Flow, error) {
	var (
		rid naming.RoutingID
		ep  naming.Endpoint
		err error
	)
	if ep, err = setBidiProtocol(m.PeerRemoteEndpoint); err != nil {
		return nil, err
	}
	if routes := ep.Routes(); len(routes) > 0 {
		if err := rid.FromString(routes[0]); err != nil {
			return nil, err
		}
		// Make an endpoint with the correct routingID to dial out. All other fields
		// do not matter.
		// TODO(suharshs): Make sure that the routingID from the route belongs to a
		// connection that is stored in the manager's cache. (i.e. a Server has connected
		// with the routingID before)
		if ep, err = setEndpointRoutingID(ep, rid); err != nil {
			return nil, err
		}
		// Remove the read route from the setup message endpoint.
		if m.PeerRemoteEndpoint, err = setEndpointRoutes(m.PeerRemoteEndpoint, routes[1:]); err != nil {
			return nil, err
		}
	}
	fout, err := p.m.Dial(ctx, ep, proxyAuthorizer{}, 0)
	if err != nil {
		return nil, err
	}
	if err := p.authorizeFlow(ctx, fout); err != nil {
		return nil, err
	}
	// Write the setup message back onto the flow for the next hop to read.
	return fout, writeMessage(ctx, m, fout)
}

func (p *proxy) replyToServerLocked(ctx *context.T, f flow.Flow) error {
	if err := p.authorizeFlow(ctx, f); err != nil {
		// TODO(suharshs): should we return the err to the server in the ProxyResponse message?
		f.Close()
		return err
	}
	rid := f.RemoteEndpoint().RoutingID()
	eps, err := p.returnEndpointsLocked(ctx, rid, "")
	if err != nil {
		return err
	}
	return writeMessage(ctx, &message.ProxyResponse{Endpoints: eps}, f)
}

func (p *proxy) authorizeFlow(ctx *context.T, f flow.Flow) error {
	call := security.NewCall(&security.CallParams{
		LocalPrincipal:   v23.GetPrincipal(ctx),
		LocalBlessings:   f.LocalBlessings(),
		RemoteBlessings:  f.RemoteBlessings(),
		LocalEndpoint:    f.LocalEndpoint(),
		RemoteEndpoint:   f.RemoteEndpoint(),
		RemoteDischarges: f.RemoteDischarges(),
	})
	return p.auth.Authorize(ctx, call)
}

func (p *proxy) replyToProxyLocked(ctx *context.T, f flow.Flow) error {
	// Add the routing id of the incoming proxy to the routes. The routing id of the
	// returned endpoint doesn't matter because it will eventually be replaced
	// by a server's rid by some later proxy.
	// TODO(suharshs): Use a local route instead of this global routingID.
	rid := f.RemoteEndpoint().RoutingID()
	eps, err := p.returnEndpointsLocked(ctx, naming.NullRoutingID, rid.String())
	if err != nil {
		return err
	}
	return writeMessage(ctx, &message.ProxyResponse{Endpoints: eps}, f)
}

func (p *proxy) returnEndpointsLocked(ctx *context.T, rid naming.RoutingID, route string) ([]naming.Endpoint, error) {
	eps, _ := p.m.ListeningEndpoints()
	for _, peps := range p.proxyEndpoints {
		eps = append(eps, peps...)
	}
	if len(eps) == 0 {
		return nil, NewErrNotListening(ctx)
	}
	for idx, ep := range eps {
		var err error
		if rid != naming.NullRoutingID {
			ep, err = setEndpointRoutingID(ep, rid)
			if err != nil {
				return nil, err
			}
		}
		if len(route) > 0 {
			var cp []string
			cp = append(cp, ep.Routes()...)
			cp = append(cp, route)
			ep, err = setEndpointRoutes(ep, cp)
			if err != nil {
				return nil, err
			}
		}
		eps[idx] = ep
	}
	return eps, nil
}

func (p *proxy) connectToProxy(ctx *context.T, name string) {
	defer p.wg.Done()
	for delay := reconnectDelay; ; delay = nextDelay(delay) {
		time.Sleep(delay - reconnectDelay)
		select {
		case <-ctx.Done():
			return
		default:
		}
		eps, err := resolveToEndpoint(ctx, name)
		if err != nil {
			ctx.Error(err)
			continue
		}
		if err = p.tryProxyEndpoints(ctx, name, eps); err != nil {
			ctx.Error(err)
		} else {
			delay = reconnectDelay / 2
		}
	}
}

func (p *proxy) tryProxyEndpoints(ctx *context.T, name string, eps []naming.Endpoint) error {
	var lastErr error
	for _, ep := range eps {
		if lastErr = p.proxyListen(ctx, name, ep); lastErr == nil {
			break
		}
	}
	return lastErr
}

func (p *proxy) proxyListen(ctx *context.T, name string, ep naming.Endpoint) error {
	defer p.updateProxyEndpoints(ctx, name, nil)
	f, err := p.m.Dial(ctx, ep, proxyAuthorizer{}, 0)
	if err != nil {
		return err
	}
	// Send a byte telling the acceptor that we are a proxy.
	if err := writeMessage(ctx, &message.MultiProxyRequest{}, f); err != nil {
		return err
	}
	for {
		// we keep reading updates until we encounter an error, usually because the
		// flow has been closed.
		eps, err := readProxyResponse(ctx, f)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		p.updateProxyEndpoints(ctx, name, eps)
	}
}

func nextDelay(delay time.Duration) time.Duration {
	delay *= 2
	if delay > maxBackoff {
		delay = maxBackoff
	}
	return delay
}

func (p *proxy) updateProxyEndpoints(ctx *context.T, address string, eps []naming.Endpoint) {
	p.mu.Lock()
	if len(eps) > 0 {
		p.proxyEndpoints[address] = eps
	} else {
		delete(p.proxyEndpoints, address)
	}
	p.sendUpdatesLocked(ctx)
	p.mu.Unlock()
}

func resolveToEndpoint(ctx *context.T, name string) ([]naming.Endpoint, error) {
	ns := v23.GetNamespace(ctx)
	ns.FlushCacheEntry(ctx, name)
	resolved, err := ns.Resolve(ctx, name)
	if err != nil {
		return nil, err
	}
	var eps []naming.Endpoint
	for _, n := range resolved.Names() {
		address, suffix := naming.SplitAddressName(n)
		if len(suffix) > 0 {
			continue
		}
		if ep, err := v23.NewEndpoint(address); err == nil {
			eps = append(eps, ep)
			continue
		}
	}
	if len(eps) > 0 {
		return eps, nil
	}
	return nil, NewErrFailedToResolveToEndpoint(ctx, name)
}
