// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mounttablelib

import (
	"sync"

	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/v23/verror"
)

// collectionServer is a very simple collection server implementation for testing, with sufficient debugging to help
// when there are problems.
type collectionServer struct {
	sync.Mutex
	contents map[string][]byte
}
type collectionDispatcher struct {
	*collectionServer
}
type rpcContext struct {
	name string
	*collectionServer
}

var instance collectionServer

func newCollectionServer() *collectionDispatcher {
	return &collectionDispatcher{collectionServer: &collectionServer{contents: make(map[string][]byte)}}
}

// Lookup implements rpc.Dispatcher.Lookup.
func (d *collectionDispatcher) Lookup(name string) (interface{}, security.Authorizer, error) {
	rpcc := &rpcContext{name: name, collectionServer: d.collectionServer}
	return rpcc, d, nil
}

func (collectionDispatcher) Authorize(*context.T) error {
	return nil
}

// Export implements CollectionServerMethods.Export.
func (c *rpcContext) Export(call rpc.ServerCall, val []byte, overwrite bool) error {
	c.Lock()
	defer c.Unlock()
	if b := c.contents[c.name]; overwrite || b == nil {
		c.contents[c.name] = val
		return nil
	}
	return verror.New(naming.ErrNameExists, call.Context(), c.name)
}

// Lookup implements CollectionServerMethods.Lookup.
func (c *rpcContext) Lookup(call rpc.ServerCall) ([]byte, error) {
	c.Lock()
	defer c.Unlock()
	if val := c.contents[c.name]; val != nil {
		return val, nil
	}
	return nil, verror.New(naming.ErrNoSuchName, call.Context(), c.name)
}
