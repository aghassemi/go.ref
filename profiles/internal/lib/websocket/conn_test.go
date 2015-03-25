// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build !nacl
package websocket

import (
	"bytes"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func writer(c net.Conn, data []byte, times int, wg *sync.WaitGroup) {
	defer wg.Done()
	b := []byte{byte(len(data))}
	b = append(b, data...)
	for i := 0; i < times; i++ {
		c.Write(b)
	}
}

func readMessage(c net.Conn) ([]byte, error) {
	var length [1]byte
	// Read the size
	for {
		n, err := c.Read(length[:])
		if err != nil {
			return nil, err
		}
		if n == 1 {
			break
		}
	}
	size := int(length[0])
	buf := make([]byte, size)
	n := 0
	for n < size {
		nn, err := c.Read(buf[n:])
		if err != nil {
			return buf, err
		}
		n += nn
	}

	return buf, nil
}

func reader(t *testing.T, c net.Conn, expected []byte, totalWrites int) {
	totalReads := 0
	for buf, err := readMessage(c); err == nil; buf, err = readMessage(c) {
		totalReads++
		if !bytes.Equal(buf, expected) {
			t.Errorf("Unexpected message %v, expected %v", buf, expected)
		}
	}
	if totalReads != totalWrites {
		t.Errorf("wrong number of messages expected %v, got %v", totalWrites, totalReads)
	}
}

func TestMultipleGoRoutines(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}
	addr := l.Addr()
	input := []byte("no races here")
	const numWriters int = 12
	const numWritesPerWriter int = 1000
	const totalWrites int = numWriters * numWritesPerWriter
	s := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "GET" {
				http.Error(w, "Method not allowed.", http.StatusMethodNotAllowed)
				return
			}
			ws, err := websocket.Upgrade(w, r, nil, 1024, 1024)
			if _, ok := err.(websocket.HandshakeError); ok {
				http.Error(w, "Not a websocket handshake", 400)
				return
			} else if err != nil {
				http.Error(w, "Internal Error", 500)
				return
			}
			reader(t, WebsocketConn(ws), input, totalWrites)
		}),
	}
	// Dial out in another go routine
	go func() {
		conn, err := Dial("tcp", addr.String(), time.Second)
		numTries := 0
		for err != nil && numTries < 5 {
			numTries++
			time.Sleep(time.Second)
		}

		if err != nil {
			t.Fatalf("failed to connect to server: %v", err)
		}
		var writers sync.WaitGroup
		writers.Add(numWriters)
		for i := 0; i < numWriters; i++ {
			go writer(conn, input, numWritesPerWriter, &writers)
		}
		writers.Wait()
		conn.Close()
		l.Close()
	}()
	s.Serve(l)
}
