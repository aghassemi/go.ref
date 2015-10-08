// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package discovery

import (
	"sync"

	"github.com/pborman/uuid"

	"v.io/v23/context"
	"v.io/v23/discovery"
	"v.io/v23/verror"
)

const pkgPath = "v.io/x/ref/runtime/internal/discovery"

// Advertisement holds a set of service properties to advertise.
type Advertisement struct {
	discovery.Service

	// The service UUID to advertise.
	ServiceUuid uuid.UUID

	// Type of encryption applied to the advertisement so that it can
	// only be decoded by authorized principals.
	EncryptionAlgorithm EncryptionAlgorithm
	// If the advertisement is encrypted, then the data required to
	// decrypt it. The format of this data is a function of the algorithm.
	EncryptionKeys []EncryptionKey

	// TODO(jhahn): Add proximity.
	// TODO(jhahn): Use proximity for Lost.
	Lost bool
}

type EncryptionAlgorithm int
type EncryptionKey []byte

const (
	NoEncryption   EncryptionAlgorithm = 0
	TestEncryption EncryptionAlgorithm = 1
	IbeEncryption  EncryptionAlgorithm = 2
)

var (
	errClosed = verror.Register(pkgPath+".errClosed", verror.NoRetry, "{1:}{2:} closed")
)

// ds is an implementation of discovery.T.
type ds struct {
	plugins []Plugin

	mu     sync.Mutex
	closed bool                  // GUARDED_BY(mu)
	tasks  map[*context.T]func() // GUARDED_BY(mu)

	wg sync.WaitGroup
}

func (ds *ds) Close() {
	ds.mu.Lock()
	if ds.closed {
		ds.mu.Unlock()
		return
	}
	for _, cancel := range ds.tasks {
		cancel()
	}
	ds.closed = true
	ds.mu.Unlock()
	ds.wg.Wait()
}

func (ds *ds) addTask(ctx *context.T) (*context.T, func(), error) {
	ds.mu.Lock()
	if ds.closed {
		ds.mu.Unlock()
		return nil, nil, verror.New(errClosed, ctx)
	}
	ctx, cancel := context.WithCancel(ctx)
	ds.tasks[ctx] = cancel
	ds.wg.Add(1)
	ds.mu.Unlock()
	return ctx, cancel, nil
}

func (ds *ds) removeTask(ctx *context.T) {
	ds.mu.Lock()
	_, exist := ds.tasks[ctx]
	delete(ds.tasks, ctx)
	ds.mu.Unlock()
	if exist {
		ds.wg.Done()
	}
}

// New returns a new Discovery instance initialized with the given plugins.
//
// Mostly for internal use. Consider to use factory.New.
func NewWithPlugins(plugins []Plugin) discovery.T {
	ds := &ds{
		plugins: make([]Plugin, len(plugins)),
		tasks:   make(map[*context.T]func()),
	}
	copy(ds.plugins, plugins)
	return ds
}
