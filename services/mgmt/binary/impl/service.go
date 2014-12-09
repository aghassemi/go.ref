// The implementation of the binary repository interface stores
// objects identified by object name suffixes using the local file
// system. Given an object name suffix, the implementation computes an
// MD5 hash of the suffix and generates the following path in the
// local filesystem: /<root_dir>/<dir_1>/.../<dir_n>/<hash>. The root
// directory and the directory depth are parameters of the
// implementation. The contents of the directory include the checksum
// and data for each of the individual parts of the binary, and the
// name of the object:
//
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

	"veyron.io/veyron/veyron2/ipc"
	"veyron.io/veyron/veyron2/services/mgmt/binary"
	"veyron.io/veyron/veyron2/services/mgmt/repository"
	"veyron.io/veyron/veyron2/verror"
	"veyron.io/veyron/veyron2/vlog"
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
	suffix string
}

var (
	errExists          = verror.Existsf("binary already exists")
	errNotFound        = verror.NoExistf("binary not found")
	errInProgress      = verror.Internalf("identical upload already in progress")
	errInvalidParts    = verror.BadArgf("invalid number of binary parts")
	errInvalidPart     = verror.BadArgf("invalid binary part number")
	errOperationFailed = verror.Internalf("operation failed")
)

// TODO(jsimsa): When VDL supports composite literal constants, remove
// this definition.
var MissingPart = binary.PartInfo{
	Checksum: binary.MissingChecksum,
	Size:     binary.MissingSize,
}

// newBinaryService returns a new Binary service implementation.
func newBinaryService(state *state, suffix string) *binaryService {
	return &binaryService{
		path:   state.dir(suffix),
		state:  state,
		suffix: suffix,
	}
}

const bufferLength = 4096

func (i *binaryService) Create(_ ipc.ServerContext, nparts int32, mediaInfo repository.MediaInfo) error {
	vlog.Infof("%v.Create(%v, %v)", i.suffix, nparts, mediaInfo)
	if nparts < 1 {
		return errInvalidParts
	}
	parent, perm := filepath.Dir(i.path), os.FileMode(0700)
	if err := os.MkdirAll(parent, perm); err != nil {
		vlog.Errorf("MkdirAll(%v, %v) failed: %v", parent, perm, err)
		return errOperationFailed
	}
	prefix := "creating-"
	tmpDir, err := ioutil.TempDir(parent, prefix)
	if err != nil {
		vlog.Errorf("TempDir(%v, %v) failed: %v", parent, prefix, err)
		return errOperationFailed
	}
	nameFile := filepath.Join(tmpDir, "name")
	if err := ioutil.WriteFile(nameFile, []byte(i.suffix), os.FileMode(0600)); err != nil {
		vlog.Errorf("WriteFile(%q) failed: %v", nameFile)
		return errOperationFailed
	}
	infoFile := filepath.Join(tmpDir, "mediainfo")
	jInfo, err := json.Marshal(mediaInfo)
	if err != nil {
		vlog.Errorf("json.Marshal(%v) failed: %v", mediaInfo, err)
		return errOperationFailed
	}
	if err := ioutil.WriteFile(infoFile, jInfo, os.FileMode(0600)); err != nil {
		vlog.Errorf("WriteFile(%q) failed: %v", infoFile, err)
		return errOperationFailed
	}
	for j := 0; j < int(nparts); j++ {
		partPath, partPerm := generatePartPath(tmpDir, j), os.FileMode(0700)
		if err := os.MkdirAll(partPath, partPerm); err != nil {
			vlog.Errorf("MkdirAll(%v, %v) failed: %v", partPath, partPerm, err)
			if err := os.RemoveAll(tmpDir); err != nil {
				vlog.Errorf("RemoveAll(%v) failed: %v", tmpDir, err)
			}
			return errOperationFailed
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
			return errExists
		}
		vlog.Errorf("Rename(%v, %v) failed: %v", tmpDir, i.path, err)
		return errOperationFailed
	}
	return nil
}

func (i *binaryService) Delete(context ipc.ServerContext) error {
	vlog.Infof("%v.Delete()", i.suffix)
	if _, err := os.Stat(i.path); err != nil {
		if os.IsNotExist(err) {
			return errNotFound
		}
		vlog.Errorf("Stat(%v) failed: %v", i.path, err)
		return errOperationFailed
	}
	// Use os.Rename() to atomically remove the binary directory
	// structure.
	path := filepath.Join(filepath.Dir(i.path), "removing-"+filepath.Base(i.path))
	if err := os.Rename(i.path, path); err != nil {
		vlog.Errorf("Rename(%v, %v) failed: %v", i.path, path, err)
		return errOperationFailed
	}
	if err := os.RemoveAll(path); err != nil {
		vlog.Errorf("Remove(%v) failed: %v", path, err)
		return errOperationFailed
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
			return errOperationFailed
		}
	}
	return nil
}

func (i *binaryService) Download(context repository.BinaryDownloadContext, part int32) error {
	vlog.Infof("%v.Download(%v)", i.suffix, part)
	path := i.generatePartPath(int(part))
	if err := checksumExists(path); err != nil {
		return err
	}
	dataPath := filepath.Join(path, data)
	file, err := os.Open(dataPath)
	if err != nil {
		vlog.Errorf("Open(%v) failed: %v", dataPath, err)
		return errOperationFailed
	}
	defer file.Close()
	buffer := make([]byte, bufferLength)
	sender := context.SendStream()
	for {
		n, err := file.Read(buffer)
		if err != nil && err != io.EOF {
			vlog.Errorf("Read() failed: %v", err)
			return errOperationFailed
		}
		if n == 0 {
			break
		}
		if err := sender.Send(buffer[:n]); err != nil {
			vlog.Errorf("Send() failed: %v", err)
			return errOperationFailed
		}
	}
	return nil
}

