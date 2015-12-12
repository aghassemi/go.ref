// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package v23test defines Shell, a wrapper around gosh.Shell that provides
// Vanadium-specific functionality such as credentials management,
// JiriBuildGoPkg, and StartRootMountTable.
package v23test

// TODO(sadovsky): Add DebugSystemShell command.

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"runtime/debug"
	"testing"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/rpc"
	"v.io/x/lib/gosh"
	"v.io/x/ref"
	"v.io/x/ref/test"
)

const (
	envBinDir           = "V23_BIN_DIR"
	envChildOutputDir   = "TMPDIR"
	envShellTestProcess = "V23_SHELL_TEST_PROCESS"
)

// Cmd wraps gosh.Cmd and provides Vanadium-specific functionality.
type Cmd struct {
	*gosh.Cmd
}

// WithCredentials configures this command to use the given credentials.
func (c *Cmd) WithCredentials(cr *Credentials) *Cmd {
	c.Cmd.Vars[ref.EnvCredentials] = cr.Handle
	return c
}

// Shell wraps gosh.Shell and provides Vanadium-specific functionality.
type Shell struct {
	*gosh.Shell
	Ctx         *context.T
	Credentials *Credentials
}

// Opts augments gosh.Opts with Vanadium-specific options. See gosh.Opts for
// field descriptions.
type Opts struct {
	Errorf              func(format string, v ...interface{})
	Logf                func(format string, v ...interface{})
	SuppressChildOutput bool
	ChildOutputDir      string
	BinDir              string
	// Large means this test should be run iff the -v23.tests flag was specified.
	// Only takes effect if a *testing.T was provided.
	Large bool
}

var calledRun = false

func fillDefaults(t *testing.T, opts *Opts) {
	if opts.Errorf == nil {
		if t != nil {
			opts.Errorf = func(format string, v ...interface{}) {
				debug.PrintStack()
				t.Fatalf(format, v...)
			}
		} else {
			opts.Errorf = func(format string, v ...interface{}) {
				panic(fmt.Sprintf(format, v...))
			}
		}
	}
	if opts.Logf == nil {
		if t != nil {
			opts.Logf = t.Logf
		} else {
			opts.Logf = log.Printf
		}
	}
	if opts.ChildOutputDir == "" {
		opts.ChildOutputDir = os.Getenv(envChildOutputDir)
	}
	if opts.BinDir == "" {
		opts.BinDir = os.Getenv(envBinDir)
	}
}

// NewShell creates a new Shell. 't' may be nil. Use v23.GetPrincipal(sh.Ctx) to
// get the bound principal, if needed.
func NewShell(t *testing.T, opts Opts) *Shell {
	fillDefaults(t, &opts)

	if t != nil {
		if opts.Large && !test.IntegrationTestsEnabled {
			t.SkipNow()
			return nil
		}
		if !calledRun {
			t.Fatal("must call v23test.Run(m.Run) from TestMain")
			return nil
		}
	}

	// Note: On error, NewShell returns a *Shell with Opts.Errorf initialized to
	// simplify things for the caller.
	sh := &Shell{
		Shell: gosh.NewShell(gosh.Opts{
			Errorf:              opts.Errorf,
			Logf:                opts.Logf,
			SuppressChildOutput: opts.SuppressChildOutput,
			ChildOutputDir:      opts.ChildOutputDir,
			BinDir:              opts.BinDir,
		}),
	}

	// Create context.
	// TODO(sadovsky): Similar to gosh.Shell.newShell, we should call sh.cleanup()
	// without setting sh.calledCleanup to true if any initialization step fails.
	ctx, shutdown := v23.Init()
	if sh.AddToCleanup(shutdown); sh.Err != nil {
		return sh
	}
	if t != nil {
		ctx = v23.WithListenSpec(ctx, rpc.ListenSpec{Addrs: rpc.ListenAddrs{{Protocol: "tcp", Address: "127.0.0.1:0"}}})
		sh.Vars[envShellTestProcess] = "1"
	}

	// Create principal and update context.
	dir := sh.MakeTempDir()
	if sh.Err != nil {
		return sh
	}
	creds, err := newRootCredentials(newFilesystemPrincipalManager(dir))
	if err != nil {
		sh.HandleError(err)
		return sh
	}
	ctx, err = v23.WithPrincipal(ctx, creds.Principal)
	if err != nil {
		sh.HandleError(err)
		return sh
	}

	sh.Ctx = ctx
	sh.Credentials = creds
	return sh
}

