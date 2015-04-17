// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal

import (
	"sync"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/x/lib/vlog"

	"v.io/x/ref/profiles/internal/rpc/stress"
)

type impl struct {
	statsMu        sync.Mutex
	sumCount       uint64 // GUARDED_BY(statsMu)
	sumStreamCount uint64 // GUARDED_BY(statsMu)

	stop chan struct{}
}

func (s *impl) Sum(_ *context.T, _ rpc.ServerCall, arg stress.Arg) ([]byte, error) {
	defer s.incSumCount()
	return doSum(arg)
}

func (s *impl) SumStream(_ *context.T, call stress.StressSumStreamServerCall) error {
	defer s.incSumStreamCount()
	rStream := call.RecvStream()
	sStream := call.SendStream()
	for rStream.Advance() {
		sum, err := doSum(rStream.Value())
		if err != nil {
			return err
		}
		sStream.Send(sum)
	}
	if err := rStream.Err(); err != nil {
		return err
	}
	return nil
}

func (s *impl) GetStats(*context.T, rpc.ServerCall) (stress.Stats, error) {
	s.statsMu.Lock()
	defer s.statsMu.Unlock()
	return stress.Stats{s.sumCount, s.sumStreamCount}, nil
}

func (s *impl) Stop(*context.T, rpc.ServerCall) error {
	s.stop <- struct{}{}
	return nil
}

func (s *impl) incSumCount() {
	s.statsMu.Lock()
	defer s.statsMu.Unlock()
	s.sumCount++
}

func (s *impl) incSumStreamCount() {
	s.statsMu.Lock()
	defer s.statsMu.Unlock()
	s.sumStreamCount++
}

type allowEveryoneAuthorizer struct{}

func (allowEveryoneAuthorizer) Authorize(*context.T, security.Call) error { return nil }

// StartServer starts a server that implements the Stress service, and returns
// the server and its vanadium address. It also returns a channel carrying stop
// requests. After reading from the stop channel, the application should exit.
func StartServer(ctx *context.T, listenSpec rpc.ListenSpec) (rpc.Server, naming.Endpoint, <-chan struct{}) {
	server, err := v23.NewServer(ctx)
	if err != nil {
		vlog.Fatalf("NewServer failed: %v", err)
	}
	eps, err := server.Listen(listenSpec)
	if err != nil {
		vlog.Fatalf("Listen failed: %v", err)
	}

	s := impl{stop: make(chan struct{})}
	if err := server.Serve("", stress.StressServer(&s), allowEveryoneAuthorizer{}); err != nil {
		vlog.Fatalf("Serve failed: %v", err)
	}

	return server, eps[0], s.stop
}
