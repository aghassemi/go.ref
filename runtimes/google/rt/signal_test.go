package rt_test

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"syscall"
	"testing"
	"time"

	"veyron.io/veyron/veyron2"
	"veyron.io/veyron/veyron2/rt"

	"veyron.io/veyron/veyron/lib/expect"
	"veyron.io/veyron/veyron/lib/modules"
)

func init() {
	modules.RegisterChild("withRuntime", "", withRuntime)
	modules.RegisterChild("withoutRuntime", "", withoutRuntime)
}

func simpleEchoProgram(stdin io.Reader, stdout io.Writer) {
	fmt.Fprintf(stdout, "ready\n")
	scanner := bufio.NewScanner(stdin)
	if scanner.Scan() {
		fmt.Fprintf(stdout, "%s\n", scanner.Text())
	}
	modules.WaitForEOF(stdin)
}

func withRuntime(stdin io.Reader, stdout, stderr io.Writer, env map[string]string, args ...string) error {
	// Make sure that we use "google" runtime implementation in this
	// package even though we have to use the public API which supports
	// arbitrary runtime implementations.
	rt.Init(veyron2.RuntimeOpt{veyron2.GoogleRuntimeName})
	simpleEchoProgram(stdin, stdout)
	return nil
}

func withoutRuntime(stdin io.Reader, stdout, stderr io.Writer, env map[string]string, args ...string) error {
	simpleEchoProgram(stdin, stdout)
	return nil
}

func TestWithRuntime(t *testing.T) {
	sh := modules.NewShell("withRuntime")
	defer sh.Cleanup(os.Stderr, os.Stderr)
	h, err := sh.Start("withRuntime")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	defer h.Shutdown(os.Stderr, os.Stderr)
	s := expect.NewSession(t, h.Stdout(), time.Minute)
	s.Expect("ready")
	syscall.Kill(h.Pid(), syscall.SIGHUP)
	h.Stdin().Write([]byte("foo\n"))
	s.Expect("foo")
	h.CloseStdin()
	s.ExpectEOF()
}

func TestWithoutRuntime(t *testing.T) {
	sh := modules.NewShell("withoutRuntime")
	defer sh.Cleanup(os.Stderr, os.Stderr)
	h, err := sh.Start("withoutRuntime")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	defer h.Shutdown(os.Stderr, os.Stderr)
	s := expect.NewSession(t, h.Stdout(), time.Minute)
	s.Expect("ready")
	syscall.Kill(h.Pid(), syscall.SIGHUP)
	s.ExpectEOF()
	err = h.Shutdown(os.Stderr, os.Stderr)
	want := "exit status 2"
	if err == nil || err.Error() != want {
		t.Errorf("got %s, want %s", err, want)

	}
}
