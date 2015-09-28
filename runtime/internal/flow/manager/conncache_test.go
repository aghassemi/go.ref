// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package manager

import (
	"strconv"
	"testing"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/flow"
	"v.io/v23/naming"
	"v.io/v23/rpc/version"

	connpackage "v.io/x/ref/runtime/internal/flow/conn"
	"v.io/x/ref/runtime/internal/flow/flowtest"
	inaming "v.io/x/ref/runtime/internal/naming"
)

func TestCache(t *testing.T) {
	ctx, shutdown := v23.Init()
	defer shutdown()

	c := NewConnCache()
	remote := &inaming.Endpoint{
		Protocol:  "tcp",
		Address:   "127.0.0.1:1111",
		RID:       naming.FixedRoutingID(0x5555),
		Blessings: []string{"A", "B", "C"},
	}
	conn := makeConnAndFlow(t, ctx, remote).c
	if err := c.Insert(conn); err != nil {
		t.Fatal(err)
	}
	// We should be able to find the conn in the cache.
	if got, err := c.ReservedFind(remote.Protocol, remote.Address, remote.Blessings); err != nil || got != conn {
		t.Errorf("got %v, want %v, err: %v", got, conn, err)
	}
	c.Unreserve(remote.Protocol, remote.Address, remote.Blessings)
	// Changing the protocol should fail.
	if got, err := c.ReservedFind("wrong", remote.Address, remote.Blessings); err != nil || got != nil {
		t.Errorf("got %v, want <nil>, err: %v", got, err)
	}
	c.Unreserve("wrong", remote.Address, remote.Blessings)
	// Changing the address should fail.
	if got, err := c.ReservedFind(remote.Protocol, "wrong", remote.Blessings); err != nil || got != nil {
		t.Errorf("got %v, want <nil>, err: %v", got, err)
	}
	c.Unreserve(remote.Protocol, "wrong", remote.Blessings)
	// Changing the blessingNames should fail.
	if got, err := c.ReservedFind(remote.Protocol, remote.Address, []string{"wrong"}); err != nil || got != nil {
		t.Errorf("got %v, want <nil>, err: %v", got, err)
	}
	c.Unreserve(remote.Protocol, remote.Address, []string{"wrong"})

	// We should be able to find the conn in the cache by looking up the RoutingID.
	if got, err := c.FindWithRoutingID(remote.RID); err != nil || got != conn {
		t.Errorf("got %v, want %v, err: %v", got, conn, err)
	}
	// Looking up the wrong RID should fail.
	if got, err := c.FindWithRoutingID(naming.FixedRoutingID(0x1111)); err != nil || got != nil {
		t.Errorf("got %v, want <nil>, err: %v", got, err)
	}

	// Caching with InsertWithRoutingID should only cache by RoutingID, not with network/address.
	ridEP := &inaming.Endpoint{
		Protocol:  "ridonly",
		Address:   "ridonly",
		RID:       naming.FixedRoutingID(0x1111),
		Blessings: []string{"ridonly"},
	}
	ridConn := makeConnAndFlow(t, ctx, ridEP).c
	if err := c.InsertWithRoutingID(ridConn); err != nil {
		t.Fatal(err)
	}
	if got, err := c.ReservedFind(ridEP.Protocol, ridEP.Address, ridEP.Blessings); err != nil || got != nil {
		t.Errorf("got %v, want <nil>, err: %v", got, err)
	}
	c.Unreserve(ridEP.Protocol, ridEP.Address, ridEP.Blessings)
	if got, err := c.FindWithRoutingID(ridEP.RID); err != nil || got != ridConn {
		t.Errorf("got %v, want %v, err: %v", got, ridConn, err)
	}

	otherEP := &inaming.Endpoint{
		Protocol:  "other",
		Address:   "other",
		Blessings: []string{"other"},
	}
	otherConn := makeConnAndFlow(t, ctx, otherEP).c

	// Looking up a not yet inserted endpoint should fail.
	if got, err := c.ReservedFind(otherEP.Protocol, otherEP.Address, otherEP.Blessings); err != nil || got != nil {
		t.Errorf("got %v, want <nil>, err: %v", got, err)
	}
	// Looking it up again should block until a matching Unreserve call is made.
	ch := make(chan *connpackage.Conn, 1)
	go func(ch chan *connpackage.Conn) {
		conn, err := c.ReservedFind(otherEP.Protocol, otherEP.Address, otherEP.Blessings)
		if err != nil {
			t.Fatal(err)
		}
		ch <- conn
	}(ch)

	// We insert the other conn into the cache.
	if err := c.Insert(otherConn); err != nil {
		t.Fatal(err)
	}
	c.Unreserve(otherEP.Protocol, otherEP.Address, otherEP.Blessings)
	// Now the c.ReservedFind should have unblocked and returned the correct Conn.
	if cachedConn := <-ch; cachedConn != otherConn {
		t.Errorf("got %v, want %v", cachedConn, otherConn)
	}

	// Insert a duplicate conn to ensure that replaced conns still get closed.
	dupConn := makeConnAndFlow(t, ctx, remote).c
	if err := c.Insert(dupConn); err != nil {
		t.Fatal(err)
	}

	// Closing the cache should close all the connections in the cache.
	// Ensure that the conns are not closed yet.
	if isClosed(conn) {
		t.Fatal("wanted conn to not be closed")
	}
	if isClosed(dupConn) {
		t.Fatal("wanted dupConn to not be closed")
	}
	if isClosed(otherConn) {
		t.Fatal("wanted otherConn to not be closed")
	}
	c.Close(ctx)
	// Now the connections should be closed.
	<-conn.Closed()
	<-dupConn.Closed()
	<-otherConn.Closed()
}

