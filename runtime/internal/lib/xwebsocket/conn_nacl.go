// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build nacl

package xwebsocket

import (
	"net"
	"net/url"
	"runtime/ppapi"
	"sync"
	"time"

	"v.io/v23/context"
	"v.io/v23/flow"
)

// Ppapi instance which must be set before the Dial is called.
var PpapiInstance ppapi.Instance

func WebsocketConn(address string, ws *ppapi.WebsocketConn) flow.Conn {
	return &wrappedConn{
		address: address,
		ws:      ws,
	}
}

type wrappedConn struct {
	address   string
	ws        *ppapi.WebsocketConn
	readLock  sync.Mutex
	writeLock sync.Mutex
}

func Dial(ctx *context.T, protocol, address string, timeout time.Duration) (flow.Conn, error) {
	inst := PpapiInstance
	u, err := url.Parse("ws://" + address)
	if err != nil {
		return nil, err
	}

	ws, err := inst.DialWebsocket(u.String())
	if err != nil {
		return nil, err
	}
	return WebsocketConn(address, ws), nil
}

func Resolve(ctx *context.T, protocol, address string) (string, string, error) {
	return "ws", address, nil
}

func (c *wrappedConn) ReadMsg() ([]byte, error) {
	defer c.readLock.Unlock()
	c.readLock.Lock()
	return c.ws.ReceiveMessage()
}

func (c *wrappedConn) WriteMsg(bufs ...[]byte) (int, error) {
	defer c.writeLock.Unlock()
	c.writeLock.Lock()
	var b []byte
	for _, buf := range bufs {
		b = append(b, buf...)
	}
	if err := c.ws.SendMessage(b); err != nil {
		return 0, err
	}
	return len(b), nil
}

func (c *wrappedConn) Close() error {
	return c.ws.Close()
}

func (c *wrappedConn) LocalAddr() net.Addr {
	return websocketAddr{s: c.address}
}

type websocketAddr struct {
	s string
}

func (websocketAddr) Network() string {
	return "ws"
}

func (w websocketAddr) String() string {
	return w.s
}
