// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The implementation of the binary repository interface stores objects
// identified by object name suffixes using the local file system. Given
// an object name suffix, the implementation computes an MD5 hash of the
// suffix and generates the following path in the local filesystem:
// /<root_dir>/<dir_1>/.../<dir_n>/<hash>. The root directory and the
// directory depth are parameters of the implementation. <root_dir> also
// contains __acls/data and __acls/sig files storing the AccessLists for the
// root level. The contents of the directory include the checksum and
// data for each of the individual parts of the binary, the name of the
// object and a directory containing the acls for this particular object:
//
// name
// acls/data
// acls/sig
// mediainfo
// name
// <part_1>/checksum
// <part_1>/data
// ...
// <part_n>/checksum
// <part_n>/data
//
// TODO(jsimsa): Add an "fsck" method that cleans up existing on-disk
// repository and provide a command-line flag that identifies whether
// fsck should run when new repository server process starts up.
package impl

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/v23/services/mgmt/binary"
	"v.io/v23/services/mgmt/repository"
	"v.io/v23/services/security/access"
	"v.io/v23/verror"
	"v.io/x/lib/vlog"
	"v.io/x/ref/services/mgmt/lib/acls"
)

// binaryService implements the Binary server interface.
type binaryService struct {
	// path is the local filesystem path to the object identified by the
	// object name suffix.
	path string
	// state holds the state shared across different binary repository
	// invocations.
	state *state
	// suffix is the name of the binary object.
	suffix   string
	aclstore *acls.PathStore
}

const pkgPath = "v.io/x/ref/services/mgmt/binary/impl"

var (
	ErrInProgress      = verror.Register(pkgPath+".errInProgress", verror.NoRetry, "{1:}{2:} identical upload already in progress{:_}")
	ErrInvalidParts    = verror.Register(pkgPath+".errInvalidParts", verror.NoRetry, "{1:}{2:} invalid number of binary parts{:_}")
	ErrInvalidPart     = verror.Register(pkgPath+".errInvalidPart", verror.NoRetry, "{1:}{2:} invalid binary part number{:_}")
	ErrOperationFailed = verror.Register(pkgPath+".errOperationFailed", verror.NoRetry, "{1:}{2:} operation failed{:_}")
	ErrNotAuthorized   = verror.Register(pkgPath+".errNotAuthorized", verror.NoRetry, "{1:}{2:} none of the client's blessings are valid {:_}")
)

// TODO(jsimsa): When VDL supports composite literal constants, remove
// this definition.
var MissingPart = binary.PartInfo{
	Checksum: binary.MissingChecksum,
	Size:     binary.MissingSize,
}

// newBinaryService returns a new Binary service implementation.
func newBinaryService(state *state, suffix string, aclstore *acls.PathStore) *binaryService {
	return &binaryService{
		path:     state.dir(suffix),
		state:    state,
		suffix:   suffix,
		aclstore: aclstore,
	}
}

const BufferLength = 4096

func prefixPatterns(blessings []string) []security.BlessingPattern {
	var patterns []security.BlessingPattern
	for _, b := range blessings {
		patterns = append(patterns, security.BlessingPattern(b).PrefixPatterns()...)
	}
	return patterns
}

// insertAccessLists configures the starting AccessList set for a newly "Create"-d binary based
// on the caller's blessings.
func insertAccessLists(dir string, aclstore *acls.PathStore, blessings []string) error {
	tam := make(access.Permissions)

	// Add the invoker's blessings and all its prefixes.
	for _, p := range prefixPatterns(blessings) {
		for _, tag := range access.AllTypicalTags() {
			tam.Add(p, string(tag))
		}
	}
	return aclstore.Set(dir, tam, "")
}

