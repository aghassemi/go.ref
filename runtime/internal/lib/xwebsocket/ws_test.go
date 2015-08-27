// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package xwebsocket_test

import (
	"bytes"
	"crypto/rand"
	"testing"
	"time"

	"v.io/x/ref/runtime/internal/lib/tcputil"
	websocket "v.io/x/ref/runtime/internal/lib/xwebsocket"

	"v.io/v23/context"
	"v.io/v23/flow"
)

func TestWSToWS(t *testing.T) {
	runTest(t, websocket.WS{}, websocket.WS{}, "ws", "ws")
}

func TestWSToWSH(t *testing.T) {
	runTest(t, websocket.WS{}, websocket.WSH{}, "ws", "wsh")
}

func TestWSHToWSH(t *testing.T) {
	runTest(t, websocket.WSH{}, websocket.WSH{}, "wsh", "wsh")
}

func TestTCPToWSH(t *testing.T) {
	runTest(t, tcputil.TCP{}, websocket.WSH{}, "tcp", "wsh")
}

var randData []byte

const (
	chunkSize = 1 << 10
	numChunks = 10
)

func init() {
	randData = make([]byte, chunkSize*numChunks)
	if _, err := rand.Read(randData); err != nil {
		panic(err)
	}
}

func runTest(t *testing.T, dialObj, listenObj flow.Protocol, dialP, listenP string) {
	ctx, _ := context.RootContext()
	address := "127.0.0.1:0"
	timeout := time.Second
	acceptCh := make(chan flow.MsgReadWriteCloser)

	ln, err := listenObj.Listen(ctx, listenP, address)
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		a, err := ln.Accept(ctx)
		if err != nil {
			t.Fatal(err)
		}
		acceptCh <- a
	}()

	dialed, err := dialObj.Dial(ctx, dialP, ln.Addr().String(), timeout)
	if err != nil {
		t.Fatal(err)
	}
	go writeData(t, dialed, randData)
	go readData(t, dialed, randData)
	accepted := <-acceptCh
	go writeData(t, accepted, randData)
	go readData(t, accepted, randData)
}

func writeData(t *testing.T, c flow.MsgReadWriteCloser, data []byte) {
	for i := 0; i < numChunks; i++ {
		if _, err := c.WriteMsg(data[:chunkSize]); err != nil {
			t.Fatal(err)
		}
		data = data[chunkSize:]
	}
}

func readData(t *testing.T, c flow.MsgReadWriteCloser, expected []byte) {
	read := make([]byte, len(expected))
	read = read[:0]
	for i := 0; i < numChunks; i++ {
		b, err := c.ReadMsg()
		if err != nil {
			t.Fatal(err)
		}
		if len(b) != chunkSize {
			t.Errorf("got message of size %v, want %v", len(b), chunkSize)
		}
		read = append(read, b...)
	}
	if !bytes.Equal(read, expected) {
		t.Errorf("read %v, want %v", read, expected)
	}
}
