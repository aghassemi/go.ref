package core_test

import (
	"bytes"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strconv"
	"testing"

	"v.io/x/lib/vlog"

	"v.io/x/ref/lib/flags/consts"
	_ "v.io/x/ref/profiles"
	"v.io/x/ref/test"
	"v.io/x/ref/test/modules"
	"v.io/x/ref/test/modules/core"
)

// We create our own TestMain here because v23 test generate currently does not
// recognize that this requires modules.Dispatch().
func TestMain(m *testing.M) {
	test.Init()
	if modules.IsModulesChildProcess() {
		if err := modules.Dispatch(); err != nil {
			fmt.Fprintf(os.Stderr, "modules.Dispatch failed: %v\n", err)
			os.Exit(1)
		}
		return
	}
	os.Exit(m.Run())
}

func TestCommands(t *testing.T) {
	sh, fn := newShell(t)
	defer fn()
	for _, c := range []string{core.RootMTCommand, core.MTCommand} {
		if len(sh.Help(c)) == 0 {
			t.Fatalf("missing command %q", c)
		}
	}
}

// TODO(cnicolaou): add test for proxyd

func newShell(t *testing.T) (*modules.Shell, func()) {
	ctx, shutdown := test.InitForTest()
	sh, err := modules.NewShell(ctx, nil, testing.Verbose(), t)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	return sh, func() {
		if testing.Verbose() {
			sh.Cleanup(os.Stderr, os.Stderr)
		} else {
			sh.Cleanup(nil, nil)
		}
		shutdown()
	}
}

func testArgs(args ...string) []string {
	var targs = []string{"--veyron.tcp.address=127.0.0.1:0"}
	return append(targs, args...)
}

func TestRoot(t *testing.T) {
	sh, fn := newShell(t)
	defer fn()
	root, err := sh.Start(core.RootMTCommand, nil, testArgs()...)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	root.ExpectVar("PID")
	root.ExpectVar("MT_NAME")
	root.CloseStdin()
}

func startMountTables(t *testing.T, sh *modules.Shell, mountPoints ...string) (map[string]string, func(), error) {
	// Start root mount table
	root, err := sh.Start(core.RootMTCommand, nil, testArgs()...)
	if err != nil {
		t.Fatalf("unexpected error for root mt: %s", err)
	}
	sh.Forget(root)
	root.ExpectVar("PID")
	rootName := root.ExpectVar("MT_NAME")
	if t.Failed() {
		return nil, nil, root.Error()
	}
	sh.SetVar(consts.NamespaceRootPrefix, rootName)
	mountAddrs := make(map[string]string)
	mountAddrs["root"] = rootName

	// Start the mount tables
	for _, mp := range mountPoints {
		h, err := sh.Start(core.MTCommand, nil, testArgs(mp)...)
		if err != nil {
			return nil, nil, fmt.Errorf("unexpected error for mt %q: %s", mp, err)
		}
		// Wait until each mount table has at least called Serve to
		// mount itself.
		h.ExpectVar("PID")
		mountAddrs[mp] = h.ExpectVar("MT_NAME")
		if h.Failed() {
			return nil, nil, h.Error()
		}
	}
	deferFn := func() {
		if testing.Verbose() {
			vlog.Infof("------ root shutdown ------")
			root.Shutdown(os.Stderr, os.Stderr)
		} else {
			root.Shutdown(nil, nil)
		}
	}
	return mountAddrs, deferFn, nil
}

func getMatchingMountpoint(r [][]string) string {
	if len(r) != 1 {
		return ""
	}
	shortest := ""
	for _, p := range r[0][1:] {
		if len(p) > 0 {
			if len(shortest) == 0 {
				shortest = p
			}
			if len(shortest) > 0 && len(p) < len(shortest) {
				shortest = p
			}
		}
	}
	return shortest
}

func TestMountTableAndGlob(t *testing.T) {
	sh, fn := newShell(t)
	defer fn()

	mountPoints := []string{"a", "b", "c", "d", "e"}
	_, fn, err := startMountTables(t, sh, mountPoints...)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	defer fn()

	// Run the ls command in a subprocess, with consts.NamespaceRootPrefix as set above.
	lse, err := sh.Start(core.LSCommand, nil, "...")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if got, want := lse.ExpectVar("RN"), strconv.Itoa(len(mountPoints)); got != want {
		t.Fatalf("got %v, want %v", got, want)
	}

	pattern := ""
	for _, n := range mountPoints {
		// Since the LSExternalCommand runs in a subprocess with
		// consts.NamespaceRootPrefix set to the name of the root mount
		// table it sees to the relative name format of the mounted
		// mount tables.
		pattern = pattern + "^R[\\d]+=(" + n + "$)|"
	}
	pattern = pattern[:len(pattern)-1]
	found := []string{}
	for i := 0; i < len(mountPoints); i++ {
		found = append(found, getMatchingMountpoint(lse.ExpectRE(pattern, 1)))
	}
	sort.Strings(found)
	sort.Strings(mountPoints)
	if got, want := found, mountPoints; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestEcho(t *testing.T) {
	sh, fn := newShell(t)
	defer fn()

	srv, err := sh.Start(core.EchoServerCommand, nil, testArgs("test", "")...)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	srv.ExpectVar("PID")
	name := srv.ExpectVar("NAME")
	if len(name) == 0 {
		t.Fatalf("failed to get name")
	}
	clt, err := sh.Start(core.EchoClientCommand, nil, name, "a message")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	clt.Expect("test: a message")
}

func TestExec(t *testing.T) {
	sh, cleanup := newShell(t)
	defer cleanup()
	h, err := sh.StartWithOpts(sh.DefaultStartOpts().NoExecCommand(), nil, "/bin/sh", "-c", "echo hello world")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	output := bytes.NewBuffer([]byte{})
	if _, err := output.ReadFrom(h.Stdout()); err != nil {
		t.Fatalf("could not read output from command: %v", err)
	}
	if got, want := output.String(), "hello world\n"; got != want {
		t.Fatalf("unexpected output: got %v, want %v", got, want)
	}
}

func TestExecWithEnv(t *testing.T) {
	sh, cleanup := newShell(t)
	defer cleanup()
	h, err := sh.StartWithOpts(sh.DefaultStartOpts().NoExecCommand(), []string{"BLAH=hello world"}, "/bin/sh", "-c", "printenv BLAH")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	output := bytes.NewBuffer([]byte{})
	if _, err := output.ReadFrom(h.Stdout()); err != nil {
		t.Fatalf("could not read output from command: %v", err)
	}
	if got, want := output.String(), "hello world\n"; got != want {
		t.Fatalf("unexpected output: got %v, want %v", got, want)
	}
}
