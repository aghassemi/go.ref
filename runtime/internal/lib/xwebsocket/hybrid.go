// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package xwebsocket

import (
	"net"
	"time"

	"v.io/x/ref/runtime/internal/lib/framer"
	"v.io/x/ref/runtime/internal/lib/tcputil"

	"v.io/v23/context"
	"v.io/v23/flow"
)

type WSH struct{}

// Dial returns flow.MsgReadWriteCloser that can be used with a
// HybridListener but always uses tcp. A client must specifically elect to use
// websockets by calling websocket.Dialer. The returned net.Conn will report
// 'tcp' as its Network.
func (WSH) Dial(ctx *context.T, network, address string, timeout time.Duration) (flow.MsgReadWriteCloser, error) {
	tcp := mapWebSocketToTCP[network]
	conn, err := net.DialTimeout(tcp, address, timeout)
	if err != nil {
		return nil, err
	}
	if err := tcputil.EnableTCPKeepAlive(conn); err != nil {
		return nil, err
	}
	return framer.New(conn), nil
}

// Resolve performs a DNS resolution on the network, address and always
// returns tcp as its Network.
func (WSH) Resolve(ctx *context.T, network, address string) (string, string, error) {
	tcp := mapWebSocketToTCP[network]
	tcpAddr, err := net.ResolveTCPAddr(tcp, address)
	if err != nil {
		return "", "", err
	}
	return tcp, tcpAddr.String(), nil
}

// Listener returns a flow.MsgReadWriteCloser that supports both tcp and
// websockets over the same, single, port. A listen address of
// --v23.tcp.protocol=wsh --v23.tcp.address=127.0.0.1:8101 means
// that port 8101 can accept connections that use either tcp or websocket.
// The listener looks at the first 4 bytes of the incoming data stream
// to decide if it's a websocket protocol or not. These must be 'GET ' for
// websockets, all other protocols must guarantee to not send 'GET ' as the
// first four bytes of the payload.
func (WSH) Listen(ctx *context.T, protocol, address string) (flow.MsgListener, error) {
	return listener(protocol, address, true)
}