func (i *binaryService) Create(call rpc.ServerCall, nparts int32, mediaInfo repository.MediaInfo) error {
	vlog.Infof("%v.Create(%v, %v)", i.suffix, nparts, mediaInfo)
	if nparts < 1 {
		return verror.New(ErrInvalidParts, call.Context())
	}
	parent, perm := filepath.Dir(i.path), os.FileMode(0700)
	if err := os.MkdirAll(parent, perm); err != nil {
		vlog.Errorf("MkdirAll(%v, %v) failed: %v", parent, perm, err)
		return verror.New(ErrOperationFailed, call.Context())
	}
	prefix := "creating-"
	tmpDir, err := ioutil.TempDir(parent, prefix)
	if err != nil {
		vlog.Errorf("TempDir(%v, %v) failed: %v", parent, prefix, err)
		return verror.New(ErrOperationFailed, call.Context())
	}
	nameFile := filepath.Join(tmpDir, nameFileName)
	if err := ioutil.WriteFile(nameFile, []byte(i.suffix), os.FileMode(0600)); err != nil {
		vlog.Errorf("WriteFile(%q) failed: %v", nameFile)
		return verror.New(ErrOperationFailed, call.Context())
	}

	rb, _ := security.RemoteBlessingNames(call.Context())
	if len(rb) == 0 {
		// None of the client's blessings are valid.
		return verror.New(ErrNotAuthorized, call.Context())
	}
	if err := insertAccessLists(aclPath(i.state.rootDir, i.suffix), i.aclstore, rb); err != nil {
		vlog.Errorf("insertAccessLists(%v) failed: %v", rb, err)
		return verror.New(ErrOperationFailed, call.Context())
	}

	infoFile := filepath.Join(tmpDir, mediaInfoFileName)
	jInfo, err := json.Marshal(mediaInfo)
	if err != nil {
		vlog.Errorf("json.Marshal(%v) failed: %v", mediaInfo, err)
		return verror.New(ErrOperationFailed, call.Context())
	}
	if err := ioutil.WriteFile(infoFile, jInfo, os.FileMode(0600)); err != nil {
		vlog.Errorf("WriteFile(%q) failed: %v", infoFile, err)
		return verror.New(ErrOperationFailed, call.Context())
	}
	for j := 0; j < int(nparts); j++ {
		partPath, partPerm := generatePartPath(tmpDir, j), os.FileMode(0700)
		if err := os.MkdirAll(partPath, partPerm); err != nil {
			vlog.Errorf("MkdirAll(%v, %v) failed: %v", partPath, partPerm, err)
			if err := os.RemoveAll(tmpDir); err != nil {
				vlog.Errorf("RemoveAll(%v) failed: %v", tmpDir, err)
			}
			return verror.New(ErrOperationFailed, call.Context())
		}
	}
	// Use os.Rename() to atomically create the binary directory
	// structure.
	if err := os.Rename(tmpDir, i.path); err != nil {
		defer func() {
			if err := os.RemoveAll(tmpDir); err != nil {
				vlog.Errorf("RemoveAll(%v) failed: %v", tmpDir, err)
			}
		}()
		if linkErr, ok := err.(*os.LinkError); ok && linkErr.Err == syscall.ENOTEMPTY {
			return verror.New(verror.ErrExist, call.Context(), i.path)
		}
		vlog.Errorf("Rename(%v, %v) failed: %v", tmpDir, i.path, err)
		return verror.New(ErrOperationFailed, call.Context(), i.path)
	}
	return nil
}

func (i *binaryService) Delete(call rpc.ServerCall) error {
	vlog.Infof("%v.Delete()", i.suffix)
	if _, err := os.Stat(i.path); err != nil {
		if os.IsNotExist(err) {
			return verror.New(verror.ErrNoExist, call.Context(), i.path)
		}
		vlog.Errorf("Stat(%v) failed: %v", i.path, err)
		return verror.New(ErrOperationFailed, call.Context())
	}
	// Use os.Rename() to atomically remove the binary directory
	// structure.
	path := filepath.Join(filepath.Dir(i.path), "removing-"+filepath.Base(i.path))
	if err := os.Rename(i.path, path); err != nil {
		vlog.Errorf("Rename(%v, %v) failed: %v", i.path, path, err)
		return verror.New(ErrOperationFailed, call.Context(), i.path)
	}
	if err := os.RemoveAll(path); err != nil {
		vlog.Errorf("Remove(%v) failed: %v", path, err)
		return verror.New(ErrOperationFailed, call.Context())
	}
	for {
		// Remove the binary and all directories on the path back to the
		// root directory that are left empty after the binary is removed.
		path = filepath.Dir(path)
		if i.state.rootDir == path {
			break
		}
		if err := os.Remove(path); err != nil {
			if err.(*os.PathError).Err.Error() == syscall.ENOTEMPTY.Error() {
				break
			}
			vlog.Errorf("Remove(%v) failed: %v", path, err)
			return verror.New(ErrOperationFailed, call.Context())
		}
	}
	return nil
}

