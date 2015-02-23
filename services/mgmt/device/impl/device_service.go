package impl

// The device invoker is responsible for managing the state of the device
// manager itself.  The implementation expects that the device manager
// installations are all organized in the following directory structure:
//
// <config.Root>/
//   device-manager/
//     info                    - metadata for the device manager (such as object
//                               name and process id)
//     logs/                   - device manager logs
//       STDERR-<timestamp>    - one for each execution of device manager
//       STDOUT-<timestamp>    - one for each execution of device manager
//     <version 1 timestamp>/  - timestamp of when the version was downloaded
//       deviced               - the device manager binary
//       deviced.sh            - a shell script to start the binary
//     <version 2 timestamp>
//     ...
//     device-data/
//       acls/
//         data
//         signature
//	 associated.accounts
//       persistent-args       - list of persistent arguments for the device
//                               manager (json encoded)
//
// The device manager is always expected to be started through the symbolic link
// passed in as config.CurrentLink, which is monitored by an init daemon. This
// provides for simple and robust updates.
//
// To update the device manager to a newer version, a new workspace is created
// and the symlink is updated to the new deviced.sh script. Similarly, to revert
// the device manager to a previous version, all that is required is to update
// the symlink to point to the previous deviced.sh script.

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"v.io/core/veyron2"
	"v.io/core/veyron2/context"
	"v.io/core/veyron2/ipc"
	"v.io/core/veyron2/mgmt"
	"v.io/core/veyron2/naming"
	"v.io/core/veyron2/security"
	"v.io/core/veyron2/services/mgmt/application"
	"v.io/core/veyron2/services/mgmt/binary"
	"v.io/core/veyron2/services/mgmt/device"
	"v.io/core/veyron2/services/security/access"
	"v.io/core/veyron2/verror"
	"v.io/core/veyron2/vlog"

	vexec "v.io/core/veyron/lib/exec"
	"v.io/core/veyron/lib/flags/consts"
	vsecurity "v.io/core/veyron/security"
	"v.io/core/veyron/services/mgmt/device/config"
	"v.io/core/veyron/services/mgmt/profile"
)

type updatingState struct {
	// updating is a flag that records whether this instance of device
	// manager is being updated.
	updating bool
	// updatingMutex is a lock for coordinating concurrent access to
	// <updating>.
	updatingMutex sync.Mutex
}

func newUpdatingState() *updatingState {
	return new(updatingState)
}

func (u *updatingState) testAndSetUpdating() bool {
	u.updatingMutex.Lock()
	defer u.updatingMutex.Unlock()
	if u.updating {
		return true
	}
	u.updating = true
	return false
}

func (u *updatingState) unsetUpdating() {
	u.updatingMutex.Lock()
	u.updating = false
	u.updatingMutex.Unlock()
}

// deviceService implements the Device manager's Device interface.
type deviceService struct {
	updating       *updatingState
	restartHandler func()
	callback       *callbackState
	config         *config.State
	disp           *dispatcher
	uat            BlessingSystemAssociationStore
	securityAgent  *securityAgentState
}

// ManagerInfo holds state about a running device manager or a running agentd
type ManagerInfo struct {
	Pid int
}

func SaveManagerInfo(dir string, info *ManagerInfo) error {
	jsonInfo, err := json.Marshal(info)
	if err != nil {
		vlog.Errorf("Marshal(%v) failed: %v", info, err)
		return verror.New(ErrOperationFailed, nil)
	}
	if err := os.MkdirAll(dir, os.FileMode(0700)); err != nil {
		vlog.Errorf("MkdirAll(%v) failed: %v", dir, err)
		return verror.New(ErrOperationFailed, nil)
	}
	infoPath := filepath.Join(dir, "info")
	if err := ioutil.WriteFile(infoPath, jsonInfo, 0600); err != nil {
		vlog.Errorf("WriteFile(%v) failed: %v", infoPath, err)
		return verror.New(ErrOperationFailed, nil)
	}
	return nil
}

func loadManagerInfo(dir string) (*ManagerInfo, error) {
	infoPath := filepath.Join(dir, "info")
	info := new(ManagerInfo)
	if infoBytes, err := ioutil.ReadFile(infoPath); err != nil {
		vlog.Errorf("ReadFile(%v) failed: %v", infoPath, err)
		return nil, verror.New(ErrOperationFailed, nil)
	} else if err := json.Unmarshal(infoBytes, info); err != nil {
		vlog.Errorf("Unmarshal(%v) failed: %v", infoBytes, err)
		return nil, verror.New(ErrOperationFailed, nil)
	}
	return info, nil
}

