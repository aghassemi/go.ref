package impl

// The app invoker is responsible for managing the state of applications on the
// node manager.  The node manager manages the applications it installs and runs
// using the following directory structure:
//
// TODO(caprita): Not all is yet implemented.
//
// <config.Root>/
//   app-<hash 1>/                  - the application dir is named using a hash of the application title
//     installation-<id 1>/         - installations are labelled with ids
//       <version 1 timestamp>/     - timestamp of when the version was downloaded
//         bin                      - application binary
//         previous                 - symbolic link to previous version directory (TODO)
//         origin                   - object name for application envelope
//         envelope                 - application envelope (JSON-encoded)
//       <version 2 timestamp>
//       ...
//       current                    - symbolic link to the current version
//       instances/
//         instance-<id a>/         - instances are labelled with ids
//           root/                  - workspace that the instance is run from
//           logs/                  - stderr/stdout and log files generated by instance
//           info                   - app manager name and process id for the instance (if running)
//           version                - symbolic link to installation version for the instance
//         instance-<id b>
//         ...
//         stopped-instance-<id c>  - stopped instances have their directory name prepended by 'stopped-'
//         ...
//     installation-<id 2>
//     ...
//   app-<hash 2>
//   ...
//
// When node manager starts up, it goes through all instances and resumes the
// ones that are not suspended.  If the application was still running, it
// suspends it first.  If an application fails to resume, it stays suspended.
//
// When node manager shuts down, it suspends all running instances.
//
// Start starts an instance.  Suspend kills the process but leaves the workspace
// untouched. Resume restarts the process. Stop kills the process and prevents
// future resumes (it also eventually gc's the workspace).
//
// If the process dies on its own, it stays dead and is assumed suspended.
// TODO(caprita): Later, we'll add auto-restart option.
//
// Concurrency model: installations can be created independently of one another;
// installations can be removed at any time (any running instances will be
// stopped). The first call to Uninstall will rename the installation dir as a
// first step; subsequent Uninstalls will fail. Instances can be created
// independently of one another, as long as the installation exists (if it gets
// Uninstalled during an instance Start, the Start may fail). When an instance
// is stopped, the first call to Stop renames the instance dir; subsequent Stop
// calls will fail. Resume will attempt to create an info file; if one exists
// already, Resume fails. Suspend will attempt to rename the info file; if none
// present, Suspend will fail.
//
// TODO(caprita): There is room for synergy between how node manager organizes
// its own workspace and that for the applications it runs.  In particular,
// previous, origin, and envelope could be part of a single config.  We'll
// refine that later.

import (
	"crypto/md5"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash/crc64"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"veyron/lib/config"
	vexec "veyron/services/mgmt/lib/exec"
	iconfig "veyron/services/mgmt/node/config"

	"veyron2/ipc"
	"veyron2/mgmt"
	"veyron2/naming"
	"veyron2/rt"
	"veyron2/services/mgmt/appcycle"
	"veyron2/services/mgmt/application"
	"veyron2/vlog"
)

// instanceInfo holds state about a running instance.
type instanceInfo struct {
	AppCycleMgrName string
	Pid             int
}

func saveInstanceInfo(dir string, info *instanceInfo) error {
	jsonInfo, err := json.Marshal(info)
	if err != nil {
		vlog.Errorf("Marshal(%v) failed: %v", info, err)
		return errOperationFailed
	}
	infoPath := filepath.Join(dir, "info")
	if err := ioutil.WriteFile(infoPath, jsonInfo, 0600); err != nil {
		vlog.Errorf("WriteFile(%v) failed: %v", infoPath, err)
		return errOperationFailed
	}
	return nil
}

func loadInstanceInfo(dir string) (*instanceInfo, error) {
	infoPath := filepath.Join(dir, "info")
	info := new(instanceInfo)
	if infoBytes, err := ioutil.ReadFile(infoPath); err != nil {
		vlog.Errorf("ReadFile(%v) failed: %v", infoPath, err)
		return nil, errOperationFailed
	} else if err := json.Unmarshal(infoBytes, info); err != nil {
		vlog.Errorf("Unmarshal(%v) failed: %v", infoBytes, err)
		return nil, errOperationFailed
	}
	return info, nil
}

