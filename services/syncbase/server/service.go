// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package server

// TODO(sadovsky): Check Resolve access on parent where applicable. Relatedly,
// convert ErrNoExist and ErrNoAccess to ErrNoExistOrNoAccess where needed to
// preserve privacy.

import (
	"path"
	"sync"

	"v.io/v23/context"
	"v.io/v23/glob"
	"v.io/v23/rpc"
	"v.io/v23/security/access"
	wire "v.io/v23/services/syncbase"
	"v.io/v23/verror"
	"v.io/v23/vom"
	"v.io/x/ref/services/syncbase/server/interfaces"
	"v.io/x/ref/services/syncbase/server/nosql"
	"v.io/x/ref/services/syncbase/server/util"
	"v.io/x/ref/services/syncbase/store"
	"v.io/x/ref/services/syncbase/vsync"
)

// service is a singleton (i.e. not per-request) that handles Service RPCs.
type service struct {
	st   store.Store // keeps track of which apps and databases exist, etc.
	sync interfaces.SyncServerMethods
	opts ServiceOptions
	// Guards the fields below. Held during app Create, Delete, and
	// SetPermissions.
	mu   sync.Mutex
	apps map[string]*app
}

var (
	_ wire.ServiceServerMethods = (*service)(nil)
	_ interfaces.Service        = (*service)(nil)
)

// ServiceOptions configures a service.
type ServiceOptions struct {
	// Service-level permissions.
	Perms access.Permissions
	// Root dir for data storage.
	RootDir string
	// Storage engine to use (for service and per-database engines).
	Engine string
}

// NewService creates a new service instance and returns it.
// TODO(sadovsky): If possible, close all stores when the server is stopped.
func NewService(ctx *context.T, call rpc.ServerCall, opts ServiceOptions) (*service, error) {
	if opts.Perms == nil {
		return nil, verror.New(verror.ErrInternal, ctx, "perms must be specified")
	}
	st, err := util.OpenStore(opts.Engine, path.Join(opts.RootDir, opts.Engine), util.OpenOptions{CreateIfMissing: true, ErrorIfExists: false})
	if err != nil {
		return nil, err
	}
	s := &service{
		st:   st,
		opts: opts,
		apps: map[string]*app{},
	}
	data := &serviceData{
		Perms: opts.Perms,
	}
	if err := util.Get(ctx, st, s.stKey(), &serviceData{}); verror.ErrorID(err) != verror.ErrNoExist.ID {
		if err != nil {
			return nil, err
		}
		// Service exists. Initialize in-memory data structures.
		// Read all apps, populate apps map.
		aIt := st.Scan(util.ScanPrefixArgs(util.AppPrefix, ""))
		aBytes := []byte{}
		for aIt.Advance() {
			aBytes = aIt.Value(aBytes)
			aData := &appData{}
			if err := vom.Decode(aBytes, aData); err != nil {
				return nil, verror.New(verror.ErrInternal, ctx, err)
			}
			a := &app{
				name:   aData.Name,
				s:      s,
				exists: true,
				dbs:    make(map[string]interfaces.Database),
			}
			s.apps[a.name] = a
			// Read all dbs for this app, populate dbs map.
			dIt := st.Scan(util.ScanPrefixArgs(util.JoinKeyParts(util.DbInfoPrefix, aData.Name), ""))
			dBytes := []byte{}
			for dIt.Advance() {
				dBytes = dIt.Value(dBytes)
				info := &dbInfo{}
				if err := vom.Decode(dBytes, info); err != nil {
					return nil, verror.New(verror.ErrInternal, ctx, err)
				}
				d, err := nosql.OpenDatabase(ctx, a, info.Name, nosql.DatabaseOptions{
					RootDir: info.RootDir,
					Engine:  info.Engine,
				}, util.OpenOptions{
					CreateIfMissing: false,
					ErrorIfExists:   false,
				})
				if err != nil {
					return nil, verror.New(verror.ErrInternal, ctx, err)
				}
				a.dbs[info.Name] = d
			}
			if err := dIt.Err(); err != nil {
				return nil, verror.New(verror.ErrInternal, ctx, err)
			}
		}
		if err := aIt.Err(); err != nil {
			return nil, verror.New(verror.ErrInternal, ctx, err)
		}
	} else {
		// Service does not exist.
		if err := util.Put(ctx, st, s.stKey(), data); err != nil {
			return nil, err
		}
	}
	// Note, vsync.New internally handles both first-time and subsequent
	// invocations.
	if s.sync, err = vsync.New(ctx, call, s, opts.Engine, opts.RootDir); err != nil {
		return nil, err
	}
	return s, nil
}

////////////////////////////////////////
// RPC methods