func savePersistentArgs(root string, args []string) error {
	dir := filepath.Join(root, "device-manager", "device-data")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("MkdirAll(%q) failed: %v", dir)
	}
	data, err := json.Marshal(args)
	if err != nil {
		return fmt.Errorf("Marshal(%v) failed: %v", args, err)
	}
	fileName := filepath.Join(dir, "persistent-args")
	return ioutil.WriteFile(fileName, data, 0600)
}

func loadPersistentArgs(root string) ([]string, error) {
	fileName := filepath.Join(root, "device-manager", "device-data", "persistent-args")
	bytes, err := ioutil.ReadFile(fileName)
	if err != nil {
		return nil, err
	}
	args := []string{}
	if err := json.Unmarshal(bytes, &args); err != nil {
		return nil, fmt.Errorf("json.Unmarshal(%v) failed: %v", bytes, err)
	}
	return args, nil
}

func (*deviceService) Describe(ipc.ServerContext) (device.Description, error) {
	return Describe()
}

func (*deviceService) IsRunnable(_ ipc.ServerContext, description binary.Description) (bool, error) {
	deviceProfile, err := ComputeDeviceProfile()
	if err != nil {
		return false, err
	}
	binaryProfiles := make([]*profile.Specification, 0)
	for name, _ := range description.Profiles {
		profile, err := getProfile(name)
		if err != nil {
			return false, err
		}
		binaryProfiles = append(binaryProfiles, profile)
	}
	result := matchProfiles(deviceProfile, binaryProfiles)
	return len(result.Profiles) > 0, nil
}

func (*deviceService) Reset(call ipc.ServerContext, deadline uint64) error {
	// TODO(jsimsa): Implement.
	return nil
}

// getCurrentFileInfo returns the os.FileInfo for both the symbolic link
// CurrentLink, and the device script in the workspace that this link points to.
func (s *deviceService) getCurrentFileInfo() (os.FileInfo, string, error) {
	path := s.config.CurrentLink
	link, err := os.Lstat(path)
	if err != nil {
		vlog.Errorf("Lstat(%v) failed: %v", path, err)
		return nil, "", err
	}
	scriptPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		vlog.Errorf("EvalSymlinks(%v) failed: %v", path, err)
		return nil, "", err
	}
	return link, scriptPath, nil
}

func (s *deviceService) revertDeviceManager(ctx *context.T) error {
	if err := updateLink(s.config.Previous, s.config.CurrentLink); err != nil {
		vlog.Errorf("updateLink failed: %v", err)
		return err
	}
	if s.restartHandler != nil {
		s.restartHandler()
	}
	veyron2.GetAppCycle(ctx).Stop()
	return nil
}

func (s *deviceService) newLogfile(prefix string) (*os.File, error) {
	d := filepath.Join(s.config.Root, "device_test_logs")
	if _, err := os.Stat(d); err != nil {
		if err := os.MkdirAll(d, 0700); err != nil {
			return nil, err
		}
	}
	f, err := ioutil.TempFile(d, "__device_impl_test__"+prefix)
	if err != nil {
		return nil, err
	}
	return f, nil
}

