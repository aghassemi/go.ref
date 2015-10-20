// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package test

import (
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/options"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/v23/verror"
	"v.io/x/ref"
	"v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/runtime/internal/flow/conn"
	inaming "v.io/x/ref/runtime/internal/naming"
	"v.io/x/ref/runtime/internal/rpc/stream/vc"
	"v.io/x/ref/test"
)

type canceld struct {
	name     string
	child    string
	started  chan struct{}
	canceled chan struct{}
}

func (c *canceld) Run(ctx *context.T, _ rpc.ServerCall) error {
	close(c.started)
	client := v23.GetClient(ctx)
	var done chan struct{}
	if c.child != "" {
		done = make(chan struct{})
		go func() {
			client.Call(ctx, c.child, "Run", nil, nil)
			close(done)
		}()
	}
	<-ctx.Done()
	if done != nil {
		<-done
	}
	close(c.canceled)
	return nil
}

func makeCanceld(ctx *context.T, name, child string) (*canceld, error) {
	c := &canceld{
		name:     name,
		child:    child,
		started:  make(chan struct{}, 0),
		canceled: make(chan struct{}, 0),
	}
	_, _, err := v23.WithNewServer(ctx, name, c, security.AllowEveryone())
	if err != nil {
		return nil, err
	}
	return c, nil
}

// TestCancellationPropagation tests that cancellation propogates along an
// RPC call chain without user intervention.
func TestCancellationPropagation(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	c1, err := makeCanceld(ctx, "c1", "c2")
	if err != nil {
		t.Fatalf("Can't start server:", err, verror.DebugString(err))
	}
	c2, err := makeCanceld(ctx, "c2", "")
	if err != nil {
		t.Fatalf("Can't start server:", err)
	}

	ctx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		v23.GetClient(ctx).Call(ctx, "c1", "Run", nil, nil)
		close(done)
	}()

	<-c1.started
	<-c2.started
	cancel()
	<-c1.canceled
	<-c2.canceled
	<-done
}

type cancelTestServer struct {
	started   chan struct{}
	cancelled chan struct{}
	t         *testing.T
}

func newCancelTestServer(t *testing.T) *cancelTestServer {
	return &cancelTestServer{
		started:   make(chan struct{}),
		cancelled: make(chan struct{}),
		t:         t,
	}
}

func (s *cancelTestServer) CancelStreamReader(ctx *context.T, call rpc.StreamServerCall) error {
	close(s.started)
	var b []byte
	if err := call.Recv(&b); err != io.EOF {
		s.t.Errorf("Got error %v, want io.EOF", err)
	}
	<-ctx.Done()
	close(s.cancelled)
	return nil
}

// CancelStreamIgnorer doesn't read from it's input stream so all it's
// buffers fill.  The intention is to show that call.Done() is closed
// even when the stream is stalled.
func (s *cancelTestServer) CancelStreamIgnorer(ctx *context.T, _ rpc.StreamServerCall) error {
	close(s.started)
	<-ctx.Done()
	close(s.cancelled)
	return nil
}

func waitForCancel(t *testing.T, ts *cancelTestServer, cancel context.CancelFunc) {
	<-ts.started
	cancel()
	<-ts.cancelled
}

// TestCancel tests cancellation while the server is reading from a stream.
func TestCancel(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()
	var (
		sctx = withPrincipal(t, ctx, "server")
		cctx = withPrincipal(t, ctx, "client")
		ts   = newCancelTestServer(t)
	)
	_, _, err := v23.WithNewServer(sctx, "cancel", ts, security.AllowEveryone())
	if err != nil {
		t.Fatal(err)
	}
	cctx, cancel := context.WithCancel(cctx)
	done := make(chan struct{})
	go func() {
		v23.GetClient(cctx).Call(cctx, "cancel", "CancelStreamReader", nil, nil)
		close(done)
	}()
	waitForCancel(t, ts, cancel)
	<-done
}

