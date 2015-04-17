// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rt_test

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"regexp"
	"testing"
	"time"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/security"
	"v.io/x/lib/vlog"
	"v.io/x/ref/envvar"
	vsecurity "v.io/x/ref/lib/security"
	"v.io/x/ref/test"
	"v.io/x/ref/test/expect"
	"v.io/x/ref/test/modules"
)

//go:generate v23 test generate

func TestInit(t *testing.T) {
	envvar.ClearCredentials()
	ctx, shutdown := v23.Init()
	defer shutdown()

	l := vlog.Log
	fmt.Println(l)
	args := fmt.Sprintf("%s", l)
	expected := regexp.MustCompile("name=vanadium logdirs=\\[/tmp\\] logtostderr=true|false alsologtostderr=false|true max_stack_buf_size=4292608 v=[0-9] stderrthreshold=2 vmodule= log_backtrace_at=:0")
	if !expected.MatchString(args) {
		t.Errorf("unexpected default args: %s", args)
	}
	p := v23.GetPrincipal(ctx)
	if p == nil {
		t.Fatalf("A new principal should have been created")
	}
	if p.BlessingStore() == nil {
		t.Fatalf("The principal must have a BlessingStore")
	}
	if p.BlessingStore().Default().IsZero() {
		t.Errorf("Principal().BlessingStore().Default() should not be the zero value")
	}
	if p.BlessingStore().ForPeer().IsZero() {
		t.Errorf("Principal().BlessingStore().ForPeer() should not be the zero value")
	}
}

func child(stdin io.Reader, stdout, stderr io.Writer, env map[string]string, args ...string) error {
	_, shutdown := test.InitForTest()
	defer shutdown()

	logger := vlog.Log
	vlog.Infof("%s\n", logger)
	fmt.Fprintf(stdout, "%s\n", logger)
	modules.WaitForEOF(stdin)
	fmt.Fprintf(stdout, "done\n")
	return nil
}

func TestInitArgs(t *testing.T) {
	sh, err := modules.NewShell(nil, nil, testing.Verbose(), t)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	defer sh.Cleanup(os.Stderr, os.Stderr)
	h, err := sh.Start("child", nil, "--logtostderr=true", "--vmodule=*=3", "--", "foobar")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	h.Expect(fmt.Sprintf("name=vanadium "+
		"logdirs=[%s] "+
		"logtostderr=true "+
		"alsologtostderr=true "+
		"max_stack_buf_size=4292608 "+
		"v=0 "+
		"stderrthreshold=2 "+
		"vmodule=*=3 "+
		"log_backtrace_at=:0",
		os.TempDir()))
	h.CloseStdin()
	h.Expect("done")
	h.ExpectEOF()
	h.Shutdown(os.Stderr, os.Stderr)
}

func validatePrincipal(p security.Principal) error {
	if p == nil {
		return fmt.Errorf("nil principal")
	}
	call := security.NewCall(&security.CallParams{LocalPrincipal: p, RemoteBlessings: p.BlessingStore().Default()})
	ctx, cancel := context.RootContext()
	defer cancel()
	blessings, rejected := security.RemoteBlessingNames(ctx, call)
	if n := len(blessings); n != 1 {
		return fmt.Errorf("rt.Principal().BlessingStore().Default() return blessings:%v (rejected:%v), want exactly one recognized blessing", blessings, rejected)
	}
	return nil
}

func defaultBlessing(p security.Principal) string {
	call := security.NewCall(&security.CallParams{LocalPrincipal: p, RemoteBlessings: p.BlessingStore().Default()})
	ctx, cancel := context.RootContext()
	defer cancel()
	b, _ := security.RemoteBlessingNames(ctx, call)
	return b[0]
}

func tmpDir(t *testing.T) string {
	dir, err := ioutil.TempDir("", "rt_test_dir")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	return dir
}

func principal(stdin io.Reader, stdout, stderr io.Writer, env map[string]string, args ...string) error {
	ctx, shutdown := test.InitForTest()
	defer shutdown()

	p := v23.GetPrincipal(ctx)
	if err := validatePrincipal(p); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "DEFAULT_BLESSING=%s\n", defaultBlessing(p))
	return nil
}

// Runner runs a principal as a subprocess and reports back with its
// own security info and it's childs.
func runner(stdin io.Reader, stdout, stderr io.Writer, env map[string]string, args ...string) error {
	ctx, shutdown := test.InitForTest()
	defer shutdown()

	p := v23.GetPrincipal(ctx)
	if err := validatePrincipal(p); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "RUNNER_DEFAULT_BLESSING=%v\n", defaultBlessing(p))
	sh, err := modules.NewShell(ctx, p, false, nil)
	if err != nil {
		return err
	}
	if _, err := sh.Start("principal", nil, args...); err != nil {
		return err
	}
	// Cleanup copies the output of sh to these Writers.
	sh.Cleanup(stdout, stderr)
	return nil
}