func (i *binaryService) Download(call repository.BinaryDownloadServerCall, part int32) error {
	vlog.Infof("%v.Download(%v)", i.suffix, part)
	path := i.generatePartPath(int(part))
	if err := checksumExists(path); err != nil {
		return err
	}
	dataPath := filepath.Join(path, dataFileName)
	file, err := os.Open(dataPath)
	if err != nil {
		vlog.Errorf("Open(%v) failed: %v", dataPath, err)
		return verror.New(ErrOperationFailed, call.Context())
	}
	defer file.Close()
	buffer := make([]byte, BufferLength)
	sender := call.SendStream()
	for {
		n, err := file.Read(buffer)
		if err != nil && err != io.EOF {
			vlog.Errorf("Read() failed: %v", err)
			return verror.New(ErrOperationFailed, call.Context())
		}
		if n == 0 {
			break
		}
		if err := sender.Send(buffer[:n]); err != nil {
			vlog.Errorf("Send() failed: %v", err)
			return verror.New(ErrOperationFailed, call.Context())
		}
	}
	return nil
}

// TODO(jsimsa): Design and implement an access control mechanism for
// the URL-based downloads.
func (i *binaryService) DownloadUrl(rpc.ServerCall) (string, int64, error) {
	vlog.Infof("%v.DownloadUrl()", i.suffix)
	return i.state.rootURL + "/" + i.suffix, 0, nil
}

func (i *binaryService) Stat(call rpc.ServerCall) ([]binary.PartInfo, repository.MediaInfo, error) {
	vlog.Infof("%v.Stat()", i.suffix)
	result := make([]binary.PartInfo, 0)
	parts, err := getParts(i.path)
	if err != nil {
		return []binary.PartInfo{}, repository.MediaInfo{}, err
	}
	for _, part := range parts {
		checksumFile := filepath.Join(part, checksumFileName)
		bytes, err := ioutil.ReadFile(checksumFile)
		if err != nil {
			if os.IsNotExist(err) {
				result = append(result, MissingPart)
				continue
			}
			vlog.Errorf("ReadFile(%v) failed: %v", checksumFile, err)
			return []binary.PartInfo{}, repository.MediaInfo{}, verror.New(ErrOperationFailed, call.Context())
		}
		dataFile := filepath.Join(part, dataFileName)
		fi, err := os.Stat(dataFile)
		if err != nil {
			if os.IsNotExist(err) {
				result = append(result, MissingPart)
				continue
			}
			vlog.Errorf("Stat(%v) failed: %v", dataFile, err)
			return []binary.PartInfo{}, repository.MediaInfo{}, verror.New(ErrOperationFailed, call.Context())
		}
		result = append(result, binary.PartInfo{Checksum: string(bytes), Size: fi.Size()})
	}
	infoFile := filepath.Join(i.path, mediaInfoFileName)
	jInfo, err := ioutil.ReadFile(infoFile)
	if err != nil {
		vlog.Errorf("ReadFile(%q) failed: %v", infoFile)
		return []binary.PartInfo{}, repository.MediaInfo{}, verror.New(ErrOperationFailed, call.Context())
	}
	var mediaInfo repository.MediaInfo
	if err := json.Unmarshal(jInfo, &mediaInfo); err != nil {
		vlog.Errorf("json.Unmarshal(%v) failed: %v", jInfo, err)
		return []binary.PartInfo{}, repository.MediaInfo{}, verror.New(ErrOperationFailed, call.Context())
	}
	return result, mediaInfo, nil
}