// TestCancelWithFullBuffers tests that even if the writer has filled the buffers and
// the server is not reading that the cancel message gets through.
func TestCancelWithFullBuffers(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()
	var (
		sctx = withPrincipal(t, ctx, "server")
		cctx = withPrincipal(t, ctx, "client")
		ts   = newCancelTestServer(t)
	)
	_, _, err := v23.WithNewServer(sctx, "cancel", ts, security.AllowEveryone())
	if err != nil {
		t.Fatal(err)
	}
	cctx, cancel := context.WithCancel(cctx)
	call, err := v23.GetClient(cctx).StartCall(cctx, "cancel", "CancelStreamIgnorer", nil)
	if err != nil {
		t.Fatalf("Start call failed: %v", err)
	}

	// Fill up all the write buffers to ensure that cancelling works even when the stream
	// is blocked.
	if ref.RPCTransitionState() >= ref.XServers {
		call.Send(make([]byte, conn.DefaultBytesBufferedPerFlow-2048))
	} else {
		call.Send(make([]byte, vc.MaxSharedBytes))
		call.Send(make([]byte, vc.DefaultBytesBufferedPerFlow))
	}
	done := make(chan struct{})
	go func() {
		call.Finish()
		close(done)
	}()

	waitForCancel(t, ts, cancel)
	<-done
}

type channelTestServer struct {
	waiting  chan struct{}
	canceled chan struct{}
}

func (s *channelTestServer) Run(ctx *context.T, call rpc.ServerCall, wait time.Duration) error {
	time.Sleep(wait)
	return nil
}

func (s *channelTestServer) WaitForCancel(ctx *context.T, call rpc.ServerCall) error {
	close(s.waiting)
	<-ctx.Done()
	close(s.canceled)
	return nil
}

type disConn struct {
	net.Conn
	mu                  sync.Mutex
	stopread, stopwrite bool
}

func (p *disConn) stop(read, write bool) {
	p.mu.Lock()
	p.stopread = read
	p.stopwrite = write
	p.mu.Unlock()
}
func (p *disConn) Write(b []byte) (int, error) {
	p.mu.Lock()
	stopwrite := p.stopwrite
	p.mu.Unlock()
	if stopwrite {
		return len(b), nil
	}
	return p.Conn.Write(b)
}
func (p *disConn) Read(b []byte) (int, error) {
	for {
		n, err := p.Conn.Read(b)
		p.mu.Lock()
		stopread := p.stopread
		p.mu.Unlock()
		if err != nil || !stopread {
			return n, err
		}
	}
}

func registerDisProtocol(wrap string, conns chan *disConn) {
	dial, resolve, listen, protonames := rpc.RegisteredProtocol(wrap)
	rpc.RegisterProtocol("dis", func(ctx *context.T, p, a string, t time.Duration) (net.Conn, error) {
		conn, err := dial(ctx, protonames[0], a, t)
		if err == nil {
			dc := &disConn{Conn: conn}
			conns <- dc
			conn = dc
		}
		return conn, err
	}, func(ctx *context.T, protocol, address string) (string, string, error) {
		_, a, err := resolve(ctx, protonames[0], address)
		return "dis", a, err
	}, func(ctx *context.T, protocol, address string) (net.Listener, error) {
		return listen(ctx, protonames[0], address)
	})
}

func findEndpoint(ctx *context.T, s rpc.Server) naming.Endpoint {
	if status := s.Status(); len(status.Endpoints) > 0 {
		return status.Endpoints[0]
	} else {
		timer := time.NewTicker(10 * time.Millisecond)
		defer timer.Stop()
		for _ = range timer.C {
			if status = s.Status(); len(status.Proxies) > 0 {
				return status.Proxies[0].Endpoint
			}
		}
	}
	return nil // Unreachable
}

