// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package lockfile_test contains an integration test for the lockfile package.
//
// Unfortunately, has to be in its own package to avoid an import cycle with
// the test/modules framework, which includes an agent implementation.
//
// TODO(sadovsky): The above statement is no longer true. Maybe move this test
// elsewhere?
package lockfile_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"v.io/x/lib/gosh"
	"v.io/x/ref/lib/v23test"
	_ "v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/services/agent/internal/lockfile"
)

var createLockfile = gosh.Register("createLockfile", func(file string) {
	err := lockfile.CreateLockfile(file)
	if err == nil {
		fmt.Println("Grabbed lock")
	} else {
		fmt.Println("Lock failed")
	}
})

func TestLockFile(t *testing.T) {
	dir, err := ioutil.TempDir("", "lf")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	file := filepath.Join(dir, "myfile")
	if err = lockfile.CreateLockfile(file); err != nil {
		t.Fatal(err)
	}
	lockpath := file + "-lock"
	bytes, err := ioutil.ReadFile(lockpath)
	if err != nil {
		t.Fatal(err)
	}
	err, running := lockfile.StillRunning(bytes)
	if err != nil {
		t.Fatal(err)
	}
	if !running {
		t.Fatal("expected StillRunning() = true")
	}

	if err = lockfile.CreateLockfile(file); err == nil {
		t.Fatal("Creating 2nd lockfile should fail")
	}

	lockfile.RemoveLockfile(file)
	if _, err = os.Lstat(lockpath); !os.IsNotExist(err) {
		t.Fatalf("%s: expected NotExist, got %v", lockpath, err)
	}
}

func TestOtherProcess(t *testing.T) {
	dir, err := ioutil.TempDir("", "lf")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	file := filepath.Join(dir, "myfile")

	sh := v23test.NewShell(t, v23test.Opts{})
	defer sh.Cleanup()

	// Start a new child which creates a lockfile and exits.
	output := sh.Fn(createLockfile, file).CombinedOutput()
	if output != "Grabbed lock\n" {
		t.Fatal("Unexpected output: %s", output)
	}

	// Verify it created a lockfile.
	lockpath := file + "-lock"
	bytes, err := ioutil.ReadFile(lockpath)
	if err != nil {
		t.Fatal(err)
	}
	// And that we know the lockfile is invalid.
	err, running := lockfile.StillRunning(bytes)
	if err != nil {
		t.Fatal(err)
	}
	if running {
		t.Fatal("child process is dead")
	}

	// Now create a lockfile for the process.
	if err = lockfile.CreateLockfile(file); err != nil {
		t.Fatal(err)
	}

	// Now the child should fail to create one.
	output = sh.Fn(createLockfile, file).CombinedOutput()
	if output != "Lock failed\n" {
		t.Fatal("Unexpected output: %s", output)
	}
}

func TestMain(m *testing.M) {
	os.Exit(v23test.Run(m.Run))
}
