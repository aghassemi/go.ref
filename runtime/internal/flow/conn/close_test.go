// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package conn

import (
	"bytes"
	"fmt"
	"io"
	"testing"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/flow"
	_ "v.io/x/ref/runtime/factories/fake"
	"v.io/x/ref/runtime/internal/flow/flowtest"
	"v.io/x/ref/test/goroutines"
)

func TestRemoteDialerClose(t *testing.T) {
	defer goroutines.NoLeaks(t, leakWaitTime)()

	ctx, shutdown := v23.Init()
	defer shutdown()
	d, a, w := setupConns(t, ctx, ctx, nil, nil, false)
	d.Close(ctx, fmt.Errorf("Closing randomly."))
	<-d.Closed()
	<-a.Closed()
	if !w.IsClosed() {
		t.Errorf("The connection should be closed")
	}
}

func TestRemoteAcceptorClose(t *testing.T) {
	defer goroutines.NoLeaks(t, leakWaitTime)()

	ctx, shutdown := v23.Init()
	defer shutdown()
	d, a, w := setupConns(t, ctx, ctx, nil, nil, false)
	a.Close(ctx, fmt.Errorf("Closing randomly."))
	<-a.Closed()
	<-d.Closed()
	if !w.IsClosed() {
		t.Errorf("The connection should be closed")
	}
}

func TestUnderlyingConnectionClosed(t *testing.T) {
	defer goroutines.NoLeaks(t, leakWaitTime)()

	ctx, shutdown := v23.Init()
	defer shutdown()
	d, a, w := setupConns(t, ctx, ctx, nil, nil, false)
	w.Close()
	<-a.Closed()
	<-d.Closed()
}

func TestDialAfterConnClose(t *testing.T) {
	defer goroutines.NoLeaks(t, leakWaitTime)()

	ctx, shutdown := v23.Init()
	defer shutdown()
	d, a, _ := setupConns(t, ctx, ctx, nil, nil, false)

	d.Close(ctx, fmt.Errorf("Closing randomly."))
	<-d.Closed()
	<-a.Closed()
	if _, err := d.Dial(ctx, flowtest.AllowAllPeersAuthorizer{}); err == nil {
		t.Errorf("Nil error dialing on dialer")
	}
	if _, err := a.Dial(ctx, flowtest.AllowAllPeersAuthorizer{}); err == nil {
		t.Errorf("Nil error dialing on acceptor")
	}
}

func TestReadWriteAfterConnClose(t *testing.T) {
	defer goroutines.NoLeaks(t, leakWaitTime)()

	ctx, shutdown := v23.Init()
	defer shutdown()
	for _, dialerDials := range []bool{true, false} {
		df, flows, cl := setupFlow(t, ctx, ctx, dialerDials)
		if _, err := df.WriteMsg([]byte("hello")); err != nil {
			t.Fatalf("write failed: %v", err)
		}
		af := <-flows
		if got, err := af.ReadMsg(); err != nil {
			t.Fatalf("read failed: %v", err)
		} else if !bytes.Equal(got, []byte("hello")) {
			t.Errorf("got %s want %s", string(got), "hello")
		}
		if _, err := df.WriteMsg([]byte("there")); err != nil {
			t.Fatalf("second write failed: %v", err)
		}
		df.(*flw).conn.Close(ctx, fmt.Errorf("Closing randomly."))
		<-af.Conn().Closed()
		if got, err := af.ReadMsg(); err != nil {
			t.Fatalf("read failed: %v", err)
		} else if !bytes.Equal(got, []byte("there")) {
			t.Errorf("got %s want %s", string(got), "there")
		}
		if _, err := df.WriteMsg([]byte("fail")); err == nil {
			t.Errorf("nil error for write after close.")
		}
		if _, err := af.ReadMsg(); err == nil {
			t.Fatalf("nil error for read after close.")
		}
		cl()
	}
}

func TestFlowCancelOnWrite(t *testing.T) {
	defer goroutines.NoLeaks(t, leakWaitTime)()

	ctx, shutdown := v23.Init()
	defer shutdown()
	accept := make(chan flow.Flow, 1)
	dc, ac, _ := setupConns(t, ctx, ctx, nil, accept, false)
	defer func() {
		dc.Close(ctx, nil)
		ac.Close(ctx, nil)
	}()
	dctx, cancel := context.WithCancel(ctx)
	df, err := dc.Dial(dctx, flowtest.AllowAllPeersAuthorizer{})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		if _, err := df.WriteMsg([]byte("hello")); err != nil {
			panic("could not write flow: " + err.Error())
		}
		for {
			if _, err := df.WriteMsg([]byte("hello")); err == io.EOF {
				break
			} else if err != nil {
				panic("unexpected error waiting for cancel: " + err.Error())
			}
		}
		close(done)
	}()
	af := <-accept
	cancel()
	<-done
	<-af.Closed()
}

func TestFlowCancelOnRead(t *testing.T) {
	defer goroutines.NoLeaks(t, leakWaitTime)()

	ctx, shutdown := v23.Init()
	defer shutdown()
	accept := make(chan flow.Flow, 1)
	dc, ac, _ := setupConns(t, ctx, ctx, nil, accept, false)
	defer func() {
		dc.Close(ctx, nil)
		ac.Close(ctx, nil)
	}()
	dctx, cancel := context.WithCancel(ctx)
	df, err := dc.Dial(dctx, flowtest.AllowAllPeersAuthorizer{})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		if _, err := df.WriteMsg([]byte("hello")); err != nil {
			t.Fatalf("could not write flow: %v", err)
		}
		if _, err := df.ReadMsg(); err != io.EOF {
			t.Fatalf("unexpected error waiting for cancel: %v", err)
		}
		close(done)
	}()
	af := <-accept
	cancel()
	<-done
	<-af.Closed()
}