func createCredentialsInDir(t *testing.T, dir string, blessing string) {
	principal, err := vsecurity.CreatePersistentPrincipal(dir, nil)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if err := vsecurity.InitDefaultBlessings(principal, blessing); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
}

func TestPrincipalInheritance(t *testing.T) {
	sh, err := modules.NewShell(nil, nil, testing.Verbose(), t)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	defer func() {
		sh.Cleanup(os.Stdout, os.Stderr)
	}()

	// Test that the child inherits from the parent's credentials correctly.
	// The running test process may or may not have a credentials directory set
	// up so we have to use a 'runner' process to ensure the correct setup.
	cdir := tmpDir(t)
	defer os.RemoveAll(cdir)

	createCredentialsInDir(t, cdir, "test")

	// directory supplied by the environment.
	credEnv := []string{envvar.Credentials + "=" + cdir}

	h, err := sh.Start("runner", credEnv)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	runnerBlessing := h.ExpectVar("RUNNER_DEFAULT_BLESSING")
	principalBlessing := h.ExpectVar("DEFAULT_BLESSING")
	if err := h.Error(); err != nil {
		t.Fatalf("failed to read input from children: %s", err)
	}
	h.Shutdown(os.Stdout, os.Stderr)

	wantRunnerBlessing := "test"
	wantPrincipalBlessing := "test/child"
	if runnerBlessing != wantRunnerBlessing || principalBlessing != wantPrincipalBlessing {
		t.Fatalf("unexpected default blessing: got runner %s, principal %s, want runner %s, principal %s", runnerBlessing, principalBlessing, wantRunnerBlessing, wantPrincipalBlessing)
	}

}

func TestPrincipalInit(t *testing.T) {
	// Collect the process' public key and error status
	collect := func(sh *modules.Shell, env []string, args ...string) string {
		h, err := sh.Start("principal", env, args...)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		s := expect.NewSession(t, h.Stdout(), time.Minute)
		s.SetVerbosity(testing.Verbose())
		return s.ExpectVar("DEFAULT_BLESSING")
	}

	// A credentials directory may, or may, not have been already specified.
	// Either way, we want to use our own, so we set it aside and use our own.
	origCredentialsDir := os.Getenv(envvar.Credentials)
	defer os.Setenv(envvar.Credentials, origCredentialsDir)
	if err := os.Setenv(envvar.Credentials, ""); err != nil {
		t.Fatal(err)
	}

	// We create two shells -- one initializing the principal for a child process
	// via a credentials directory and the other via an agent.
	sh, err := modules.NewShell(nil, nil, testing.Verbose(), t)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	defer sh.Cleanup(os.Stderr, os.Stderr)

	ctx, shutdown := test.InitForTest()
	defer shutdown()

	agentSh, err := modules.NewShell(ctx, v23.GetPrincipal(ctx), testing.Verbose(), t)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	defer agentSh.Cleanup(os.Stderr, os.Stderr)

	// Test that with envvar.Credentials unset the runtime's Principal
	// is correctly initialized for both shells.
	if len(collect(sh, nil)) == 0 {
		t.Fatalf("Without agent: child returned an empty default blessings set")
	}
	if got, want := collect(agentSh, nil), test.TestBlessing+security.ChainSeparator+"child"; got != want {
		t.Fatalf("With agent: got %q, want %q", got, want)
	}

	// Test that credentials specified via the envvar.Credentials
	// environment variable take precedence over an agent.
	cdir1 := tmpDir(t)
	defer os.RemoveAll(cdir1)
	createCredentialsInDir(t, cdir1, "test_env")
	credEnv := []string{envvar.Credentials + "=" + cdir1}

	if got, want := collect(sh, credEnv), "test_env"; got != want {
		t.Errorf("Without agent: got default blessings: %q, want %q", got, want)
	}
	if got, want := collect(agentSh, credEnv), "test_env"; got != want {
		t.Errorf("With agent: got default blessings: %q, want %q", got, want)
	}

	// Test that credentials specified via the command line take precedence over the
	// envvar.Credentials environment variable and also the agent.
	cdir2 := tmpDir(t)
	defer os.RemoveAll(cdir2)
	createCredentialsInDir(t, cdir2, "test_cmd")

	if got, want := collect(sh, credEnv, "--v23.credentials="+cdir2), "test_cmd"; got != want {
		t.Errorf("Without agent: got %q, want %q", got, want)
	}
	if got, want := collect(agentSh, credEnv, "--v23.credentials="+cdir2), "test_cmd"; got != want {
		t.Errorf("With agent: got %q, want %q", got, want)
	}
}