// ForkCredentials creates a new Credentials (with a fresh principal) and
// blesses it with the given extension and no caveats, using this principal's
// default blessings. Additionally, it calls SetDefaultBlessings.
func (sh *Shell) ForkCredentials(extension string) *Credentials {
	sh.Ok()
	res, err := sh.Credentials.Fork(extension)
	sh.HandleError(err)
	return res
}

// ForkContext creates a new Context with forked credentials.
func (sh *Shell) ForkContext(extension string) *context.T {
	sh.Ok()
	c := sh.ForkCredentials(extension)
	if sh.Err != nil {
		return nil
	}
	ctx, err := v23.WithPrincipal(sh.Ctx, c.Principal)
	sh.HandleError(err)
	return ctx
}

// JiriBuildGoPkg compiles a Go package using the "jiri go build" command and
// writes the resulting binary to sh.Opts.BinDir, similar to BuildGoPkg. Returns
// the absolute path to the binary.
func (sh *Shell) JiriBuildGoPkg(pkg string, flags ...string) string {
	sh.Ok()
	binPath := filepath.Join(sh.Opts.BinDir, path.Base(pkg))
	// If this binary has already been built, don't rebuild it.
	if _, err := os.Stat(binPath); err == nil {
		return binPath
	} else if !os.IsNotExist(err) {
		sh.HandleError(err)
		return ""
	}
	// Build binary to tempBinPath, then move it to binPath.
	tempDir, err := ioutil.TempDir(sh.Opts.BinDir, "")
	if err != nil {
		sh.HandleError(err)
		return ""
	}
	defer os.RemoveAll(tempDir)
	tempBinPath := filepath.Join(tempDir, path.Base(pkg))
	args := []string{"go", "build", "-x", "-o", tempBinPath}
	args = append(args, flags...)
	args = append(args, pkg)
	c := sh.Cmd("jiri", args...)
	if sh.Err != nil {
		return ""
	}
	c.SuppressOutput = true
	if c.Run(); sh.Err != nil {
		return ""
	}
	if sh.Rename(tempBinPath, binPath); sh.Err != nil {
		return ""
	}
	return binPath
}

// Run does some initialization work, then returns run(). Exported so that
// TestMain functions can simply call os.Exit(v23test.Run(m.Run)).
func Run(run func() int) int {
	gosh.MaybeRunFnAndExit()
	calledRun = true
	// Set up shared bin dir if V23_BIN_DIR is not already set.
	if dir := os.Getenv(envBinDir); len(dir) == 0 {
		if dir, err := ioutil.TempDir("", "bin-"); err != nil {
			panic(err)
		} else {
			os.Setenv(envBinDir, dir)
			defer os.Unsetenv(envBinDir)
			defer os.RemoveAll(dir)
		}
	}
	return run()
}

////////////////////////////////////////////////////////////////////////////////
// Methods for starting subprocesses

func newCmd(sh *Shell, c *gosh.Cmd) *Cmd {
	return (&Cmd{Cmd: c}).WithCredentials(sh.ForkCredentials("child"))
}

// Cmd returns a Cmd for an invocation of the named program.
func (sh *Shell) Cmd(name string, args ...string) *Cmd {
	c := sh.Shell.Cmd(name, args...)
	if sh.Err != nil {
		return nil
	}
	return newCmd(sh, c)
}

// Fn returns a Cmd for an invocation of the given registered Fn.
func (sh *Shell) Fn(fn *gosh.Fn, args ...interface{}) *Cmd {
	c := sh.Shell.Fn(fn, args...)
	if sh.Err != nil {
		return nil
	}
	return newCmd(sh, c)
}

// Main returns a Cmd for an invocation of the given registered main() function.
// Intended usage: Have your program's main() call RealMain, then write a parent
// program that uses Shell.Main to run RealMain in a child process. With this
// approach, RealMain can be compiled into the parent program's binary. Caveat:
// potential flag collisions.
func (sh *Shell) Main(fn *gosh.Fn, args ...string) *Cmd {
	c := sh.Shell.Main(fn, args...)
	if sh.Err != nil {
		return nil
	}
	return newCmd(sh, c)
}
