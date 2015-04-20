// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package server

import (
	"v.io/v23/context"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/v23/security/access"
	"v.io/v23/services/groups"
	"v.io/v23/verror"
)

type group struct {
	name string
	m    *manager
}

var _ groups.GroupServerMethods = (*group)(nil)

// TODO(sadovsky): verror.NewErrFoo(...) methods do not accept extra arguments,
// so in various places below we call verror.New(verror.ErrFoo, ...) instead.

// TODO(sadovsky): What prevents "mike" from creating group "adam:AllMyDevices"?
// It seems we need either (a) identity providers to manage group servers and
// reserve buckets for users they've blessed, or (b) some way to determine the
// user name from a blessing and enforce that group names start with user names.
func (g *group) Create(ctx *context.T, call rpc.ServerCall, perms access.Permissions, entries []groups.BlessingPatternChunk) error {
	// Perform Permissions check.
	// TODO(sadovsky): Enable this Permissions check and acquire a lock on the
	// group server Permissions.
	if false {
		if err := g.authorize(ctx, call.Security(), g.m.perms); err != nil {
			return err
		}
	}
	if perms == nil {
		perms = access.Permissions{}
		blessings, _ := security.RemoteBlessingNames(ctx, call.Security())
		if len(blessings) == 0 {
			// The blessings presented by the caller do not give it a name for this
			// operation. We could create a world-accessible group, but it seems safer
			// to return an error.
			return groups.NewErrNoBlessings(ctx)
		}
		for _, tag := range access.AllTypicalTags() {
			for _, b := range blessings {
				perms.Add(security.BlessingPattern(b), string(tag))
			}
		}
	}
	entrySet := map[groups.BlessingPatternChunk]struct{}{}
	for _, v := range entries {
		entrySet[v] = struct{}{}
	}
	gd := groupData{Perms: perms, Entries: entrySet}
	if err := g.m.st.Insert(g.name, gd); err != nil {
		// TODO(sadovsky): We are leaking the fact that this group exists. If the
		// client doesn't have access to this group, we should probably return an
		// opaque error. (That's not a great solution either. Having per-user
		// buckets seems like a better solution.)
		if _, ok := err.(*ErrKeyAlreadyExists); ok {
			return verror.New(verror.ErrExist, ctx, g.name)
		}
		return verror.New(verror.ErrInternal, ctx, err)
	}
	return nil
}

func (g *group) Delete(ctx *context.T, call rpc.ServerCall, version string) error {
	return g.readModifyWrite(ctx, call.Security(), version, func(gd *groupData, versionSt string) error {
		return g.m.st.Delete(g.name, versionSt)
	})
}

func (g *group) Add(ctx *context.T, call rpc.ServerCall, entry groups.BlessingPatternChunk, version string) error {
	return g.update(ctx, call.Security(), version, func(gd *groupData) {
		gd.Entries[entry] = struct{}{}
	})
}

func (g *group) Remove(ctx *context.T, call rpc.ServerCall, entry groups.BlessingPatternChunk, version string) error {
	return g.update(ctx, call.Security(), version, func(gd *groupData) {
		delete(gd.Entries, entry)
	})
}

// TODO(sadovsky): Replace fake implementation with real implementation.
func (g *group) Get(ctx *context.T, call rpc.ServerCall, req groups.GetRequest, reqVersion string) (res groups.GetResponse, version string, err error) {
	gd, version, err := g.getInternal(ctx, call.Security())
	if err != nil {
		return groups.GetResponse{}, "", err
	}
	return groups.GetResponse{Entries: gd.Entries}, version, nil
}

// TODO(sadovsky): Replace fake implementation with real implementation.
func (g *group) Rest(ctx *context.T, call rpc.ServerCall, req groups.RestRequest, reqVersion string) (res groups.RestResponse, version string, err error) {
	_, version, err = g.getInternal(ctx, call.Security())
	if err != nil {
		return groups.RestResponse{}, "", err
	}
	return groups.RestResponse{}, version, nil
}

func (g *group) SetPermissions(ctx *context.T, call rpc.ServerCall, perms access.Permissions, version string) error {
	return g.update(ctx, call.Security(), version, func(gd *groupData) {
		gd.Perms = perms
	})
}

func (g *group) GetPermissions(ctx *context.T, call rpc.ServerCall) (perms access.Permissions, version string, err error) {
	gd, version, err := g.getInternal(ctx, call.Security())
	if err != nil {
		return nil, "", err
	}
	return gd.Perms, version, nil
}

////////////////////////////////////////
// Internal helpers

// Returns a VDL-compatible error.
func (g *group) authorize(ctx *context.T, call security.Call, perms access.Permissions) error {
	// TODO(sadovsky): We ignore the returned error since TypicalTagType is
	// guaranteed to return a valid tagType. It would be nice to have an
	// alternative function that assumes TypicalTagType, since presumably that's
	// the overwhelmingly common case.
	auth, _ := access.PermissionsAuthorizer(perms, access.TypicalTagType())
	if err := auth.Authorize(ctx, call); err != nil {
		// TODO(sadovsky): Return NoAccess if appropriate.
		return verror.New(verror.ErrNoExistOrNoAccess, ctx, err)
	}
	return nil
}

// Returns a VDL-compatible error. Performs access check.
func (g *group) getInternal(ctx *context.T, call security.Call) (gd groupData, version string, err error) {
	v, version, err := g.m.st.Get(g.name)
	if err != nil {
		if _, ok := err.(*ErrUnknownKey); ok {
			// TODO(sadovsky): Return NoExist if appropriate.
			return groupData{}, "", verror.New(verror.ErrNoExistOrNoAccess, ctx, g.name)
		}
		return groupData{}, "", verror.New(verror.ErrInternal, ctx, err)
	}
	gd, ok := v.(groupData)
	if !ok {
		return groupData{}, "", verror.New(verror.ErrInternal, ctx, "bad value for key: "+g.name)
	}
	if err := g.authorize(ctx, call, gd.Perms); err != nil {
		return groupData{}, "", err
	}
	return gd, version, nil
}

// Returns a VDL-compatible error. Performs access check.
func (g *group) update(ctx *context.T, call security.Call, version string, fn func(gd *groupData)) error {
	return g.readModifyWrite(ctx, call, version, func(gd *groupData, versionSt string) error {
		fn(gd)
		return g.m.st.Update(g.name, *gd, versionSt)
	})
}

// Returns a VDL-compatible error. Performs access check.
// fn should perform the "modify, write" part of "read, modify, write", and
// should return a Store error.
func (g *group) readModifyWrite(ctx *context.T, call security.Call, version string, fn func(gd *groupData, versionSt string) error) error {
	// Transaction retry loop.
	for i := 0; i < 3; i++ {
		gd, versionSt, err := g.getInternal(ctx, call)
		if err != nil {
			return err
		}
		// Fail early if possible.
		if version != "" && version != versionSt {
			return verror.NewErrBadVersion(ctx)
		}
		if err := fn(&gd, versionSt); err != nil {
			if err, ok := err.(*ErrBadVersion); ok {
				// Retry on version error if the original version was empty.
				if version != "" {
					return verror.NewErrBadVersion(ctx)
				}
			} else {
				// Abort on non-version error.
				return verror.New(verror.ErrInternal, ctx, err)
			}
		} else {
			return nil
		}
	}
	return groups.NewErrExcessiveContention(ctx)
}
