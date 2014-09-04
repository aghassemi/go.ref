package impl_test

import (
	"fmt"
	"os"
	"testing"

	mtlib "veyron/services/mounttable/lib"

	"veyron/lib/testutil/blackbox"
	"veyron/lib/testutil/security"
	"veyron/services/mgmt/lib/exec"

	"veyron2"
	"veyron2/ipc"
	"veyron2/naming"
	"veyron2/rt"
	"veyron2/services/mgmt/node"
	"veyron2/verror"
	"veyron2/vlog"
)

// TODO(caprita): I've had to write one too many of these, let's move it to some
// central utility library.

// setupLocalNamespace sets up a mounttable and sets the local namespace root
// to point to it.  Returns a cleanup function.
func setupLocalNamespace(t *testing.T) func() {
	server, err := rt.R().NewServer(veyron2.ServesMountTableOpt(true))
	if err != nil {
		t.Fatalf("NewServer() failed: %v", err)
	}
	dispatcher, err := mtlib.NewMountTable("")
	if err != nil {
		t.Fatalf("NewMountTable() failed: %v", err)
	}
	protocol, hostname := "tcp", "127.0.0.1:0"
	endpoint, err := server.Listen(protocol, hostname)
	if err != nil {
		t.Fatalf("Listen(%v, %v) failed: %v", protocol, hostname, err)
	}
	if err := server.Serve("", dispatcher); err != nil {
		t.Fatalf("Serve(%v) failed: %v", dispatcher, err)
	}
	name := naming.JoinAddressName(endpoint.String(), "")
	vlog.VI(1).Infof("Mount table object name: %v", name)
	ns := rt.R().Namespace()
	// Make the runtime's namespace rooted at the MountTable server started
	// above.
	ns.SetRoots(name)
	return func() {
		if err := server.Stop(); err != nil {
			t.Fatalf("Stop() failed: %v", err)
		}
		// The runtime outlives the particular test case that invokes
		// setupLocalNamespace. It's good practice to reset the
		// runtime's state before the next test uses it.
		ns.SetRoots()
	}
}

// TODO(caprita): Move this setup into the blackbox lib.

// setupChildCommand configures the child to use the right mounttable root
// and identity.  It returns a cleanup function.
func setupChildCommand(child *blackbox.Child) func() {
	cmd := child.Cmd
	for i, root := range rt.R().Namespace().Roots() {
		cmd.Env = exec.Setenv(cmd.Env, fmt.Sprintf("NAMESPACE_ROOT%d", i), root)
	}
	idFile := security.SaveIdentityToFile(security.NewBlessedIdentity(rt.R().Identity(), "test"))
	cmd.Env = exec.Setenv(cmd.Env, "VEYRON_IDENTITY", idFile)
	return func() {
		os.Remove(idFile)
	}
}

func newServer() (ipc.Server, string) {
	server, err := rt.R().NewServer()
	if err != nil {
		vlog.Fatalf("NewServer() failed: %v", err)
	}
	protocol, hostname := "tcp", "127.0.0.1:0"
	endpoint, err := server.Listen(protocol, hostname)
	if err != nil {
		vlog.Fatalf("Listen(%v, %v) failed: %v", protocol, hostname, err)
	}
	return server, endpoint.String()
}

// resolveExpectError verifies that the given name is not in the mounttable.
func resolveExpectNotFound(t *testing.T, name string) {
	if results, err := rt.R().Namespace().Resolve(rt.R().NewContext(), name); err == nil {
		t.Fatalf("Resolve(%v) succeeded with results %v when it was expected to fail", name, results)
	} else if expectErr := verror.NotFound; !verror.Is(err, expectErr) {
		t.Fatalf("Resolve(%v) failed with error %v, expected error ID %v", err, expectErr)
	}
}

// resolve looks up the given name in the mounttable.
func resolve(t *testing.T, name string, replicas int) []string {
	results, err := rt.R().Namespace().Resolve(rt.R().NewContext(), name)
	if err != nil {
		t.Fatalf("Resolve(%v) failed: %v", name, err)
	}
	if want, got := replicas, len(results); want != got {
		t.Fatalf("Resolve(%v) expected %d result(s), got %d instead", name, want, got)
	}
	return results
}

// The following set of functions are convenience wrappers around Update and
// Revert for node manager.

func nodeStub(t *testing.T, name string) node.Node {
	nodeName := naming.Join(name, "nm")
	stub, err := node.BindNode(nodeName)
	if err != nil {
		t.Fatalf("BindNode(%v) failed: %v", nodeName, err)
	}
	return stub
}

