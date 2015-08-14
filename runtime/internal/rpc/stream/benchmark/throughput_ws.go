// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package benchmark

import (
	"io"
	"net"
	"testing"

	"v.io/x/ref/runtime/internal/lib/websocket"

	"v.io/v23/context"
)

// benchmarkWS sets up nConns WS connections and measures throughput.
func benchmarkWS(b *testing.B, nConns int) {
	ctx, _ := context.RootContext()
	rchan := make(chan net.Conn, nConns)
	wchan := make(chan net.Conn, nConns)
	ln, err := websocket.Listener(ctx, "ws", "127.0.0.1:0")
	if err != nil {
		b.Fatalf("websocket.Listener failed: %v", err)
		return
	}
	defer ln.Close()
	// One goroutine to dial nConns connections.
	go func() {
		for i := 0; i < nConns; i++ {
			conn, err := websocket.Dial(ctx, "ws", ln.Addr().String(), 0)
			if err != nil {
				b.Fatalf("websocket.Dial(%q, %q) failed: %v", "ws", ln.Addr(), err)
				wchan <- nil
				return
			}
			wchan <- conn
		}
		close(wchan)
	}()
	// One goroutine to accept nConns connections.
	go func() {
		for i := 0; i < nConns; i++ {
			conn, err := ln.Accept()
			if err != nil {
				b.Fatalf("Accept failed: %v", err)
				rchan <- nil
				return
			}
			rchan <- conn
		}
		close(rchan)
	}()

	var readers []io.ReadCloser
	var writers []io.WriteCloser
	for r := range rchan {
		readers = append(readers, r)
	}
	for w := range wchan {
		writers = append(writers, w)
	}
	if b.Failed() {
		return
	}
	(&throughputTester{b: b, readers: readers, writers: writers}).Run()
}