func TestLRU(t *testing.T) {
	ctx, shutdown := v23.Init()
	defer shutdown()

	// Ensure that the least recently created conns are killed by KillConnections.
	c := NewConnCache()
	conns := nConnAndFlows(t, ctx, 10)
	for _, conn := range conns {
		if err := c.Insert(conn.c); err != nil {
			t.Fatal(err)
		}
	}
	if err := c.KillConnections(ctx, 3); err != nil {
		t.Fatal(err)
	}
	if !cacheSizeMatches(c) {
		t.Errorf("the size of the caches and LRU list do not match")
	}
	// conns[3:] should not be closed and still in the cache.
	// conns[:3] should be closed and removed from the cache.
	for _, conn := range conns[3:] {
		if isClosed(conn.c) {
			t.Errorf("conn %v should not have been closed", conn)
		}
		if !isInCache(t, c, conn.c) {
			t.Errorf("conn %v should still be in cache", conn)
		}
	}
	for _, conn := range conns[:3] {
		<-conn.c.Closed()
		if isInCache(t, c, conn.c) {
			t.Errorf("conn %v should not be in cache", conn)
		}
	}

	// Ensure that writing to conns marks conns as more recently used.
	c = NewConnCache()
	conns = nConnAndFlows(t, ctx, 10)
	for _, conn := range conns {
		if err := c.Insert(conn.c); err != nil {
			t.Fatal(err)
		}
	}
	for _, conn := range conns[:7] {
		conn.write()
	}
	if err := c.KillConnections(ctx, 3); err != nil {
		t.Fatal(err)
	}
	if !cacheSizeMatches(c) {
		t.Errorf("the size of the caches and LRU list do not match")
	}
	// conns[:7] should not be closed and still in the cache.
	// conns[7:] should be closed and removed from the cache.
	for _, conn := range conns[:7] {
		if isClosed(conn.c) {
			t.Errorf("conn %v should not have been closed", conn)
		}
		if !isInCache(t, c, conn.c) {
			t.Errorf("conn %v should still be in cache", conn)
		}
	}
	for _, conn := range conns[7:] {
		<-conn.c.Closed()
		if isInCache(t, c, conn.c) {
			t.Errorf("conn %v should not be in cache", conn)
		}
	}

	// Ensure that reading from conns marks conns as more recently used.
	c = NewConnCache()
	conns = nConnAndFlows(t, ctx, 10)
	for _, conn := range conns {
		if err := c.Insert(conn.c); err != nil {
			t.Fatal(err)
		}
	}
	for _, conn := range conns[:7] {
		conn.read()
	}
	if err := c.KillConnections(ctx, 3); err != nil {
		t.Fatal(err)
	}
	if !cacheSizeMatches(c) {
		t.Errorf("the size of the caches and LRU list do not match")
	}
	// conns[:7] should not be closed and still in the cache.
	// conns[7:] should be closed and removed from the cache.
	for _, conn := range conns[:7] {
		if isClosed(conn.c) {
			t.Errorf("conn %v should not have been closed", conn)
		}
		if !isInCache(t, c, conn.c) {
			t.Errorf("conn %v should still be in cache", conn)
		}
	}
	for _, conn := range conns[7:] {
		<-conn.c.Closed()
		if isInCache(t, c, conn.c) {
			t.Errorf("conn %v should not be in cache", conn)
		}
	}
}

