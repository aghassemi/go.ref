// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package test

import (
	"flag"
	"os"
	"runtime"
	"sync"

	"v.io/v23"
	"v.io/v23/context"

	"v.io/x/ref/internal/logger"
	"v.io/x/ref/lib/flags"
	"v.io/x/ref/test/testutil"
)

const (
	TestBlessing = "test-blessing"
)

var once sync.Once
var IntegrationTestsEnabled bool
var IntegrationTestsDebugShellOnError bool

const IntegrationTestsFlag = "v23.tests"
const IntegrationTestsDebugShellOnErrorFlag = "v23.tests.shell-on-fail"

func init() {
	flag.BoolVar(&IntegrationTestsEnabled, IntegrationTestsFlag, false, "Run integration tests.")
	flag.BoolVar(&IntegrationTestsDebugShellOnError, IntegrationTestsDebugShellOnErrorFlag, false, "Drop into a debug shell if an integration test fails.")
}

// Init sets up state for running tests: Adjusting GOMAXPROCS,
// configuring the vlog logging library, setting up the random number generator
// etc.
//
// Doing so requires flags to be parse, so this function explicitly parses
// flags. Thus, it is NOT a good idea to call this from the init() function
// of any module except "main" or _test.go files.
func Init() {
	init := func() {
		if os.Getenv("GOMAXPROCS") == "" {
			// Set the number of logical processors to the number of CPUs,
			// if GOMAXPROCS is not set in the environment.
			runtime.GOMAXPROCS(runtime.NumCPU())
		}
		flags.SetDefaultProtocol("tcp")
		flags.SetDefaultHostPort("127.0.0.1:0")
		flags.SetDefaultNamespaceRoot("/127.0.0.1:8101")
		// At this point all of the flags that we're going to use for
		// tests must be defined.
		// This will be the case if this is called from the init()
		// function of a _test.go file.
		flag.Parse()
		logger.Manager(logger.Global()).ConfigureFromFlags()
	}
	once.Do(init)
}

// V23Init initializes a new context.T and sets a freshly created principal
// (with a single self-signed blessing) on it. The principal setting step is
// skipped if this function is invoked from a process run using the modules
// package.
func V23Init() (*context.T, v23.Shutdown) {
	ctx, shutdown := v23.Init()
	if len(os.Getenv("V23_SHELL_HELPER_PROCESS_ENTRY_POINT")) != 0 {
		return ctx, shutdown
	}
	var err error
	if ctx, err = v23.WithPrincipal(ctx, testutil.NewPrincipal(TestBlessing)); err != nil {
		panic(err)
	}
	return ctx, shutdown
}