func testChannelTimeout(t *testing.T, ctx *context.T) {
	_, s, err := v23.WithNewServer(ctx, "", &channelTestServer{}, security.AllowEveryone())
	if err != nil {
		t.Fatal(err)
	}
	ep := findEndpoint(ctx, s)
	conns := make(chan *disConn, 1)
	registerDisProtocol(ep.Addr().Network(), conns)

	iep := ep.(*inaming.Endpoint)
	iep.Protocol = "dis"

	// Long calls don't cause the timeout, the control stream is still operating.
	err = v23.GetClient(ctx).Call(ctx, iep.Name(), "Run", []interface{}{2 * time.Second},
		nil, options.ChannelTimeout(500*time.Millisecond))
	if err != nil {
		t.Errorf("got %v want nil", err)
	}
	(<-conns).stop(true, true)
	err = v23.GetClient(ctx).Call(ctx, iep.Name(), "Run", []interface{}{time.Duration(0)},
		nil, options.ChannelTimeout(100*time.Millisecond))
	if err == nil {
		t.Errorf("wanted non-nil error", err)
	}
}

func TestChannelTimeout(t *testing.T) {
	if ref.RPCTransitionState() >= ref.XServers {
		t.Skip("The new RPC system does not yet support channel timeouts")
	}
	ctx, shutdown := test.V23Init()
	defer shutdown()
	testChannelTimeout(t, ctx)
}

func TestChannelTimeout_Proxy(t *testing.T) {
	if ref.RPCTransitionState() >= ref.XServers {
		t.Skip("The new RPC system does not yet support channel timeouts")
	}
	ctx, shutdown := test.V23Init()
	defer shutdown()

	ls := v23.GetListenSpec(ctx)
	pshutdown, pendpoint, err := generic.NewProxy(ctx, ls, security.AllowEveryone(), "")
	if err != nil {
		t.Fatal(err)
	}
	defer pshutdown()
	ls.Addrs = nil
	ls.Proxy = pendpoint.Name()
	testChannelTimeout(t, v23.WithListenSpec(ctx, ls))
}

func testChannelTimeOut_Server(t *testing.T, ctx *context.T) {
	cts := &channelTestServer{
		canceled: make(chan struct{}),
		waiting:  make(chan struct{}),
	}
	_, s, err := v23.WithNewServer(ctx, "", cts, security.AllowEveryone(),
		options.ChannelTimeout(500*time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	ep := findEndpoint(ctx, s)
	conns := make(chan *disConn, 1)
	registerDisProtocol(ep.Addr().Network(), conns)

	iep := ep.(*inaming.Endpoint)
	iep.Protocol = "dis"

	// Long calls don't cause the timeout, the control stream is still operating.
	err = v23.GetClient(ctx).Call(ctx, iep.Name(), "Run", []interface{}{2 * time.Second},
		nil)
	if err != nil {
		t.Errorf("got %v want nil", err)
	}
	// When the server closes the VC in response to the channel timeout the server
	// call will see a cancellation.  We do a call and wait for that server-side
	// cancellation.  Then we cancel the client call just to clean up.
	cctx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		v23.GetClient(cctx).Call(cctx, iep.Name(), "WaitForCancel", nil, nil)
		close(done)
	}()
	<-cts.waiting
	(<-conns).stop(true, true)
	<-cts.canceled
	cancel()
	<-done
}

func TestChannelTimeout_Server(t *testing.T) {
	if ref.RPCTransitionState() >= ref.XServers {
		t.Skip("The new RPC system does not yet support channel timeouts")
	}
	ctx, shutdown := test.V23Init()
	defer shutdown()
	testChannelTimeOut_Server(t, ctx)
}

func TestChannelTimeout_ServerProxy(t *testing.T) {
	if ref.RPCTransitionState() >= ref.XServers {
		t.Skip("The new RPC system does not yet support channel timeouts")
	}
	ctx, shutdown := test.V23Init()
	defer shutdown()
	ls := v23.GetListenSpec(ctx)
	pshutdown, pendpoint, err := generic.NewProxy(ctx, ls, security.AllowEveryone(), "")
	if err != nil {
		t.Fatal(err)
	}
	defer pshutdown()
	ls.Addrs = nil
	ls.Proxy = pendpoint.Name()
	testChannelTimeOut_Server(t, v23.WithListenSpec(ctx, ls))
}