// appInvoker holds the state of an application-related method invocation.
type appInvoker struct {
	callback *callbackState
	config   *iconfig.State
	// suffix contains the name components of the current invocation name
	// suffix.  It is used to identify an application, installation, or
	// instance.
	suffix []string
}

func saveEnvelope(dir string, envelope *application.Envelope) error {
	jsonEnvelope, err := json.Marshal(envelope)
	if err != nil {
		vlog.Errorf("Marshal(%v) failed: %v", envelope, err)
		return errOperationFailed
	}
	envelopePath := filepath.Join(dir, "envelope")
	if err := ioutil.WriteFile(envelopePath, jsonEnvelope, 0600); err != nil {
		vlog.Errorf("WriteFile(%v) failed: %v", envelopePath, err)
		return errOperationFailed
	}
	return nil
}

func loadEnvelope(dir string) (*application.Envelope, error) {
	envelopePath := filepath.Join(dir, "envelope")
	envelope := new(application.Envelope)
	if envelopeBytes, err := ioutil.ReadFile(envelopePath); err != nil {
		vlog.Errorf("ReadFile(%v) failed: %v", envelopePath, err)
		return nil, errOperationFailed
	} else if err := json.Unmarshal(envelopeBytes, envelope); err != nil {
		vlog.Errorf("Unmarshal(%v) failed: %v", envelopeBytes, err)
		return nil, errOperationFailed
	}
	return envelope, nil
}

func saveOrigin(dir, originVON string) error {
	path := filepath.Join(dir, "origin")
	if err := ioutil.WriteFile(path, []byte(originVON), 0600); err != nil {
		vlog.Errorf("WriteFile(%v) failed: %v", path, err)
		return errOperationFailed
	}
	return nil
}

// generateID returns a new unique id string.  The uniqueness is based on the
// current timestamp.  Not cryptographically secure.
func generateID() string {
	timestamp := fmt.Sprintf("%v", time.Now().Format(time.RFC3339Nano))
	h := crc64.New(crc64.MakeTable(crc64.ISO))
	h.Write([]byte(timestamp))
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, uint64(h.Sum64()))
	return strings.TrimRight(base64.URLEncoding.EncodeToString(b), "=")
}

// TODO(caprita): Nothing prevents different applications from sharing the same
// title, and thereby being installed in the same app dir.  Do we want to
// prevent that for the same user or across users?

// applicationDirName generates a cryptographic hash of the application title,
// to be used as a directory name for installations of the application with the
// given title.
func applicationDirName(title string) string {
	h := md5.New()
	h.Write([]byte(title))
	hash := strings.TrimRight(base64.URLEncoding.EncodeToString(h.Sum(nil)), "=")
	return "app-" + hash
}

func installationDirName(installationID string) string {
	return "installation-" + installationID
}

func instanceDirName(instanceID string) string {
	return "instance-" + instanceID
}

func stoppedInstanceDirName(instanceID string) string {
	return "stopped-instance-" + instanceID
}

func (i *appInvoker) Install(call ipc.ServerContext, applicationVON string) (string, error) {
	ctx, cancel := rt.R().NewContext().WithTimeout(time.Minute)
	defer cancel()
	envelope, err := fetchEnvelope(ctx, applicationVON)
	if err != nil {
		return "", err
	}
	if envelope.Title == application.NodeManagerTitle {
		// Disallow node manager apps from being installed like a
		// regular app.
		return "", errInvalidOperation
	}
	installationID := generateID()
	installationDir := filepath.Join(i.config.Root, applicationDirName(envelope.Title), installationDirName(installationID))
	versionDir := filepath.Join(installationDir, generateVersionDirName())
	perm := os.FileMode(0700)
	if err := os.MkdirAll(versionDir, perm); err != nil {
		vlog.Errorf("MkdirAll(%v, %v) failed: %v", versionDir, perm, err)
		return "", errOperationFailed
	}
	deferrer := func() {
		if err := os.RemoveAll(versionDir); err != nil {
			vlog.Errorf("RemoveAll(%v) failed: %v", versionDir, err)
		}
	}
	defer func() {
		if deferrer != nil {
			deferrer()
		}
	}()
	// TODO(caprita): Share binaries if already existing locally.
	if err := generateBinary(versionDir, "bin", envelope, true); err != nil {
		return "", err
	}
	if err := saveEnvelope(versionDir, envelope); err != nil {
		return "", err
	}
	if err := saveOrigin(versionDir, applicationVON); err != nil {
		return "", err
	}
	link := filepath.Join(installationDir, "current")
	if err := os.Symlink(versionDir, link); err != nil {
		vlog.Errorf("Symlink(%v, %v) failed: %v", versionDir, link, err)
		return "", errOperationFailed
	}
	deferrer = nil
	return naming.Join(envelope.Title, installationID), nil
}

