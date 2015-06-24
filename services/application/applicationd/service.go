// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"sort"
	"strings"

	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/v23/security/access"
	"v.io/v23/services/application"
	"v.io/v23/verror"
	"v.io/x/lib/set"
	"v.io/x/ref/services/internal/fs"
	"v.io/x/ref/services/internal/pathperms"
	"v.io/x/ref/services/repository"
)

// appRepoService implements the Application repository interface.
type appRepoService struct {
	// store is the storage server used for storing application
	// metadata.
	// All objects share the same Memstore.
	store *fs.Memstore
	// storeRoot is a name in the directory under which all data will be
	// stored.
	storeRoot string
	// suffix is the name of the application object.
	suffix string
}

const pkgPath = "v.io/x/ref/services/application/applicationd/"

var (
	ErrInvalidSuffix   = verror.Register(pkgPath+".InvalidSuffix", verror.NoRetry, "{1:}{2:} invalid suffix{:_}")
	ErrOperationFailed = verror.Register(pkgPath+".OperationFailed", verror.NoRetry, "{1:}{2:} operation failed{:_}")
	ErrNotAuthorized   = verror.Register(pkgPath+".errNotAuthorized", verror.NoRetry, "{1:}{2:} none of the client's blessings are valid {:_}")
)

// NewApplicationService returns a new Application service implementation.
func NewApplicationService(store *fs.Memstore, storeRoot, suffix string) repository.ApplicationServerMethods {
	return &appRepoService{store: store, storeRoot: storeRoot, suffix: suffix}
}

func parse(ctx *context.T, suffix string) (string, string, error) {
	tokens := strings.Split(suffix, "/")
	switch len(tokens) {
	case 2:
		return tokens[0], tokens[1], nil
	case 1:
		return tokens[0], "", nil
	default:
		return "", "", verror.New(ErrInvalidSuffix, ctx)
	}
}

func (i *appRepoService) Match(ctx *context.T, call rpc.ServerCall, profiles []string) (application.Envelope, error) {
	ctx.VI(0).Infof("%v.Match(%v)", i.suffix, profiles)
	empty := application.Envelope{}
	name, version, err := parse(ctx, i.suffix)
	if err != nil {
		return empty, err
	}

	i.store.Lock()
	defer i.store.Unlock()

	if version == "" {
		versions, err := i.allAppVersions(name)
		if err != nil {
			return empty, err
		}
		if len(versions) < 1 {
			return empty, verror.New(ErrInvalidSuffix, ctx)
		}
		sort.Strings(versions)
		version = versions[len(versions)-1]
	}

	for _, profile := range profiles {
		path := naming.Join("/applications", name, profile, version)
		entry, err := i.store.BindObject(path).Get(call)
		if err != nil {
			continue
		}
		envelope, ok := entry.Value.(application.Envelope)
		if !ok {
			continue
		}
		return envelope, nil
	}
	return empty, verror.New(verror.ErrNoExist, ctx)
}

func (i *appRepoService) Put(ctx *context.T, call rpc.ServerCall, profiles []string, envelope application.Envelope) error {
	ctx.VI(0).Infof("%v.Put(%v, %v)", i.suffix, profiles, envelope)
	name, version, err := parse(ctx, i.suffix)
	if err != nil {
		return err
	}
	if version == "" {
		return verror.New(ErrInvalidSuffix, ctx)
	}
	i.store.Lock()
	defer i.store.Unlock()
	// Transaction is rooted at "", so tname == tid.
	tname, err := i.store.BindTransactionRoot("").CreateTransaction(call)
	if err != nil {
		return err
	}

	// Only add a Permissions value if there is not already one present.
	apath := naming.Join("/acls", name, "data")
	aobj := i.store.BindObject(apath)
	if _, err := aobj.Get(call); verror.ErrorID(err) == fs.ErrNotInMemStore.ID {
		rb, _ := security.RemoteBlessingNames(ctx, call.Security())
		if len(rb) == 0 {
			// None of the client's blessings are valid.
			return verror.New(ErrNotAuthorized, ctx)
		}
		newperms := pathperms.PermissionsForBlessings(rb)
		if _, err := aobj.Put(nil, newperms); err != nil {
			return err
		}
	}

	for _, profile := range profiles {
		path := naming.Join(tname, "/applications", name, profile, version)

		object := i.store.BindObject(path)
		_, err := object.Put(call, envelope)
		if err != nil {
			return verror.New(ErrOperationFailed, ctx)
		}
	}
	if err := i.store.BindTransaction(tname).Commit(call); err != nil {
		return verror.New(ErrOperationFailed, ctx)
	}
	return nil
}

func (i *appRepoService) Remove(ctx *context.T, call rpc.ServerCall, profile string) error {
	ctx.VI(0).Infof("%v.Remove(%v)", i.suffix, profile)
	name, version, err := parse(ctx, i.suffix)
	if err != nil {
		return err
	}
	i.store.Lock()
	defer i.store.Unlock()
	// Transaction is rooted at "", so tname == tid.
	tname, err := i.store.BindTransactionRoot("").CreateTransaction(call)
	if err != nil {
		return err
	}
	path := naming.Join(tname, "/applications", name, profile)
	if version != "" {
		path += "/" + version
	}
	object := i.store.BindObject(path)
	found, err := object.Exists(call)
	if err != nil {
		return verror.New(ErrOperationFailed, ctx)
	}
	if !found {
		return verror.New(verror.ErrNoExist, ctx)
	}
	if err := object.Remove(call); err != nil {
		return verror.New(ErrOperationFailed, ctx)
	}
	if err := i.store.BindTransaction(tname).Commit(call); err != nil {
		return verror.New(ErrOperationFailed, ctx)
	}
	return nil
}

