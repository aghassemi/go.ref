package impl_test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"v.io/core/veyron2/context"
	"v.io/core/veyron2/services/mgmt/repository"

	"v.io/core/veyron/lib/testutil"
	"v.io/core/veyron/services/mgmt/binary/impl"
)

// invokeUpload invokes the Upload RPC using the given client binary
// <binary> and streams the given binary <binary> to it.
func invokeUpload(t *testing.T, ctx *context.T, binary repository.BinaryClientMethods, data []byte, part int32) (error, error) {
	stream, err := binary.Upload(ctx, part)
	if err != nil {
		t.Errorf("Upload() failed: %v", err)
		return nil, err
	}
	sender := stream.SendStream()
	if streamErr := sender.Send(data); streamErr != nil {
		err := stream.Finish()
		if err != nil {
			t.Logf("Finish() failed: %v", err)
		}
		t.Logf("Send() failed: %v", streamErr)
		return streamErr, err
	}
	if streamErr := sender.Close(); streamErr != nil {
		err := stream.Finish()
		if err != nil {
			t.Logf("Finish() failed: %v", err)
		}
		t.Logf("Close() failed: %v", streamErr)
		return streamErr, err
	}
	if err := stream.Finish(); err != nil {
		t.Logf("Finish() failed: %v", err)
		return nil, err
	}
	return nil, nil
}

// invokeDownload invokes the Download RPC using the given client binary
// <binary> and streams binary from to it.
func invokeDownload(t *testing.T, ctx *context.T, binary repository.BinaryClientMethods, part int32) ([]byte, error, error) {
	stream, err := binary.Download(ctx, part)
	if err != nil {
		t.Errorf("Download() failed: %v", err)
		return nil, nil, err
	}
	output := make([]byte, 0)
	rStream := stream.RecvStream()
	for rStream.Advance() {
		bytes := rStream.Value()
		output = append(output, bytes...)
	}

	if streamErr := rStream.Err(); streamErr != nil {
		err := stream.Finish()
		if err != nil {
			t.Logf("Finish() failed: %v", err)
		}
		t.Logf("Advance() failed with: %v", streamErr)
		return nil, streamErr, err
	}

	if err := stream.Finish(); err != nil {
		t.Logf("Finish() failed: %v", err)
		return nil, nil, err
	}
	return output, nil, nil
}

func prepDirectory(t *testing.T, rootDir string) {
	path, perm := filepath.Join(rootDir, impl.VersionFile), os.FileMode(0600)
	if err := ioutil.WriteFile(path, []byte(impl.Version), perm); err != nil {
		t.Fatalf(testutil.FormatLogLine(2, "WriteFile(%v, %v, %v) failed: %v", path, impl.Version, perm, err))
	}
}

// testData creates up to 4MB of random bytes.
func testData() []byte {
	size := testutil.Rand.Intn(1000 * impl.BufferLength)
	data := testutil.RandomBytes(size)
	return data
}
