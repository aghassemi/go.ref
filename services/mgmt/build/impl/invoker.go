package impl

import (
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"veyron2/ipc"
	"veyron2/services/mgmt/binary"
	"veyron2/services/mgmt/build"
	"veyron2/vlog"
)

var (
	errBuildFailed   = errors.New("build failed")
	errInternalError = errors.New("internal error")
)

// invoker holds the state of a build server invocation.
type invoker struct {
	// gobin is the path to the Go compiler binary.
	gobin string
}

// NewInvoker is the invoker factory.
func NewInvoker(gobin string) *invoker {
	return &invoker{
		gobin: gobin,
	}
}

// BUILD INTERFACE IMPLEMENTATION

// TODO(jsimsa): Add support for building for a specific profile
// specified as a suffix the Build().
func (i *invoker) Build(_ ipc.ServerContext, stream build.BuildServiceBuildStream) ([]byte, error) {
	vlog.VI(1).Infof("Build() called.")
	dir, prefix := "", ""
	dirPerm, filePerm := os.FileMode(0700), os.FileMode(0600)
	root, err := ioutil.TempDir(dir, prefix)
	if err != nil {
		vlog.Errorf("TempDir(%v, %v) failed: %v", dir, prefix, err)
		return nil, errInternalError
	}
	defer os.RemoveAll(root)
	srcDir := filepath.Join(root, "go", "src")
	if err := os.MkdirAll(srcDir, dirPerm); err != nil {
		vlog.Errorf("MkdirAll(%v, %v) failed: %v", srcDir, dirPerm, err)
		return nil, errInternalError
	}
	for {
		srcFile, err := stream.Recv()
		if err != nil && err != io.EOF {
			vlog.Errorf("Recv() failed: %v", err)
			return nil, errInternalError
		}
		if err == io.EOF {
			break
		}
		filePath := filepath.Join(srcDir, filepath.FromSlash(srcFile.Name))
		dir := filepath.Dir(filePath)
		if err := os.MkdirAll(dir, dirPerm); err != nil {
			vlog.Errorf("MkdirAll(%v, %v) failed: %v", dir, dirPerm, err)
			return nil, errInternalError
		}
		if err := ioutil.WriteFile(filePath, srcFile.Contents, filePerm); err != nil {
			vlog.Errorf("WriteFile(%v, %v) failed: %v", filePath, filePerm, err)
			return nil, errInternalError
		}
	}
	cmd := exec.Command(i.gobin, "install", "-v", "...")
	cmd.Env = append(cmd.Env, "GOPATH="+filepath.Dir(srcDir))
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Run(); err != nil {
		vlog.Errorf("Run() failed: %v", err)
		if output.Len() != 0 {
			vlog.Errorf("%v", output.String())
		}
		return output.Bytes(), errBuildFailed
	}
	binDir := filepath.Join(root, "go", "bin")
	files, err := ioutil.ReadDir(binDir)
	if err != nil {
		vlog.Errorf("ReadDir(%v) failed: %v", binDir, err)
		return nil, errInternalError
	}
	// TODO(jsimsa): Analyze the binary files for non-standard shared
	// library dependencies.
	for _, file := range files {
		binPath := filepath.Join(root, "go", "bin", file.Name())
		bytes, err := ioutil.ReadFile(binPath)
		if err != nil {
			vlog.Errorf("ReadFile(%v) failed: %v", binPath, err)
			return nil, errInternalError
		}
		result := build.File{
			Name:     "bin/" + file.Name(),
			Contents: bytes,
		}
		if err := stream.Send(result); err != nil {
			vlog.Errorf("Send() failed: %v", err)
			return nil, errInternalError
		}
	}
	return output.Bytes(), nil
}

func (i *invoker) Describe(_ ipc.ServerContext, name string) (binary.Description, error) {
	// TODO(jsimsa): Implement.
	return binary.Description{}, nil
}