func isInCache(t *testing.T, c *ConnCache, conn *connpackage.Conn) bool {
	rep := conn.RemoteEndpoint()
	rfconn, err := c.ReservedFind(rep.Addr().Network(), rep.Addr().String(), rep.BlessingNames())
	if err != nil {
		t.Error(err)
	}
	c.Unreserve(rep.Addr().Network(), rep.Addr().String(), rep.BlessingNames())
	ridconn, err := c.FindWithRoutingID(rep.RoutingID())
	if err != nil {
		t.Error(err)
	}
	return rfconn != nil || ridconn != nil
}

func cacheSizeMatches(c *ConnCache) bool {
	return len(c.addrCache) == len(c.ridCache)
}

type connAndFlow struct {
	c *connpackage.Conn
	f flow.Flow
}

func (c connAndFlow) write() {
	_, err := c.f.WriteMsg([]byte{0})
	if err != nil {
		panic(err)
	}
}

func (c connAndFlow) read() {
	_, err := c.f.ReadMsg()
	if err != nil {
		panic(err)
	}
}

func nConnAndFlows(t *testing.T, ctx *context.T, n int) []connAndFlow {
	cfs := make([]connAndFlow, n)
	for i := 0; i < n; i++ {
		cfs[i] = makeConnAndFlow(t, ctx, &inaming.Endpoint{
			Protocol: strconv.Itoa(i),
			RID:      naming.FixedRoutingID(uint64(i)),
		})
	}
	return cfs
}

func makeConnAndFlow(t *testing.T, ctx *context.T, ep naming.Endpoint) connAndFlow {
	dmrw, amrw, _ := flowtest.NewMRWPair(ctx)
	dch := make(chan *connpackage.Conn)
	ach := make(chan *connpackage.Conn)
	go func() {
		d, err := connpackage.NewDialed(ctx, dmrw, ep, ep,
			version.RPCVersionRange{Min: 1, Max: 5}, nil)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		dch <- d
	}()
	fh := fh{t, make(chan struct{})}
	go func() {
		a, err := connpackage.NewAccepted(ctx, amrw, ep,
			version.RPCVersionRange{Min: 1, Max: 5}, fh)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		ach <- a
	}()
	conn := <-dch
	<-ach
	f, err := conn.Dial(ctx, flowtest.BlessingsForPeer)
	if err != nil {
		t.Fatal(err)
	}
	// Write a byte to send the openFlow message.
	if _, err := f.Write([]byte{0}); err != nil {
		t.Fatal(err)
	}
	<-fh.ch
	return connAndFlow{conn, f}
}

type fh struct {
	t  *testing.T
	ch chan struct{}
}

func (h fh) HandleFlow(f flow.Flow) error {
	go func() {
		if _, err := f.WriteMsg([]byte{0}); err != nil {
			h.t.Errorf("failed to write: %v", err)
		}
		close(h.ch)
	}()
	return nil
}
