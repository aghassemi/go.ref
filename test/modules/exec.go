// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package modules

import (
	"flag"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"

	"v.io/x/lib/envvar"
	"v.io/x/lib/vlog"

	"v.io/v23/verror"

	"v.io/x/ref/internal/logger"
	vexec "v.io/x/ref/lib/exec"
	"v.io/x/ref/lib/mgmt"
	"v.io/x/ref/services/agent/agentlib"
	"v.io/x/ref/test/expect"
)

// execHandle implements both the Handle interface.
type execHandle struct {
	*expect.Session
	mu         sync.Mutex
	cmd        *exec.Cmd
	entryPoint string
	desc       string
	handle     *vexec.ParentHandle
	sh         *Shell
	stderr     *os.File
	stdout     io.ReadCloser
	stdin      io.WriteCloser
	procErrCh  chan error
	opts       *StartOpts
	external   bool
}

func testFlags() []string {
	var fl []string
	// pass logging flags to any subprocesses
	flags := logger.Manager(logger.Global()).ExplicitlySetFlags()
	for fname, fval := range flags {
		fl = append(fl, "--"+fname+"="+fval)
	}
	timeout := flag.Lookup("test.timeout")
	if timeout == nil {
		// not a go test binary
		return fl
	}
	// must be a go test binary
	val := timeout.Value.(flag.Getter).Get().(time.Duration)
	if val.String() != timeout.DefValue {
		// use supplied value for subprocesses
		fl = append(fl, "--test.timeout="+timeout.Value.String())
	} else {
		// translate default value into 3m for subproccesses.  The
		// default of 10m is too long to wait in order to find out that
		// our subprocess is wedged.
		fl = append(fl, "--test.timeout=3m")
	}
	return fl
}

func newExecHandle(entry, desc string) *execHandle {
	return &execHandle{entryPoint: entry, desc: desc, procErrCh: make(chan error, 1)}
}

func newExecHandleExternal(prog string) *execHandle {
	return &execHandle{entryPoint: prog, desc: prog, procErrCh: make(chan error, 1), external: true}
}

func (eh *execHandle) Stdout() io.Reader {
	eh.mu.Lock()
	defer eh.mu.Unlock()
	return eh.stdout
}

func (eh *execHandle) Stderr() io.Reader {
	eh.mu.Lock()
	defer eh.mu.Unlock()
	return eh.stderr
}

func (eh *execHandle) Stdin() io.Writer {
	eh.mu.Lock()
	defer eh.mu.Unlock()
	return eh.stdin
}

func (eh *execHandle) CloseStdin() {
	eh.mu.Lock()
	eh.stdin.Close()
	eh.mu.Unlock()
}

func (eh *execHandle) envelope(sh *Shell, env []string, args []string) ([]string, []string) {
	if eh.external {
		newargs := append([]string{eh.entryPoint}, args...)
		newenv := envvar.SliceToMap(env)
		delete(newenv, shellEntryPoint)
		return newargs, envvar.MapToSlice(newenv)
	}
	newargs := append([]string{os.Args[0]}, testFlags()...)
	newargs = append(newargs, args...)
	newenv := envvar.SliceToMap(env)
	newenv[shellEntryPoint] = eh.entryPoint
	return newargs, envvar.MapToSlice(newenv)
}

