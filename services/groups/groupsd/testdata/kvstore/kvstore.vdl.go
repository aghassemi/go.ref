// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file was auto-generated by the vanadium vdl tool.
// Source: kvstore.vdl

// Package kvstore implements a simple key-value store used for
// testing the groups-based authorization.
package kvstore

import (
	// VDL system imports
	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/rpc"
	"v.io/v23/vdl"

	// VDL user imports
	"v.io/v23/security/access"
)

// StoreClientMethods is the client interface
// containing Store methods.
type StoreClientMethods interface {
	Get(_ *context.T, key string, _ ...rpc.CallOpt) (string, error)
	Set(_ *context.T, key string, value string, _ ...rpc.CallOpt) error
}

// StoreClientStub adds universal methods to StoreClientMethods.
type StoreClientStub interface {
	StoreClientMethods
	rpc.UniversalServiceMethods
}

// StoreClient returns a client stub for Store.
func StoreClient(name string) StoreClientStub {
	return implStoreClientStub{name}
}

type implStoreClientStub struct {
	name string
}

func (c implStoreClientStub) Get(ctx *context.T, i0 string, opts ...rpc.CallOpt) (o0 string, err error) {
	err = v23.GetClient(ctx).Call(ctx, c.name, "Get", []interface{}{i0}, []interface{}{&o0}, opts...)
	return
}

func (c implStoreClientStub) Set(ctx *context.T, i0 string, i1 string, opts ...rpc.CallOpt) (err error) {
	err = v23.GetClient(ctx).Call(ctx, c.name, "Set", []interface{}{i0, i1}, nil, opts...)
	return
}

// StoreServerMethods is the interface a server writer
// implements for Store.
type StoreServerMethods interface {
	Get(_ *context.T, _ rpc.ServerCall, key string) (string, error)
	Set(_ *context.T, _ rpc.ServerCall, key string, value string) error
}

// StoreServerStubMethods is the server interface containing
// Store methods, as expected by rpc.Server.
// There is no difference between this interface and StoreServerMethods
// since there are no streaming methods.
type StoreServerStubMethods StoreServerMethods

// StoreServerStub adds universal methods to StoreServerStubMethods.
type StoreServerStub interface {
	StoreServerStubMethods
	// Describe the Store interfaces.
	Describe__() []rpc.InterfaceDesc
}

// StoreServer returns a server stub for Store.
// It converts an implementation of StoreServerMethods into
// an object that may be used by rpc.Server.
func StoreServer(impl StoreServerMethods) StoreServerStub {
	stub := implStoreServerStub{
		impl: impl,
	}
	// Initialize GlobState; always check the stub itself first, to handle the
	// case where the user has the Glob method defined in their VDL source.
	if gs := rpc.NewGlobState(stub); gs != nil {
		stub.gs = gs
	} else if gs := rpc.NewGlobState(impl); gs != nil {
		stub.gs = gs
	}
	return stub
}

type implStoreServerStub struct {
	impl StoreServerMethods
	gs   *rpc.GlobState
}

func (s implStoreServerStub) Get(ctx *context.T, call rpc.ServerCall, i0 string) (string, error) {
	return s.impl.Get(ctx, call, i0)
}

func (s implStoreServerStub) Set(ctx *context.T, call rpc.ServerCall, i0 string, i1 string) error {
	return s.impl.Set(ctx, call, i0, i1)
}

func (s implStoreServerStub) Globber() *rpc.GlobState {
	return s.gs
}

func (s implStoreServerStub) Describe__() []rpc.InterfaceDesc {
	return []rpc.InterfaceDesc{StoreDesc}
}

// StoreDesc describes the Store interface.
var StoreDesc rpc.InterfaceDesc = descStore

// descStore hides the desc to keep godoc clean.
var descStore = rpc.InterfaceDesc{
	Name:    "Store",
	PkgPath: "v.io/x/ref/services/groups/groupsd/testdata/kvstore",
	Methods: []rpc.MethodDesc{
		{
			Name: "Get",
			InArgs: []rpc.ArgDesc{
				{"key", ``}, // string
			},
			OutArgs: []rpc.ArgDesc{
				{"", ``}, // string
			},
			Tags: []*vdl.Value{vdl.ValueOf(access.Tag("Read"))},
		},
		{
			Name: "Set",
			InArgs: []rpc.ArgDesc{
				{"key", ``},   // string
				{"value", ``}, // string
			},
			Tags: []*vdl.Value{vdl.ValueOf(access.Tag("Write"))},
		},
	},
}
