package impl

import (
	"crypto/sha256"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"v.io/x/ref/services/mgmt/device/config"
	"v.io/x/ref/services/mgmt/lib/binary"

	"v.io/v23/context"
	"v.io/v23/security"
	"v.io/v23/services/mgmt/application"
	"v.io/v23/services/mgmt/repository"
	"v.io/v23/verror"
	"v.io/x/lib/vlog"
)

// TODO(caprita): Set these timeout in a more principled manner.
const (
	childReadyTimeout     = 40 * time.Second
	childWaitTimeout      = 40 * time.Second
	ipcContextTimeout     = time.Minute
	ipcContextLongTimeout = 5 * time.Minute
)

func verifySignature(data []byte, publisher security.Blessings, sig security.Signature) error {
	if !publisher.IsZero() {
		h := sha256.Sum256(data)
		if !sig.Verify(publisher.PublicKey(), h[:]) {
			return verror.New(ErrOperationFailed, nil)
		}
	}
	return nil
}

func downloadBinary(ctx *context.T, publisher security.Blessings, bin *application.SignedFile, workspace, fileName string) error {
	// TODO(gauthamt): Reduce the number of passes we make over the binary/package
	// data to verify its checksum and signature.
	data, _, err := binary.Download(ctx, bin.File)
	if err != nil {
		return verror.New(ErrOperationFailed, ctx, fmt.Sprintf("Download(%v) failed: %v", bin.File, err))
	}
	if err := verifySignature(data, publisher, bin.Signature); err != nil {
		return verror.New(ErrOperationFailed, ctx, fmt.Sprintf("Publisher binary(%v) signature verification failed", bin.File))
	}
	path, perm := filepath.Join(workspace, fileName), os.FileMode(0755)
	if err := ioutil.WriteFile(path, data, perm); err != nil {
		return verror.New(ErrOperationFailed, ctx, fmt.Sprintf("WriteFile(%v, %v) failed: %v", path, perm, err))
	}
	return nil
}

// TODO(caprita): share code between downloadBinary and downloadPackages.
func downloadPackages(ctx *context.T, publisher security.Blessings, packages application.Packages, pkgDir string) error {
	for localPkg, pkgName := range packages {
		if localPkg == "" || localPkg[0] == '.' || strings.Contains(localPkg, string(filepath.Separator)) {
			return verror.New(ErrOperationFailed, ctx, fmt.Sprintf("invalid local package name: %q", localPkg))
		}
		path := filepath.Join(pkgDir, localPkg)
		if err := binary.DownloadToFile(ctx, pkgName.File, path); err != nil {
			return verror.New(ErrOperationFailed, ctx, fmt.Sprintf("DownloadToFile(%q, %q) failed: %v", pkgName, path, err))
		}
		data, err := ioutil.ReadFile(path)
		if err != nil {
			return verror.New(ErrOperationFailed, ctx, fmt.Sprintf("ReadPackage(%v) failed: %v", path, err))
		}
		if err := verifySignature(data, publisher, pkgName.Signature); err != nil {
			return verror.New(ErrOperationFailed, ctx, fmt.Sprintf("Publisher package(%v:%v) signature verification failed", localPkg, pkgName))
		}
	}
	return nil
}

func fetchEnvelope(ctx *context.T, origin string) (*application.Envelope, error) {
	stub := repository.ApplicationClient(origin)
	profilesSet, err := Describe()
	if err != nil {
		return nil, verror.New(ErrOperationFailed, ctx, fmt.Sprintf("Failed to obtain profile labels: %v", err))
	}
	var profiles []string
	for label := range profilesSet.Profiles {
		profiles = append(profiles, label)
	}
	envelope, err := stub.Match(ctx, profiles)
	if err != nil {
		return nil, verror.New(ErrOperationFailed, ctx, fmt.Sprintf("Match(%v) failed: %v", profiles, err))
	}
	return &envelope, nil
}

// linkSelf creates a link to the current binary.
func linkSelf(workspace, fileName string) error {
	path := filepath.Join(workspace, fileName)
	self := os.Args[0]
	if err := os.Link(self, path); err != nil {
		return verror.New(ErrOperationFailed, nil, fmt.Sprintf("Link(%v, %v) failed: %v", self, path, err))
	}
	return nil
}

func generateVersionDirName() string {
	return time.Now().Format(time.RFC3339Nano)
}

func updateLink(target, link string) error {
	newLink := link + ".new"
	fi, err := os.Lstat(newLink)
	if err == nil {
		if err := os.Remove(fi.Name()); err != nil {
			return verror.New(ErrOperationFailed, nil, fmt.Sprintf("Remove(%v) failed: %v", fi.Name(), err))
		}
	}
	if err := os.Symlink(target, newLink); err != nil {
		return verror.New(ErrOperationFailed, nil, fmt.Sprintf("Symlink(%v, %v) failed: %v", target, newLink, err))
	}
	if err := os.Rename(newLink, link); err != nil {
		return verror.New(ErrOperationFailed, nil, fmt.Sprintf("Rename(%v, %v) failed: %v", newLink, link, err))
	}
	return nil
}

func baseCleanupDir(path, helper string) {
	if helper != "" {
		out, err := exec.Command(helper, "--rm", path).CombinedOutput()
		if err != nil {
			vlog.Errorf("exec.Command(%s %s %s).CombinedOutput() failed: %v", helper, "--rm", path, err)
			return
		}
		if len(out) != 0 {
			vlog.Errorf("exec.Command(%s %s %s).CombinedOutput() generated output: %v", helper, "--rm", path, string(out))
		}
	} else {
		if err := os.RemoveAll(path); err != nil {
			vlog.Errorf("RemoveAll(%v) failed: %v", path, err)
		}
	}
}

func aclDir(c *config.State) string {
	return filepath.Join(c.Root, "device-manager", "device-data", "acls")
}

// cleanupDir is defined like this so we can override its implementation for
// tests. cleanupDir will use the helper to delete application state possibly
// owned by different accounts if helper is provided.
var cleanupDir = baseCleanupDir
