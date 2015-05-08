// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package pathperms provides a library to assist servers implementing
// GetPermissions/SetPermissions functions and authorizers where there are
// path-specific Permissions stored individually in files.
// TODO(rjkroege): Add unit tests.
package pathperms

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"

	"v.io/v23/context"
	"v.io/v23/security"
	"v.io/v23/security/access"
	"v.io/v23/verror"
	"v.io/x/lib/vlog"
	"v.io/x/ref/lib/security/serialization"
)

const (
	pkgPath   = "v.io/x/ref/services/internal/pathperms"
	sigName   = "signature"
	permsName = "data"
)

var (
	ErrOperationFailed = verror.Register(pkgPath+".OperationFailed", verror.NoRetry, "{1:}{2:} operation failed{:_}")
)

// PathStore manages storage of a set of Permissions in the filesystem where each
// path identifies a specific Permissions in the set. PathStore synchronizes
// access to its member Permissions.
type PathStore struct {
	// TODO(rjkroege): Garbage collect the locks map.
	pthlks    map[string]*sync.Mutex
	lk        sync.Mutex
	principal security.Principal
}

// NewPathStore creates a new instance of the lock map that uses
// principal to sign stored Permissions files.
func NewPathStore(principal security.Principal) *PathStore {
	return &PathStore{pthlks: make(map[string]*sync.Mutex), principal: principal}
}

// Get returns the Permissions from the data file in dir.
func (store *PathStore) Get(dir string) (access.Permissions, string, error) {
	permspath := filepath.Join(dir, permsName)
	sigpath := filepath.Join(dir, sigName)
	defer store.lockPath(dir)()
	return getCore(store.principal, permspath, sigpath)
}

// TODO(rjkroege): Improve lock handling.
func (store *PathStore) lockPath(dir string) func() {
	store.lk.Lock()
	lck, contains := store.pthlks[dir]
	if !contains {
		lck = new(sync.Mutex)
		store.pthlks[dir] = lck
	}
	store.lk.Unlock()
	lck.Lock()
	return lck.Unlock
}

func getCore(principal security.Principal, permspath, sigpath string) (access.Permissions, string, error) {
	f, err := os.Open(permspath)
	if err != nil {
		// This path is rarely a fatal error so log informationally only.
		vlog.VI(2).Infof("os.Open(%s) failed: %v", permspath, err)
		return nil, "", err
	}
	defer f.Close()

	s, err := os.Open(sigpath)
	if err != nil {
		vlog.Errorf("Signatures for Permissions are required: %s unavailable: %v", permspath, err)
		return nil, "", verror.New(ErrOperationFailed, nil)
	}
	defer s.Close()

	// read and verify the signature of the perms file
	vf, err := serialization.NewVerifyingReader(f, s, principal.PublicKey())
	if err != nil {
		vlog.Errorf("NewVerifyingReader() failed: %v (perms=%s, sig=%s)", err, permspath, sigpath)
		return nil, "", verror.New(ErrOperationFailed, nil)
	}

	perms, err := access.ReadPermissions(vf)
	if err != nil {
		vlog.Errorf("ReadPermissions(%s) failed: %v", permspath, err)
		return nil, "", err
	}
	version, err := ComputeVersion(perms)
	if err != nil {
		vlog.Errorf("pathperms.ComputeVersion failed: %v", err)
		return nil, "", err
	}
	return perms, version, nil
}

// Set writes the specified Permissions to the provided directory with
// enforcement of version synchronization mechanism and locking.
func (store *PathStore) Set(dir string, perms access.Permissions, version string) error {
	return store.SetShareable(dir, perms, version, false)
}

// SetShareable writes the specified Permissions to the provided
// directory with enforcement of version synchronization mechanism and
// locking with file modes that will give the application read-only
// access to the permissions file.
func (store *PathStore) SetShareable(dir string, perms access.Permissions, version string, shareable bool) error {
	permspath := filepath.Join(dir, permsName)
	sigpath := filepath.Join(dir, sigName)
	defer store.lockPath(dir)()
	_, oversion, err := getCore(store.principal, permspath, sigpath)
	if err != nil && !os.IsNotExist(err) {
		return verror.New(ErrOperationFailed, nil)
	}
	if len(version) > 0 && version != oversion {
		return verror.NewErrBadVersion(nil)
	}
	return write(store.principal, permspath, sigpath, dir, perms, shareable)
}