// TODO(jsimsa): Design and implement an access control mechanism for
// the URL-based downloads.
func (i *binaryService) DownloadURL(ipc.ServerContext) (string, int64, error) {
	vlog.Infof("%v.DownloadURL()", i.suffix)
	return i.state.rootURL + "/" + i.suffix, 0, nil
}

func (i *binaryService) Stat(ipc.ServerContext) ([]binary.PartInfo, repository.MediaInfo, error) {
	vlog.Infof("%v.Stat()", i.suffix)
	result := make([]binary.PartInfo, 0)
	parts, err := getParts(i.path)
	if err != nil {
		return []binary.PartInfo{}, repository.MediaInfo{}, err
	}
	for _, part := range parts {
		checksumFile := filepath.Join(part, checksum)
		bytes, err := ioutil.ReadFile(checksumFile)
		if err != nil {
			if os.IsNotExist(err) {
				result = append(result, MissingPart)
				continue
			}
			vlog.Errorf("ReadFile(%v) failed: %v", checksumFile, err)
			return []binary.PartInfo{}, repository.MediaInfo{}, errOperationFailed
		}
		dataFile := filepath.Join(part, data)
		fi, err := os.Stat(dataFile)
		if err != nil {
			if os.IsNotExist(err) {
				result = append(result, MissingPart)
				continue
			}
			vlog.Errorf("Stat(%v) failed: %v", dataFile, err)
			return []binary.PartInfo{}, repository.MediaInfo{}, errOperationFailed
		}
		result = append(result, binary.PartInfo{Checksum: string(bytes), Size: fi.Size()})
	}
	infoFile := filepath.Join(i.path, "mediainfo")
	jInfo, err := ioutil.ReadFile(infoFile)
	if err != nil {
		vlog.Errorf("ReadFile(%q) failed: %v", infoFile)
		return []binary.PartInfo{}, repository.MediaInfo{}, errOperationFailed
	}
	var mediaInfo repository.MediaInfo
	if err := json.Unmarshal(jInfo, &mediaInfo); err != nil {
		vlog.Errorf("json.Unmarshal(%v) failed: %v", jInfo, err)
		return []binary.PartInfo{}, repository.MediaInfo{}, errOperationFailed
	}
	return result, mediaInfo, nil
}

func (i *binaryService) Upload(context repository.BinaryUploadContext, part int32) error {
	vlog.Infof("%v.Upload(%v)", i.suffix, part)
	path, suffix := i.generatePartPath(int(part)), ""
	err := checksumExists(path)
	switch err {
	case nil:
		return errExists
	case errNotFound:
	default:
		return err
	}
	// Use os.OpenFile() to resolve races.
	lockPath, flags, perm := filepath.Join(path, lock), os.O_CREATE|os.O_WRONLY|os.O_EXCL, os.FileMode(0600)
	lockFile, err := os.OpenFile(lockPath, flags, perm)
	if err != nil {
		if os.IsExist(err) {
			return errInProgress
		}
		vlog.Errorf("OpenFile(%v, %v, %v) failed: %v", lockPath, flags, suffix, err)
		return errOperationFailed
	}
	defer os.Remove(lockFile.Name())
	defer lockFile.Close()
	file, err := ioutil.TempFile(path, suffix)
	if err != nil {
		vlog.Errorf("TempFile(%v, %v) failed: %v", path, suffix, err)
		return errOperationFailed
	}
	defer file.Close()
	h := md5.New()
	rStream := context.RecvStream()
	for rStream.Advance() {
		bytes := rStream.Value()
		if _, err := file.Write(bytes); err != nil {
			vlog.Errorf("Write() failed: %v", err)
			if err := os.Remove(file.Name()); err != nil {
				vlog.Errorf("Remove(%v) failed: %v", file.Name(), err)
			}
			return errOperationFailed
		}
		h.Write(bytes)
	}

	if err := rStream.Err(); err != nil {
		vlog.Errorf("Advance() failed: %v", err)
		if err := os.Remove(file.Name()); err != nil {
			vlog.Errorf("Remove(%v) failed: %v", file.Name(), err)
		}
		return errOperationFailed
	}

	hash := hex.EncodeToString(h.Sum(nil))
	checksumFile, perm := filepath.Join(path, checksum), os.FileMode(0600)
	if err := ioutil.WriteFile(checksumFile, []byte(hash), perm); err != nil {
		vlog.Errorf("WriteFile(%v, %v, %v) failed: %v", checksumFile, hash, perm, err)
		if err := os.Remove(file.Name()); err != nil {
			vlog.Errorf("Remove(%v) failed: %v", file.Name(), err)
		}
		return errOperationFailed
	}
	dataFile := filepath.Join(path, data)
	if err := os.Rename(file.Name(), dataFile); err != nil {
		vlog.Errorf("Rename(%v, %v) failed: %v", file.Name(), dataFile, err)
		if err := os.Remove(file.Name()); err != nil {
			vlog.Errorf("Remove(%v) failed: %v", file.Name(), err)
		}
		return errOperationFailed
	}
	return nil
}

func (i *binaryService) GlobChildren__(ipc.ServerContext) (<-chan string, error) {
	elems := strings.Split(i.suffix, "/")
	if len(elems) == 1 && elems[0] == "" {
		elems = nil
	}
	n := i.createObjectNameTree().find(elems, false)
	if n == nil {
		return nil, errOperationFailed
	}
	ch := make(chan string, 100)
	go func() {
		for k, _ := range n.children {
			ch <- k
		}
		close(ch)
	}()
	return ch, nil
}