func updateNodeExpectError(t *testing.T, name string, errID verror.ID) {
	if err := nodeStub(t, name).Update(rt.R().NewContext()); !verror.Is(err, errID) {
		t.Fatalf("Update(%v) expected to fail with %v, got %v instead", name, errID, err)
	}
}

func updateNode(t *testing.T, name string) {
	if err := nodeStub(t, name).Update(rt.R().NewContext()); err != nil {
		t.Fatalf("Update(%v) failed: %v", name, err)
	}
}

func revertNodeExpectError(t *testing.T, name string, errID verror.ID) {
	if err := nodeStub(t, name).Revert(rt.R().NewContext()); !verror.Is(err, errID) {
		t.Fatalf("Revert(%v) expected to fail with %v, got %v instead", name, errID, err)
	}
}

func revertNode(t *testing.T, name string) {
	if err := nodeStub(t, name).Revert(rt.R().NewContext()); err != nil {
		t.Fatalf("Revert(%v) failed: %v", name, err)
	}
}

// The following set of functions are convenience wrappers around various app
// management methods.

func appStub(t *testing.T, nameComponents ...string) node.Application {
	appsName := "nm//apps"
	appName := naming.Join(append([]string{appsName}, nameComponents...)...)
	stub, err := node.BindApplication(appName)
	if err != nil {
		t.Fatalf("BindApplication(%v) failed: %v", appName, err)
	}
	return stub
}

func installApp(t *testing.T) string {
	appID, err := appStub(t).Install(rt.R().NewContext(), "ar")
	if err != nil {
		t.Fatalf("Install failed: %v", err)
	}
	return appID
}

func startAppImpl(t *testing.T, appID string) (string, error) {
	if instanceIDs, err := appStub(t, appID).Start(rt.R().NewContext()); err != nil {
		return "", err
	} else {
		if want, got := 1, len(instanceIDs); want != got {
			t.Fatalf("Start(%v): expected %v instance ids, got %v instead", appID, want, got)
		}
		return instanceIDs[0], nil
	}
}

func startApp(t *testing.T, appID string) string {
	instanceID, err := startAppImpl(t, appID)
	if err != nil {
		t.Fatalf("Start(%v) failed: %v", appID, err)
	}
	return instanceID
}

func startAppExpectError(t *testing.T, appID string, expectedError verror.ID) {
	if _, err := startAppImpl(t, appID); err == nil || !verror.Is(err, expectedError) {
		t.Fatalf("Start(%v) expected to fail with %v, got %v instead", appID, expectedError, err)
	}
}

func stopApp(t *testing.T, appID, instanceID string) {
	if err := appStub(t, appID, instanceID).Stop(rt.R().NewContext(), 5); err != nil {
		t.Fatalf("Stop(%v/%v) failed: %v", appID, instanceID, err)
	}
}

func suspendApp(t *testing.T, appID, instanceID string) {
	if err := appStub(t, appID, instanceID).Suspend(rt.R().NewContext()); err != nil {
		t.Fatalf("Suspend(%v/%v) failed: %v", appID, instanceID, err)
	}
}

func resumeApp(t *testing.T, appID, instanceID string) {
	if err := appStub(t, appID, instanceID).Resume(rt.R().NewContext()); err != nil {
		t.Fatalf("Resume(%v/%v) failed: %v", appID, instanceID, err)
	}
}

func updateApp(t *testing.T, appID string) {
	if err := appStub(t, appID).Update(rt.R().NewContext()); err != nil {
		t.Fatalf("Update(%v) failed: %v", appID, err)
	}
}

func updateAppExpectError(t *testing.T, appID string, expectedError verror.ID) {
	if err := appStub(t, appID).Update(rt.R().NewContext()); err == nil || !verror.Is(err, expectedError) {
		t.Fatalf("Update(%v) expected to fail with %v, got %v instead", appID, expectedError, err)
	}
}

func revertApp(t *testing.T, appID string) {
	if err := appStub(t, appID).Revert(rt.R().NewContext()); err != nil {
		t.Fatalf("Revert(%v) failed: %v", appID, err)
	}
}

func revertAppExpectError(t *testing.T, appID string, expectedError verror.ID) {
	if err := appStub(t, appID).Revert(rt.R().NewContext()); err == nil || !verror.Is(err, expectedError) {
		t.Fatalf("Revert(%v) expected to fail with %v, got %v instead", appID, expectedError, err)
	}
}

func uninstallApp(t *testing.T, appID string) {
	if err := appStub(t, appID).Uninstall(rt.R().NewContext()); err != nil {
		t.Fatalf("Uninstall(%v) failed: %v", appID, err)
	}
}
