// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package discovery_test

import (
	"sync"
	"testing"

	"v.io/v23"
	"v.io/v23/discovery"
	"v.io/v23/naming"
	"v.io/v23/rpc"

	idiscovery "v.io/x/ref/lib/discovery"
	fdiscovery "v.io/x/ref/lib/discovery/factory"
	"v.io/x/ref/lib/discovery/plugins/mock"
	_ "v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/test"
)

type mockServer struct {
	mu    sync.Mutex
	eps   []naming.Endpoint
	valid chan struct{}
}

func (s *mockServer) AddName(string) error    { return nil }
func (s *mockServer) RemoveName(string)       {}
func (s *mockServer) Stop() error             { return nil }
func (s *mockServer) Closed() <-chan struct{} { return nil }
func (s *mockServer) Status() rpc.ServerStatus {
	defer s.mu.Unlock()
	s.mu.Lock()
	return rpc.ServerStatus{
		Endpoints: s.eps,
		Valid:     s.valid,
	}
}

func (s *mockServer) updateNetwork(eps []naming.Endpoint) {
	defer s.mu.Unlock()
	s.mu.Lock()
	s.eps = eps
	close(s.valid)
	s.valid = make(chan struct{})
}

func newMockServer(eps []naming.Endpoint) *mockServer {
	return &mockServer{
		eps:   eps,
		valid: make(chan struct{}),
	}
}

func newEndpoints(addrs ...string) []naming.Endpoint {
	eps := make([]naming.Endpoint, len(addrs))
	for i, a := range addrs {
		eps[i], _ = v23.NewEndpoint(a)
	}
	return eps
}

func setServiceAddrs(service *discovery.Service, eps []naming.Endpoint, suffix string) {
	service.Addrs = make([]string, len(eps))
	for i, ep := range eps {
		service.Addrs[i] = naming.JoinAddressName(ep.Name(), suffix)
	}
}

func TestAdvertiseServer(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	df, _ := idiscovery.NewFactory(ctx, mock.New())
	fdiscovery.InjectFactory(df)

	const suffix = "test"

	eps := newEndpoints("addr1:123")
	mock := newMockServer(eps)

	service := discovery.Service{
		InterfaceName: "v.io/v23/a",
		Attrs:         discovery.Attributes{"a1": "v1"},
	}

	_, err := idiscovery.AdvertiseServer(ctx, nil, mock, suffix, &service, nil)
	if err != nil {
		t.Fatal(err)
	}

	d, _ := v23.NewDiscovery(ctx)
	setServiceAddrs(&service, eps, suffix)
	if err := scanAndMatch(ctx, d, "", service); err != nil {
		t.Error(err)
	}

	tests := [][]naming.Endpoint{
		newEndpoints("addr2:123", "addr3:456"),
		newEndpoints("addr4:123"),
		newEndpoints("addr5:123", "addr6:456"),
	}
	for _, eps := range tests {
		mock.updateNetwork(eps)

		setServiceAddrs(&service, eps, suffix)
		if err := scanAndMatch(ctx, d, "", service); err != nil {
			t.Error(err)
		}
	}
}