func (i *appRepoService) allApplications() ([]string, error) {
	apps, err := i.store.BindObject("/applications").Children()
	if err != nil {
		return nil, err
	}
	return apps, nil
}

func (i *appRepoService) allAppVersions(appName string) ([]string, error) {
	profiles, err := i.store.BindObject(naming.Join("/applications", appName)).Children()
	if err != nil {
		return nil, err
	}
	uniqueVersions := make(map[string]struct{})
	for _, profile := range profiles {
		versions, err := i.store.BindObject(naming.Join("/applications", appName, profile)).Children()
		if err != nil {
			return nil, err
		}
		set.String.Union(uniqueVersions, set.String.FromSlice(versions))
	}
	return set.String.ToSlice(uniqueVersions), nil
}

func (i *appRepoService) GlobChildren__(ctx *context.T, _ rpc.ServerCall) (<-chan string, error) {
	ctx.VI(0).Infof("%v.GlobChildren__()", i.suffix)
	i.store.Lock()
	defer i.store.Unlock()

	var elems []string
	if i.suffix != "" {
		elems = strings.Split(i.suffix, "/")
	}

	var results []string
	var err error
	switch len(elems) {
	case 0:
		results, err = i.allApplications()
		if err != nil {
			return nil, err
		}
	case 1:
		results, err = i.allAppVersions(elems[0])
		if err != nil {
			return nil, err
		}
	case 2:
		versions, err := i.allAppVersions(elems[0])
		if err != nil {
			return nil, err
		}
		for _, v := range versions {
			if v == elems[1] {
				return nil, nil
			}
		}
		return nil, verror.New(verror.ErrNoExist, nil)
	default:
		return nil, verror.New(verror.ErrNoExist, nil)
	}

	ch := make(chan string, len(results))
	for _, r := range results {
		ch <- r
	}
	close(ch)
	return ch, nil
}

func (i *appRepoService) GetPermissions(ctx *context.T, call rpc.ServerCall) (perms access.Permissions, version string, err error) {
	name, _, err := parse(ctx, i.suffix)
	if err != nil {
		return nil, "", err
	}
	i.store.Lock()
	defer i.store.Unlock()
	path := naming.Join("/acls", name, "data")

	perms, version, err = getPermissions(ctx, i.store, path)
	if verror.ErrorID(err) == verror.ErrNoExist.ID {
		return pathperms.NilAuthPermissions(ctx, call.Security()), "", nil
	}

	return perms, version, err
}

func (i *appRepoService) SetPermissions(ctx *context.T, _ rpc.ServerCall, perms access.Permissions, version string) error {
	name, _, err := parse(ctx, i.suffix)
	if err != nil {
		return err
	}
	i.store.Lock()
	defer i.store.Unlock()
	path := naming.Join("/acls", name, "data")
	return setPermissions(ctx, i.store, path, perms, version)
}

// getPermissions fetches a Permissions out of the Memstore at the provided path.
// path is expected to already have been cleaned by naming.Join or its ilk.
func getPermissions(ctx *context.T, store *fs.Memstore, path string) (access.Permissions, string, error) {
	entry, err := store.BindObject(path).Get(nil)

	if verror.ErrorID(err) == fs.ErrNotInMemStore.ID {
		// No Permissions exists
		return nil, "", verror.New(verror.ErrNoExist, nil)
	} else if err != nil {
		ctx.Errorf("getPermissions: internal failure in fs.Memstore")
		return nil, "", err
	}

	perms, ok := entry.Value.(access.Permissions)
	if !ok {
		return nil, "", err
	}

	version, err := pathperms.ComputeVersion(perms)
	if err != nil {
		return nil, "", err
	}
	return perms, version, nil
}

// setPermissions writes a Permissions into the Memstore at the provided path.
// where path is expected to have already been cleaned by naming.Join.
func setPermissions(ctx *context.T, store *fs.Memstore, path string, perms access.Permissions, version string) error {
	if version != "" {
		_, oversion, err := getPermissions(ctx, store, path)
		if verror.ErrorID(err) == verror.ErrNoExist.ID {
			oversion = version
		} else if err != nil {
			return err
		}

		if oversion != version {
			return verror.NewErrBadVersion(nil)
		}
	}

	tname, err := store.BindTransactionRoot("").CreateTransaction(nil)
	if err != nil {
		return err
	}

	object := store.BindObject(path)

	if _, err := object.Put(nil, perms); err != nil {
		return err
	}
	if err := store.BindTransaction(tname).Commit(nil); err != nil {
		return verror.New(ErrOperationFailed, nil)
	}
	return nil
}

func (i *appRepoService) TidyNow(ctx *context.T, call rpc.ServerCall) error {
	return fmt.Errorf("method not implemented")
}