// TODO(cnicolaou): would this be better implemented using the modules
// framework now that it exists?
func (s *deviceService) testDeviceManager(ctx *context.T, workspace string, envelope *application.Envelope) error {
	path := filepath.Join(workspace, "deviced.sh")
	cmd := exec.Command(path)
	cmd.Env = []string{"DEVICE_MANAGER_DONT_REDIRECT_STDOUT_STDERR=1"}

	for k, v := range map[string]*io.Writer{
		"stdout": &cmd.Stdout,
		"stderr": &cmd.Stderr,
	} {
		// Using a log file makes it less likely that stdout and stderr
		// output will be lost if the child crashes.
		file, err := s.newLogfile(fmt.Sprintf("deviced-test-%s", k))
		if err != nil {
			return err
		}
		fName := file.Name()
		defer os.Remove(fName)
		*v = file

		defer func() {
			if f, err := os.Open(fName); err == nil {
				scanner := bufio.NewScanner(f)
				for scanner.Scan() {
					vlog.Infof("[testDeviceManager %s] %s", k, scanner.Text())
				}
			}
		}()
	}

	// Setup up the child process callback.
	callbackState := s.callback
	listener := callbackState.listenFor(mgmt.ChildNameConfigKey)
	defer listener.cleanup()
	cfg := vexec.NewConfig()

	cfg.Set(mgmt.ParentNameConfigKey, listener.name())
	cfg.Set(mgmt.ProtocolConfigKey, "tcp")
	cfg.Set(mgmt.AddressConfigKey, "127.0.0.1:0")

	var p security.Principal
	var agentHandle []byte
	if s.securityAgent != nil {
		// TODO(rthellend): Cleanup principal
		handle, conn, err := s.securityAgent.keyMgrAgent.NewPrincipal(ctx, false)
		if err != nil {
			vlog.Errorf("NewPrincipal() failed %v", err)
			return verror.New(ErrOperationFailed, nil)
		}
		agentHandle = handle
		var cancel func()
		if p, cancel, err = agentPrincipal(ctx, conn); err != nil {
			vlog.Errorf("agentPrincipal failed: %v", err)
			return verror.New(ErrOperationFailed, nil)
		}
		defer cancel()

	} else {
		credentialsDir := filepath.Join(workspace, "credentials")
		var err error
		if p, err = vsecurity.CreatePersistentPrincipal(credentialsDir, nil); err != nil {
			vlog.Errorf("CreatePersistentPrincipal(%v, nil) failed: %v", credentialsDir, err)
			return verror.New(ErrOperationFailed, nil)
		}
		cmd.Env = append(cmd.Env, consts.VeyronCredentials+"="+credentialsDir)
	}
	dmPrincipal := veyron2.GetPrincipal(ctx)
	dmBlessings, err := dmPrincipal.Bless(p.PublicKey(), dmPrincipal.BlessingStore().Default(), "testdm", security.UnconstrainedUse())
	if err := p.BlessingStore().SetDefault(dmBlessings); err != nil {
		vlog.Errorf("BlessingStore.SetDefault() failed: %v", err)
		return verror.New(ErrOperationFailed, nil)
	}
	if _, err := p.BlessingStore().Set(dmBlessings, security.AllPrincipals); err != nil {
		vlog.Errorf("BlessingStore.Set() failed: %v", err)
		return verror.New(ErrOperationFailed, nil)
	}
	if err := p.AddToRoots(dmBlessings); err != nil {
		vlog.Errorf("AddToRoots() failed: %v", err)
		return verror.New(ErrOperationFailed, nil)
	}

	if s.securityAgent != nil {
		file, err := s.securityAgent.keyMgrAgent.NewConnection(agentHandle)
		if err != nil {
			vlog.Errorf("NewConnection(%v) failed: %v", agentHandle, err)
			return err
		}
		defer file.Close()

		fd := len(cmd.ExtraFiles) + vexec.FileOffset
		cmd.ExtraFiles = append(cmd.ExtraFiles, file)
		cfg.Set(mgmt.SecurityAgentFDConfigKey, strconv.Itoa(fd))
	}

	handle := vexec.NewParentHandle(cmd, vexec.ConfigOpt{cfg})
	// Start the child process.
	if err := handle.Start(); err != nil {
		vlog.Errorf("Start() failed: %v", err)
		return verror.New(ErrOperationFailed, ctx)
	}
	defer func() {
		if err := handle.Clean(); err != nil {
			vlog.Errorf("Clean() failed: %v", err)
		}
	}()
	// Wait for the child process to start.
	if err := handle.WaitForReady(childReadyTimeout); err != nil {
		vlog.Errorf("WaitForReady(%v) failed: %v", childReadyTimeout, err)
		return verror.New(ErrOperationFailed, ctx)
	}
	childName, err := listener.waitForValue(childReadyTimeout)
	if err != nil {
		vlog.Errorf("waitForValue(%v) failed: %v", childReadyTimeout, err)
		return verror.New(ErrOperationFailed, ctx)
	}
	// Check that invoking Stop() succeeds.
	childName = naming.Join(childName, "device")
	dmClient := device.DeviceClient(childName)
	if err := dmClient.Stop(ctx, 0); err != nil {
		vlog.Errorf("Stop() failed: %v", err)
		return verror.New(ErrOperationFailed, ctx)
	}
	if err := handle.Wait(childWaitTimeout); err != nil {
		vlog.Errorf("New device manager failed to exit cleanly: %v", err)
		return verror.New(ErrOperationFailed, ctx)
	}
	return nil
}

// TODO(caprita): Move this to util.go since device_installer is also using it now.