func (i *binaryService) Upload(call repository.BinaryUploadServerCall, part int32) error {
	vlog.Infof("%v.Upload(%v)", i.suffix, part)
	path, suffix := i.generatePartPath(int(part)), ""
	err := checksumExists(path)
	if err == nil {
		return verror.New(verror.ErrExist, call.Context(), path)
	} else if !verror.Is(err, verror.ErrNoExist.ID) {
		return err
	}
	// Use os.OpenFile() to resolve races.
	lockPath, flags, perm := filepath.Join(path, lockFileName), os.O_CREATE|os.O_WRONLY|os.O_EXCL, os.FileMode(0600)
	lockFile, err := os.OpenFile(lockPath, flags, perm)
	if err != nil {
		if os.IsExist(err) {
			return verror.New(ErrInProgress, call.Context(), path)
		}
		vlog.Errorf("OpenFile(%v, %v, %v) failed: %v", lockPath, flags, suffix, err)
		return verror.New(ErrOperationFailed, call.Context())
	}
	defer os.Remove(lockFile.Name())
	defer lockFile.Close()
	file, err := ioutil.TempFile(path, suffix)
	if err != nil {
		vlog.Errorf("TempFile(%v, %v) failed: %v", path, suffix, err)
		return verror.New(ErrOperationFailed, call.Context())
	}
	defer file.Close()
	h := md5.New()
	rStream := call.RecvStream()
	for rStream.Advance() {
		bytes := rStream.Value()
		if _, err := file.Write(bytes); err != nil {
			vlog.Errorf("Write() failed: %v", err)
			if err := os.Remove(file.Name()); err != nil {
				vlog.Errorf("Remove(%v) failed: %v", file.Name(), err)
			}
			return verror.New(ErrOperationFailed, call.Context())
		}
		h.Write(bytes)
	}

	if err := rStream.Err(); err != nil {
		vlog.Errorf("Advance() failed: %v", err)
		if err := os.Remove(file.Name()); err != nil {
			vlog.Errorf("Remove(%v) failed: %v", file.Name(), err)
		}
		return verror.New(ErrOperationFailed, call.Context())
	}

	hash := hex.EncodeToString(h.Sum(nil))
	checksumFile, perm := filepath.Join(path, checksumFileName), os.FileMode(0600)
	if err := ioutil.WriteFile(checksumFile, []byte(hash), perm); err != nil {
		vlog.Errorf("WriteFile(%v, %v, %v) failed: %v", checksumFile, hash, perm, err)
		if err := os.Remove(file.Name()); err != nil {
			vlog.Errorf("Remove(%v) failed: %v", file.Name(), err)
		}
		return verror.New(ErrOperationFailed, call.Context())
	}
	dataFile := filepath.Join(path, dataFileName)
	if err := os.Rename(file.Name(), dataFile); err != nil {
		vlog.Errorf("Rename(%v, %v) failed: %v", file.Name(), dataFile, err)
		if err := os.Remove(file.Name()); err != nil {
			vlog.Errorf("Remove(%v) failed: %v", file.Name(), err)
		}
		return verror.New(ErrOperationFailed, call.Context())
	}
	return nil
}

func (i *binaryService) GlobChildren__(call rpc.ServerCall) (<-chan string, error) {
	elems := strings.Split(i.suffix, "/")
	if len(elems) == 1 && elems[0] == "" {
		elems = nil
	}
	n := i.createObjectNameTree().find(elems, false)
	if n == nil {
		return nil, verror.New(ErrOperationFailed, call.Context())
	}
	ch := make(chan string)
	go func() {
		for k, _ := range n.children {
			ch <- k
		}
		close(ch)
	}()
	return ch, nil
}

func (i *binaryService) GetPermissions(call rpc.ServerCall) (acl access.Permissions, etag string, err error) {

	acl, etag, err = i.aclstore.Get(aclPath(i.state.rootDir, i.suffix))

	if os.IsNotExist(err) {
		// No AccessList file found which implies a nil authorizer. This results in default authorization.
		// Therefore we return an AccessList that mimics the default authorization policy (i.e., the AccessList
		// is matched by all blessings that are either extensions of one of the local blessings or
		// can be extended to form one of the local blessings.)
		tam := make(access.Permissions)

		lb := security.LocalBlessingNames(call.Context())
		for _, p := range prefixPatterns(lb) {
			for _, tag := range access.AllTypicalTags() {
				tam.Add(p, string(tag))
			}
		}
		return tam, "", nil
	}
	return acl, etag, err
}

func (i *binaryService) SetPermissions(_ rpc.ServerCall, acl access.Permissions, etag string) error {
	return i.aclstore.Set(aclPath(i.state.rootDir, i.suffix), acl, etag)
}