func (*appInvoker) Refresh(ipc.ServerContext) error {
	// TODO(jsimsa): Implement.
	return nil
}

func (*appInvoker) Restart(ipc.ServerContext) error {
	// TODO(jsimsa): Implement.
	return nil
}

func (*appInvoker) Resume(ipc.ServerContext) error {
	// TODO(jsimsa): Implement.
	return nil
}

func (*appInvoker) Revert(ipc.ServerContext) error {
	// TODO(jsimsa): Implement.
	return nil
}

func generateCommand(envelope *application.Envelope, binPath, instanceDir string) (*exec.Cmd, error) {
	// TODO(caprita): For the purpose of isolating apps, we should run them
	// as different users.  We'll need to either use the root process or a
	// suid script to be able to do it.
	cmd := exec.Command(binPath)
	// TODO(caprita): Also pass in configuration info like NAMESPACE_ROOT to
	// the app (to point to the device mounttable).
	cmd.Env = envelope.Env
	rootDir := filepath.Join(instanceDir, "root")
	perm := os.FileMode(0700)
	if err := os.MkdirAll(rootDir, perm); err != nil {
		vlog.Errorf("MkdirAll(%v, %v) failed: %v", rootDir, perm, err)
		return nil, err
	}
	cmd.Dir = rootDir
	logDir := filepath.Join(instanceDir, "logs")
	if err := os.MkdirAll(logDir, perm); err != nil {
		vlog.Errorf("MkdirAll(%v, %v) failed: %v", logDir, perm, err)
		return nil, err
	}
	timestamp := time.Now().UnixNano()
	var err error
	perm = os.FileMode(0600)
	cmd.Stdout, err = os.OpenFile(filepath.Join(logDir, fmt.Sprintf("STDOUT-%d", timestamp)), os.O_WRONLY|os.O_CREATE, perm)
	if err != nil {
		return nil, err
	}

	cmd.Stderr, err = os.OpenFile(filepath.Join(logDir, fmt.Sprintf("STDERR-%d", timestamp)), os.O_WRONLY|os.O_CREATE, perm)
	if err != nil {
		return nil, err
	}
	// Set up args and env.
	cmd.Args = append(cmd.Args, "--log_dir=../logs")
	cmd.Args = append(cmd.Args, envelope.Args...)
	return cmd, nil
}

func (i *appInvoker) Start(ipc.ServerContext) ([]string, error) {
	components := i.suffix
	if nComponents := len(components); nComponents < 2 {
		return nil, fmt.Errorf("Start all installations / all applications not yet implemented (%v)", naming.Join(i.suffix...))
	} else if nComponents > 2 {
		return nil, errInvalidSuffix
	}
	app, installation := components[0], components[1]
	installationDir := filepath.Join(i.config.Root, applicationDirName(app), installationDirName(installation))
	if _, err := os.Stat(installationDir); err != nil {
		if os.IsNotExist(err) {
			return nil, errNotExist
		}
		vlog.Errorf("Stat(%v) failed: %v", installationDir, err)
		return nil, errOperationFailed
	}
	currLink := filepath.Join(installationDir, "current")
	envelope, err := loadEnvelope(currLink)
	if err != nil {
		return nil, err
	}
	binPath := filepath.Join(currLink, "bin")
	if _, err := os.Stat(binPath); err != nil {
		vlog.Errorf("Stat(%v) failed: %v", binPath, err)
		return nil, errOperationFailed
	}
	instanceID := generateID()
	// TODO(caprita): Clean up instanceDir upon failure.
	instanceDir := filepath.Join(installationDir, "instances", instanceDirName(instanceID))
	cmd, err := generateCommand(envelope, binPath, instanceDir)
	if err != nil {
		vlog.Errorf("generateCommand(%v, %v, %v) failed: %v", envelope, binPath, instanceDir, err)
		return nil, errOperationFailed
	}
	// Setup up the child process callback.
	callbackState := i.callback
	listener := callbackState.listenFor(mgmt.AppCycleManagerConfigKey)
	defer listener.cleanup()
	cfg := config.New()
	cfg.Set(mgmt.ParentNodeManagerConfigKey, listener.name())
	handle := vexec.NewParentHandle(cmd, vexec.ConfigOpt{cfg})
	// Start the child process.
	if err := handle.Start(); err != nil {
		vlog.Errorf("Start() failed: %v", err)
		return nil, errOperationFailed
	}
	// Wait for the child process to start.
	timeout := 10 * time.Second
	if err := handle.WaitForReady(timeout); err != nil {
		vlog.Errorf("WaitForReady(%v) failed: %v", timeout, err)
		if err := handle.Clean(); err != nil {
			vlog.Errorf("Clean() failed: %v", err)
		}
		return nil, errOperationFailed
	}
	childName, err := listener.waitForValue(timeout)
	if err != nil {
		if err := handle.Clean(); err != nil {
			vlog.Errorf("Clean() failed: %v", err)
		}
		return nil, errOperationFailed
	}
	instanceInfo := &instanceInfo{
		AppCycleMgrName: childName,
		Pid:             handle.Pid(),
	}
	if err := saveInstanceInfo(instanceDir, instanceInfo); err != nil {
		if err := handle.Clean(); err != nil {
			vlog.Errorf("Clean() failed: %v", err)
		}
		return nil, err
	}
	// TODO(caprita): Spin up a goroutine to reap child status upon exit and
	// transition it to suspended state if it exits on its own.
	return []string{instanceID}, nil
}

