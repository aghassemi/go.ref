// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"path/filepath"

	"v.io/v23/naming"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/v23/security/access"
	"v.io/v23/verror"

	"v.io/x/ref/services/mgmt/lib/acls"
	"v.io/x/ref/services/mgmt/lib/fs"
	"v.io/x/ref/services/repository"
)

// dispatcher holds the state of the application repository dispatcher.
type dispatcher struct {
	store     *fs.Memstore
	storeRoot string
}

// NewDispatcher is the dispatcher factory. storeDir is a path to a directory in which to
// serialize the applicationd state.
func NewDispatcher(storeDir string) (rpc.Dispatcher, error) {
	store, err := fs.NewMemstore(filepath.Join(storeDir, "applicationdstate.db"))
	if err != nil {
		return nil, err
	}
	return &dispatcher{store: store, storeRoot: storeDir}, nil
}

func (d *dispatcher) Lookup(suffix string) (interface{}, security.Authorizer, error) {
	name, _, err := parse(nil, suffix)
	if err != nil {
		return nil, nil, err
	}

	auth, err := acls.NewHierarchicalAuthorizer(
		naming.Join("/acls", "data"),
		naming.Join("/acls", name, "data"),
		(*applicationAccessListStore)(d.store))
	if err != nil {
		return nil, nil, err
	}
	return repository.ApplicationServer(NewApplicationService(d.store, d.storeRoot, suffix)), auth, nil
}

type applicationAccessListStore fs.Memstore

// TAMForPath implements TAMGetter so that applicationd can use the
// hierarchicalAuthorizer
func (store *applicationAccessListStore) TAMForPath(path string) (access.Permissions, bool, error) {
	tam, _, err := getAccessList((*fs.Memstore)(store), path)

	if verror.ErrorID(err) == verror.ErrNoExist.ID {
		return nil, true, nil
	}
	if err != nil {
		return nil, false, err
	}
	return tam, false, nil
}