func (s *service) SetPermissions(ctx *context.T, call rpc.ServerCall, perms access.Permissions, version string) error {
	return store.RunInTransaction(s.st, func(tx store.Transaction) error {
		data := &serviceData{}
		return util.UpdateWithAuth(ctx, call, tx, s.stKey(), data, func() error {
			if err := util.CheckVersion(ctx, version, data.Version); err != nil {
				return err
			}
			data.Perms = perms
			data.Version++
			return nil
		})
	})
}

func (s *service) GetPermissions(ctx *context.T, call rpc.ServerCall) (perms access.Permissions, version string, err error) {
	data := &serviceData{}
	if err := util.GetWithAuth(ctx, call, s.st, s.stKey(), data); err != nil {
		return nil, "", err
	}
	return data.Perms, util.FormatVersion(data.Version), nil
}

func (s *service) GlobChildren__(ctx *context.T, call rpc.GlobChildrenServerCall, matcher *glob.Element) error {
	// Check perms.
	sn := s.st.NewSnapshot()
	defer sn.Abort()
	if err := util.GetWithAuth(ctx, call, sn, s.stKey(), &serviceData{}); err != nil {
		return err
	}
	return util.GlobChildren(ctx, call, matcher, sn, util.AppPrefix)
}

////////////////////////////////////////
// interfaces.Service methods

func (s *service) St() store.Store {
	return s.st
}

func (s *service) Sync() interfaces.SyncServerMethods {
	return s.sync
}

func (s *service) App(ctx *context.T, call rpc.ServerCall, appName string) (interfaces.App, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Note, currently the service's apps map as well as per-app dbs maps are
	// populated at startup.
	a, ok := s.apps[appName]
	if !ok {
		return nil, verror.New(verror.ErrNoExist, ctx, appName)
	}
	return a, nil
}

func (s *service) AppNames(ctx *context.T, call rpc.ServerCall) ([]string, error) {
	// In the future this API will likely be replaced by one that streams the app
	// names.
	s.mu.Lock()
	defer s.mu.Unlock()
	appNames := make([]string, 0, len(s.apps))
	for n := range s.apps {
		appNames = append(appNames, n)
	}
	return appNames, nil
}

////////////////////////////////////////
// App management methods

func (s *service) createApp(ctx *context.T, call rpc.ServerCall, appName string, perms access.Permissions) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.apps[appName]; ok {
		return verror.New(verror.ErrExist, ctx, appName)
	}

	a := &app{
		name:   appName,
		s:      s,
		exists: true,
		dbs:    make(map[string]interfaces.Database),
	}

	if err := store.RunInTransaction(s.st, func(tx store.Transaction) error {
		// Check serviceData perms.
		sData := &serviceData{}
		if err := util.GetWithAuth(ctx, call, tx, s.stKey(), sData); err != nil {
			return err
		}
		// Check for "app already exists".
		if err := util.Get(ctx, tx, a.stKey(), &appData{}); verror.ErrorID(err) != verror.ErrNoExist.ID {
			if err != nil {
				return err
			}
			return verror.New(verror.ErrExist, ctx, appName)
		}
		// Write new appData.
		if perms == nil {
			perms = sData.Perms
		}
		data := &appData{
			Name:  appName,
			Perms: perms,
		}
		return util.Put(ctx, tx, a.stKey(), data)
	}); err != nil {
		return err
	}

	s.apps[appName] = a
	return nil
}

func (s *service) destroyApp(ctx *context.T, call rpc.ServerCall, appName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.apps[appName]
	if !ok {
		return nil // destroy is idempotent
	}

	if err := store.RunInTransaction(s.st, func(tx store.Transaction) error {
		// Read-check-delete appData.
		if err := util.GetWithAuth(ctx, call, tx, a.stKey(), &appData{}); err != nil {
			if verror.ErrorID(err) == verror.ErrNoExist.ID {
				return nil // destroy is idempotent
			}
			return err
		}
		// TODO(sadovsky): Delete all databases in this app.
		return util.Delete(ctx, tx, a.stKey())
	}); err != nil {
		return err
	}

	delete(s.apps, appName)
	return nil
}

func (s *service) setAppPerms(ctx *context.T, call rpc.ServerCall, appName string, perms access.Permissions, version string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.apps[appName]
	if !ok {
		return verror.New(verror.ErrNoExist, ctx, appName)
	}
	return store.RunInTransaction(s.st, func(tx store.Transaction) error {
		data := &appData{}
		return util.UpdateWithAuth(ctx, call, tx, a.stKey(), data, func() error {
			if err := util.CheckVersion(ctx, version, data.Version); err != nil {
				return err
			}
			data.Perms = perms
			data.Version++
			return nil
		})
	})
}

////////////////////////////////////////
// Other internal helpers

func (s *service) stKey() string {
	return util.ServicePrefix
}