func (eh *execHandle) start(sh *Shell, agentfd *os.File, opts *StartOpts, env []string, args []string) (*execHandle, error) {
	eh.mu.Lock()
	defer eh.mu.Unlock()
	eh.sh = sh
	eh.opts = opts
	args, env = eh.envelope(sh, env, args)
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Env = env

	stderr, err := newLogfile("stderr", eh.entryPoint)
	if err != nil {
		return nil, err
	}
	cmd.Stderr = stderr
	// We use a custom queue-based Writer implementation for stdout to
	// decouple the consumers of eh.stdout from the file where the child
	// sends its output.  This avoids data races between closing the file
	// and reading from it (since cmd.Wait will wait for the all readers to
	// be done before closing it).  It also enables Shutdown to drain stdout
	// while respecting the timeout.
	stdout := newRW()
	cmd.Stdout = stdout

	// If we have an explicit stdin to pass to the child, use that,
	// otherwise create a pipe and return the write side of that pipe
	// in the handle.
	if eh.opts.Stdin != nil {
		cmd.Stdin = eh.opts.Stdin
	} else {
		stdin, err := cmd.StdinPipe()
		if err != nil {
			return nil, err
		}
		eh.stdin = stdin
	}
	config := vexec.NewConfig()

	execOpts := []vexec.ParentHandleOpt{}
	if !eh.opts.ExecProtocol {
		execOpts = append(execOpts, vexec.UseExecProtocolOpt(false))
	} else {
		serialized, err := sh.config.Serialize()
		if err != nil {
			return nil, err
		}
		config.MergeFrom(serialized)
		if agentfd != nil {
			childfd := len(cmd.ExtraFiles) + vexec.FileOffset
			config.Set(mgmt.SecurityAgentEndpointConfigKey, agentlib.AgentEndpoint(childfd))
			cmd.ExtraFiles = append(cmd.ExtraFiles, agentfd)
			defer agentfd.Close()
		}
		execOpts = append(execOpts, vexec.ConfigOpt{Config: config})
	}

	// TODO(cnicolaou): for external programs, vexec should either not be
	// used or it should taken an option to not use its protocol, and in
	// particular to share secrets with children.
	handle := vexec.NewParentHandle(cmd, execOpts...)
	eh.stdout = stdout
	eh.stderr = stderr
	eh.handle = handle
	eh.cmd = cmd
	vlog.VI(1).Infof("Start: %q stderr: %s", eh.desc, stderr.Name())
	vlog.VI(1).Infof("Start: %q args: %v", eh.desc, cmd.Args)
	vlog.VI(2).Infof("Start: %q env: %v", eh.desc, cmd.Env)
	if err := handle.Start(); err != nil {
		// The child process failed to start, either because of some setup
		// error (e.g. creating pipes for it to use), or a bad binary etc.
		// A handle is returned, so that Shutdown etc may be called, hence
		// the error must be sent over eh.procErrCh to allow Shutdown to
		// terminate.
		eh.procErrCh <- err
		return eh, err
	}
	if eh.opts.ExecProtocol {
		if err := eh.handle.WaitForReady(eh.opts.StartTimeout); err != nil {
			// The child failed to call SetReady, most likely because of bad
			// command line arguments or some other early exit in the child
			// process.
			// As per Start above, a handle is returned and the error
			// sent over eh.procErrCh.
			eh.procErrCh <- err
			return eh, err
		}
	}
	vlog.VI(1).Infof("Started: %q, pid %d", eh.desc, cmd.Process.Pid)
	go func() {
		eh.procErrCh <- eh.handle.Wait(0)
		// It's now safe to close eh.stdout, since Wait only returns
		// once all writes from the pipe to the stdout Writer have
		// completed.  Closing eh.stdout lets consumers of stdout wrap
		// up (they'll receive EOF).
		eh.stdout.Close()
	}()
	eh.Session = expect.NewSession(opts.ExpectTesting, stdout, opts.ExpectTimeout)
	eh.Session.SetVerbosity(eh.sh.sessionVerbosity)
	return eh, nil
}

func (eh *execHandle) Pid() int {
	return eh.cmd.Process.Pid
}

func (eh *execHandle) Shutdown(stdout, stderr io.Writer) error {
	eh.mu.Lock()
	defer eh.mu.Unlock()
	vlog.VI(1).Infof("Shutdown: %q", eh.desc)
	defer vlog.VI(1).Infof("Shutdown: %q [DONE]", eh.desc)
	if eh.stdin != nil {
		eh.stdin.Close()
	}
	defer eh.sh.Forget(eh)

	waitStdout := make(chan struct{})
	if stdout != nil {
		// Drain stdout.
		go func() {
			io.Copy(stdout, eh.stdout)
			close(waitStdout)
		}()
	} else {
		close(waitStdout)
	}

	var procErr error
	select {
	case procErr = <-eh.procErrCh:
		// The child has exited already.
	case <-time.After(eh.opts.ShutdownTimeout):
		// Time out waiting for child to exit.
		procErr = verror.New(vexec.ErrTimeout, nil)
		// Force close stdout to unblock any readers of stdout
		// (including the drain loop started above).
		eh.stdout.Close()
	}
	<-waitStdout

	// Transcribe stderr.
	outputFromFile(eh.stderr, stderr)
	os.Remove(eh.stderr.Name())

	return procErr
}