func (i *appInvoker) Stop(_ ipc.ServerContext, deadline uint32) error {
	// TODO(caprita): implement deadline.
	ctx, cancel := rt.R().NewContext().WithTimeout(time.Minute)
	defer cancel()
	components := i.suffix
	if nComponents := len(components); nComponents < 3 {
		return fmt.Errorf("Stop all instances / all installations / all applications not yet implemented (%v)", naming.Join(i.suffix...))
	} else if nComponents > 3 {
		return errInvalidSuffix
	}
	app, installation, instance := components[0], components[1], components[2]
	instancesDir := filepath.Join(i.config.Root, applicationDirName(app), installationDirName(installation), "instances")
	instanceDir := filepath.Join(instancesDir, instanceDirName(instance))
	stoppedInstanceDir := filepath.Join(instancesDir, stoppedInstanceDirName(instance))
	if err := os.Rename(instanceDir, stoppedInstanceDir); err != nil {
		vlog.Errorf("Rename(%v, %v) failed: %v", instanceDir, stoppedInstanceDir, err)
		if os.IsNotExist(err) {
			return errNotExist
		}
		vlog.Errorf("Rename(%v, %v) failed: %v", instanceDir, stoppedInstanceDir, err)
		return errOperationFailed
	}
	// TODO(caprita): restore the instance to unstopped upon failure?

	info, err := loadInstanceInfo(stoppedInstanceDir)
	if err != nil {
		return errOperationFailed
	}
	appStub, err := appcycle.BindAppCycle(info.AppCycleMgrName)
	if err != nil {
		vlog.Errorf("BindAppCycle(%v) failed: %v", info.AppCycleMgrName, err)
		return errOperationFailed
	}
	stream, err := appStub.Stop(ctx)
	if err != nil {
		vlog.Errorf("Got error: %v", err)
		return errOperationFailed
	}
	rstream := stream.RecvStream()
	for rstream.Advance() {
		vlog.VI(2).Infof("%v.Stop(%v) task update: %v", i.suffix, deadline, rstream.Value())
	}
	if err := rstream.Err(); err != nil {
		vlog.Errorf("Stream returned an error: %v", err)
		return errOperationFailed
	}
	if err := stream.Finish(); err != nil {
		vlog.Errorf("Got error: %v", err)
		return errOperationFailed
	}
	return nil
}

func (*appInvoker) Suspend(ipc.ServerContext) error {
	// TODO(jsimsa): Implement.
	return nil
}

func (*appInvoker) Uninstall(ipc.ServerContext) error {
	// TODO(jsimsa): Implement.
	return nil
}

func (i *appInvoker) Update(ipc.ServerContext) error {
	// TODO(jsimsa): Implement.
	return nil
}

func (i *appInvoker) UpdateTo(_ ipc.ServerContext, von string) error {
	// TODO(jsimsa): Implement.
	return nil
}