func generateScript(workspace string, configSettings []string, envelope *application.Envelope, logs string) error {
	// TODO(caprita): Remove this snippet of code, it doesn't seem to serve
	// any purpose.
	path, err := filepath.EvalSymlinks(os.Args[0])
	if err != nil {
		vlog.Errorf("EvalSymlinks(%v) failed: %v", os.Args[0], err)
		return verror.New(ErrOperationFailed, nil)
	}

	if err := os.MkdirAll(logs, 0700); err != nil {
		vlog.Errorf("MkdirAll(%v) failed: %v", logs, err)
		return verror.New(ErrOperationFailed, nil)
	}
	stderrLog, stdoutLog := filepath.Join(logs, "STDERR"), filepath.Join(logs, "STDOUT")

	output := "#!/bin/bash\n"
	output += "if [ -z \"$DEVICE_MANAGER_DONT_REDIRECT_STDOUT_STDERR\" ]; then\n"
	output += fmt.Sprintf("  TIMESTAMP=$(%s)\n", dateCommand)
	output += fmt.Sprintf("  exec > %s-$TIMESTAMP 2> %s-$TIMESTAMP\n", stdoutLog, stderrLog)
	output += "fi\n"
	output += strings.Join(config.QuoteEnv(append(envelope.Env, configSettings...)), " ") + " "
	// Escape the path to the binary; %q uses Go-syntax escaping, but it's
	// close enough to Bash that we're using it as an approximation.
	//
	// TODO(caprita/rthellend): expose and use shellEscape (from
	// veyron/tools/debug/impl.go) instead.
	output += fmt.Sprintf("exec %q", filepath.Join(workspace, "deviced")) + " "
	output += fmt.Sprintf("--log_dir=%q ", logs)
	output += strings.Join(envelope.Args, " ")

	path = filepath.Join(workspace, "deviced.sh")
	if err := ioutil.WriteFile(path, []byte(output), 0700); err != nil {
		vlog.Errorf("WriteFile(%v) failed: %v", path, err)
		return verror.New(ErrOperationFailed, nil)
	}
	return nil
}

func (s *deviceService) updateDeviceManager(ctx *context.T) error {
	if len(s.config.Origin) == 0 {
		return verror.New(ErrUpdateNoOp, ctx)
	}
	envelope, err := fetchEnvelope(ctx, s.config.Origin)
	if err != nil {
		return err
	}
	if envelope.Title != application.DeviceManagerTitle {
		vlog.Errorf("app title mismatch. Got %q, expected %q.", envelope.Title, application.DeviceManagerTitle)
		return verror.New(ErrAppTitleMismatch, ctx)
	}
	// Read and merge persistent args, if present.
	if args, err := loadPersistentArgs(s.config.Root); err == nil {
		envelope.Args = append(envelope.Args, args...)
	}
	if s.config.Envelope != nil && reflect.DeepEqual(envelope, s.config.Envelope) {
		return verror.New(ErrUpdateNoOp, ctx)
	}
	// Create new workspace.
	workspace := filepath.Join(s.config.Root, "device-manager", generateVersionDirName())
	perm := os.FileMode(0700)
	if err := os.MkdirAll(workspace, perm); err != nil {
		vlog.Errorf("MkdirAll(%v, %v) failed: %v", workspace, perm, err)
		return verror.New(ErrOperationFailed, ctx)
	}

	deferrer := func() {
		cleanupDir(workspace, "")
	}
	defer func() {
		if deferrer != nil {
			deferrer()
		}
	}()

	// Populate the new workspace with a device manager binary.
	// TODO(caprita): match identical binaries on binary signature
	// rather than binary object name.
	sameBinary := s.config.Envelope != nil && envelope.Binary.File == s.config.Envelope.Binary.File
	if sameBinary {
		if err := linkSelf(workspace, "deviced"); err != nil {
			return err
		}
	} else {
		publisher, err := security.NewBlessings(envelope.Publisher)
		if err != nil {
			vlog.Errorf("Failed to parse publisher blessings:%v", err)
			return verror.New(ErrOperationFailed, nil)
		}
		if err := downloadBinary(ctx, publisher, &envelope.Binary, workspace, "deviced"); err != nil {
			return err
		}
	}

	// Populate the new workspace with a device manager script.
	configSettings, err := s.config.Save(envelope)
	if err != nil {
		return verror.New(ErrOperationFailed, ctx)
	}

	logs := filepath.Join(s.config.Root, "device-manager", "logs")
	if err := generateScript(workspace, configSettings, envelope, logs); err != nil {
		return err
	}

	if err := s.testDeviceManager(ctx, workspace, envelope); err != nil {
		vlog.Errorf("testDeviceManager failed: %v", err)
		return err
	}

	if err := updateLink(filepath.Join(workspace, "deviced.sh"), s.config.CurrentLink); err != nil {
		return err
	}

	if s.restartHandler != nil {
		s.restartHandler()
	}
	veyron2.GetAppCycle(ctx).Stop()
	deferrer = nil
	return nil
}

