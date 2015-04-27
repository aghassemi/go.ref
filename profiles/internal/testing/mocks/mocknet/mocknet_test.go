// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mocknet_test

import (
	"errors"
	"io"
	"net"
	"reflect"
	"sync"
	"testing"
	"time"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/options"
	"v.io/v23/rpc"
	"v.io/v23/verror"

	_ "v.io/x/ref/profiles"
	"v.io/x/ref/profiles/internal/rpc/stream/message"
	"v.io/x/ref/profiles/internal/testing/mocks/mocknet"
	"v.io/x/ref/test"
)

//go:generate v23 test generate

func newListener(t *testing.T, opts mocknet.Opts) net.Listener {
	ln, err := mocknet.ListenerWithOpts(opts, "test", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	return ln
}

func TestTrace(t *testing.T) {
	opts := mocknet.Opts{
		Mode: mocknet.Trace,
		Tx:   make(chan int, 100),
		Rx:   make(chan int, 100),
	}
	ln := newListener(t, opts)
	defer ln.Close()

	var rxconn net.Conn
	var rxerr error
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		rxconn, rxerr = ln.Accept()
		wg.Done()
	}()

	txconn, err := mocknet.DialerWithOpts(opts, "test", ln.Addr().String(), time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	wg.Wait()

	rw := func(s string) {
		b := make([]byte, len(s))
		txconn.Write([]byte(s))
		rxconn.Read(b[:])
		if got, want := string(b), s; got != want {
			t.Fatalf("got %v, want %v", got, want)
		}
	}

	sizes := []int{}
	for _, s := range []string{"hello", " ", "world"} {
		rw(s)
		sizes = append(sizes, len(s))
	}
	rxconn.Close()
	close(opts.Tx)
	close(opts.Rx)
	sizes = append(sizes, -1)

	drain := func(ch chan int) []int {
		r := []int{}
		for v := range ch {
			r = append(r, v)
		}
		return r
	}

	if got, want := drain(opts.Rx), sizes; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	if got, want := drain(opts.Tx), sizes; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestClose(t *testing.T) {
	cases := []struct {
		txClose, rxClose int
		tx               []string
		rx               []string
		err              error
	}{
		{6, 10, []string{"hello", "world"}, []string{"hello", "w"}, io.EOF},
		{5, 10, []string{"hello", "world"}, []string{"hello", ""}, io.EOF},
		{8, 6, []string{"hello", "world"}, []string{"hello", "w"}, io.EOF},
		{8, 5, []string{"hello", "world"}, []string{"hello", ""}, errors.New("use of closed network connection")},
	}

	for ci, c := range cases {
		opts := mocknet.Opts{
			Mode:      mocknet.Close,
			TxCloseAt: c.txClose,
			RxCloseAt: c.rxClose,
		}

		ln := newListener(t, opts)
		defer ln.Close()

		var rxconn net.Conn
		var rxerr error
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			rxconn, rxerr = ln.Accept()
			wg.Done()
		}()

		txconn, err := mocknet.DialerWithOpts(opts, "test", ln.Addr().String(), time.Minute)
		if err != nil {
			t.Fatal(err)
		}
		wg.Wait()

		rw := func(s string) (int, int, string, error) {
			b := make([]byte, len(s))
			tx, _ := txconn.Write([]byte(s))
			rx, err := rxconn.Read(b[:])
			return tx, rx, string(b[0:rx]), err
		}

		txBytes := 0
		rxBytes := 0
		for i, m := range c.tx {
			tx, rx, rxed, err := rw(m)
			if got, want := rxed, c.rx[i]; got != want {
				t.Fatalf("%d: got %q, want %q", ci, got, want)
			}
			txBytes += tx
			rxBytes += rx
			if err != nil {
				if got, want := err.Error(), c.err.Error(); got != want {
					t.Fatalf("%d: got %v, want %v", ci, got, want)
				}
			}
		}
		if got, want := txBytes, c.txClose; got != want {
			t.Fatalf("%d: got %v, want %v", ci, got, want)
		}
		rxWant := c.rxClose
		if rxWant > c.txClose {
			rxWant = c.txClose
		}
		if got, want := rxBytes, rxWant; got != want {
			t.Fatalf("%d: got %v, want %v", ci, got, want)

		}
	}
}

func TestDrop(t *testing.T) {
	cases := []struct {
		txDropAfter int
		tx          []string
		rx          []string
	}{
		{6, []string{"hello", "world"}, []string{"hello", "w"}},
		{2, []string{"hello", "world"}, []string{"he", "wo"}},
		{0, []string{"hello", "world"}, []string{"", ""}},
	}

	for ci, c := range cases {
		opts := mocknet.Opts{
			Mode:        mocknet.Drop,
			TxDropAfter: func() int { return c.txDropAfter },
		}

		ln := newListener(t, opts)
		defer ln.Close()

		var rxconn net.Conn
		var rxerr error
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			rxconn, rxerr = ln.Accept()
			wg.Done()
		}()

		txconn, err := mocknet.DialerWithOpts(opts, "test", ln.Addr().String(), time.Minute)
		if err != nil {
			t.Fatal(err)
		}
		wg.Wait()

		rw := func(s string, l int) (int, int, string, error) {
			b := make([]byte, l)
			tx, _ := txconn.Write([]byte(s))
			rx, err := rxconn.Read(b[:])
			return tx, rx, string(b[0:rx]), err
		}

		for i, m := range c.tx {
			tx, rx, rxed, _ := rw(m, len(c.rx[i]))
			if got, want := rxed, c.rx[i]; got != want {
				t.Fatalf("%d: got %q, want %q", ci, got, want)
			}
			if tx != rx {
				t.Fatalf("%d: tx %d, rx %d", ci, tx, rx)
			}
		}
	}
}

func newCtx() (*context.T, v23.Shutdown) {
	ctx, shutdown := test.InitForTest()
	v23.GetNamespace(ctx).CacheCtl(naming.DisableCache(true))
	return ctx, shutdown
}

type simple struct{}

func (s *simple) Ping(call rpc.ServerCall) (string, error) {
	return "pong", nil
}

func initServer(t *testing.T, ctx *context.T) (string, func()) {
	server, err := v23.NewServer(ctx, options.SecurityNone)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	done := make(chan struct{})
	deferFn := func() { close(done); server.Stop() }

	eps, err := server.Listen(v23.GetListenSpec(ctx))
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	server.Serve("", &simple{}, nil)
	return eps[0].Name(), deferFn
}

func TestV23Control(t *testing.T) {
	ctx, shutdown := newCtx()
	defer shutdown()

	matcher := func(_ bool, msg message.T) bool {
		switch msg.(type) {
		case *message.Data:
			return false
		}
		// drop first control message
		return true
	}

	dropControlDialer := func(network, address string, timeout time.Duration) (net.Conn, error) {
		opts := mocknet.Opts{
			Mode:              mocknet.V23CloseAtMessage,
			V23MessageMatcher: matcher,
		}
		return mocknet.DialerWithOpts(opts, network, address, timeout)
	}

	rpc.RegisterProtocol("dropControl", dropControlDialer, net.Listen)

	server, fn := initServer(t, ctx)
	defer fn()

	addr, _ := naming.SplitAddressName(server)
	dropServer, err := mocknet.RewriteEndpointProtocol(addr, "dropControl")
	if err != nil {
		t.Fatal(err)
	}

	_, err = v23.GetClient(ctx).StartCall(ctx, dropServer.Name(), "Ping", nil, options.SecurityNone)
	if verror.ErrorID(err) != verror.ErrBadProtocol.ID {
		t.Fatal(err)
	}
}