// write writes the specified Permissions to the permsFile with a
// signature in sigFile.
func write(principal security.Principal, permsFile, sigFile, dir string, perms access.Permissions, shareable bool) error {
	filemode := os.FileMode(0600)
	dirmode := os.FileMode(0700)
	if shareable {
		filemode = os.FileMode(0644)
		dirmode = os.FileMode(0711)
	}

	// Create dir directory if it does not exist
	os.MkdirAll(dir, dirmode)
	// Save the object to temporary data and signature files, and then move
	// those files to the actual data and signature file.
	data, err := ioutil.TempFile(dir, permsName)
	if err != nil {
		vlog.Errorf("Failed to open tmpfile data:%v", err)
		return verror.New(ErrOperationFailed, nil)
	}
	defer os.Remove(data.Name())
	sig, err := ioutil.TempFile(dir, sigName)
	if err != nil {
		vlog.Errorf("Failed to open tmpfile sig:%v", err)
		return verror.New(ErrOperationFailed, nil)
	}
	defer os.Remove(sig.Name())
	writer, err := serialization.NewSigningWriteCloser(data, sig, principal, nil)
	if err != nil {
		vlog.Errorf("Failed to create NewSigningWriteCloser:%v", err)
		return verror.New(ErrOperationFailed, nil)
	}
	if err = perms.WriteTo(writer); err != nil {
		vlog.Errorf("Failed to SavePermissions:%v", err)
		return verror.New(ErrOperationFailed, nil)
	}
	if err = writer.Close(); err != nil {
		vlog.Errorf("Failed to Close() SigningWriteCloser:%v", err)
		return verror.New(ErrOperationFailed, nil)
	}
	if err := os.Rename(data.Name(), permsFile); err != nil {
		vlog.Errorf("os.Rename() failed:%v", err)
		return verror.New(ErrOperationFailed, nil)
	}
	if err := os.Chmod(permsFile, filemode); err != nil {
		vlog.Errorf("os.Chmod() failed:%v", err)
		return verror.New(ErrOperationFailed, nil)
	}
	if err := os.Rename(sig.Name(), sigFile); err != nil {
		vlog.Errorf("os.Rename() failed:%v", err)
		return verror.New(ErrOperationFailed, nil)
	}
	if err := os.Chmod(sigFile, filemode); err != nil {
		vlog.Errorf("os.Chmod() failed:%v", err)
		return verror.New(ErrOperationFailed, nil)
	}
	return nil
}

func (store *PathStore) PermsForPath(path string) (access.Permissions, bool, error) {
	perms, _, err := store.Get(path)
	if os.IsNotExist(err) {
		return nil, true, nil
	} else if err != nil {
		return nil, false, err
	}
	return perms, false, nil
}

// PrefixPatterns creates a pattern containing all of the prefix patterns of the
// provided blessings.
func PrefixPatterns(blessings []string) []security.BlessingPattern {
	var patterns []security.BlessingPattern
	for _, b := range blessings {
		patterns = append(patterns, security.BlessingPattern(b).PrefixPatterns()...)
	}
	return patterns
}

// PermissionsForBlessings creates the Permissions list that should be used with
// a newly created object.
func PermissionsForBlessings(blessings []string) access.Permissions {
	perms := make(access.Permissions)

	// Add the invoker's blessings and all its prefixes.
	for _, p := range PrefixPatterns(blessings) {
		for _, tag := range access.AllTypicalTags() {
			perms.Add(p, string(tag))
		}
	}
	return perms
}

// NilAuthPermissions creates Permissions that mimics the default authorization
// policy (i.e., Permissions is matched by all blessings that are either
// extensions of one of the local blessings or can be extended to form one of
// the local blessings.)
func NilAuthPermissions(ctx *context.T, call security.Call) access.Permissions {
	perms := make(access.Permissions)
	lb := security.LocalBlessingNames(ctx, call)
	for _, p := range PrefixPatterns(lb) {
		for _, tag := range access.AllTypicalTags() {
			perms.Add(p, string(tag))
		}
	}
	return perms
}