func (*deviceService) Install(ctx ipc.ServerContext, _ string, _ device.Config, _ application.Packages) (string, error) {
	return "", verror.New(ErrInvalidSuffix, ctx.Context())
}

func (*deviceService) Refresh(ipc.ServerContext) error {
	// TODO(jsimsa): Implement.
	return nil
}

func (*deviceService) Restart(ipc.ServerContext) error {
	// TODO(jsimsa): Implement.
	return nil
}

func (*deviceService) Resume(ctx ipc.ServerContext) error {
	return verror.New(ErrInvalidSuffix, ctx.Context())
}

func (s *deviceService) Revert(call ipc.ServerContext) error {
	if s.config.Previous == "" {
		vlog.Errorf("Revert failed: no previous version")
		return verror.New(ErrUpdateNoOp, call.Context())
	}
	updatingState := s.updating
	if updatingState.testAndSetUpdating() {
		vlog.Errorf("Revert failed: already in progress")
		return verror.New(ErrOperationInProgress, call.Context())
	}
	err := s.revertDeviceManager(call.Context())
	if err != nil {
		updatingState.unsetUpdating()
	}
	return err
}

func (*deviceService) Start(ctx ipc.ServerContext) ([]string, error) {
	return nil, verror.New(ErrInvalidSuffix, ctx.Context())
}

func (*deviceService) Stop(call ipc.ServerContext, _ uint32) error {
	veyron2.GetAppCycle(call.Context()).Stop()
	return nil
}

func (s *deviceService) Suspend(call ipc.ServerContext) error {
	// TODO(caprita): move this to Restart and disable Suspend for device
	// manager?
	if s.restartHandler != nil {
		s.restartHandler()
	}
	veyron2.GetAppCycle(call.Context()).Stop()
	return nil
}

func (*deviceService) Uninstall(ctx ipc.ServerContext) error {
	return verror.New(ErrInvalidSuffix, ctx.Context())
}

func (s *deviceService) Update(call ipc.ServerContext) error {
	ctx, cancel := context.WithTimeout(call.Context(), ipcContextLongTimeout)
	defer cancel()

	updatingState := s.updating
	if updatingState.testAndSetUpdating() {
		return verror.New(ErrOperationInProgress, call.Context())
	}

	err := s.updateDeviceManager(ctx)
	if err != nil {
		updatingState.unsetUpdating()
	}
	return err
}

func (*deviceService) UpdateTo(ipc.ServerContext, string) error {
	// TODO(jsimsa): Implement.
	return nil
}

func (s *deviceService) SetACL(ctx ipc.ServerContext, acl access.TaggedACLMap, etag string) error {
	p := ctx.LocalPrincipal()
	d := aclDir(s.disp.config)
	return s.disp.locks.SetPathACL(p, d, acl, etag)
}

func (s *deviceService) GetACL(ctx ipc.ServerContext) (acl access.TaggedACLMap, etag string, err error) {
	p := ctx.LocalPrincipal()
	d := aclDir(s.disp.config)
	return s.disp.locks.GetPathACL(p, d)
}

// TODO(rjkroege): Make it possible for users on the same system to also
// associate their accounts with their identities.
func (s *deviceService) AssociateAccount(call ipc.ServerContext, identityNames []string, accountName string) error {
	if accountName == "" {
		return s.uat.DisassociateSystemAccountForBlessings(identityNames)
	}
	// TODO(rjkroege): Optionally verify here that the required uname is a valid.
	return s.uat.AssociateSystemAccountForBlessings(identityNames, accountName)
}

func (s *deviceService) ListAssociations(call ipc.ServerContext) (associations []device.Association, err error) {
	// Temporary code. Dump this.
	if vlog.V(2) {
		b, r := call.RemoteBlessings().ForContext(call)
		vlog.Infof("ListAssociations given blessings: %v\n", b)
		if len(r) > 0 {
			vlog.Infof("ListAssociations rejected blessings: %v\n", r)
		}
	}
	return s.uat.AllBlessingSystemAssociations()
}

func (*deviceService) Debug(ipc.ServerContext) (string, error) {
	return "Not implemented", nil
}
