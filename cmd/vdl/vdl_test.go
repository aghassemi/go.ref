// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"v.io/x/lib/envvar"
	"v.io/x/ref/test/testutil"
	"v.io/x/ref/test/v23tests"
)

const (
	testDir    = "../../lib/vdl/testdata/base"
	outPkgPath = "v.io/x/ref/lib/vdl/testdata/base"
)

//go:generate jiri test generate

func verifyOutput(t *testing.T, outDir string) {
	entries, err := ioutil.ReadDir(testDir)
	if err != nil {
		t.Fatalf("ReadDir(%v) failed: %v", testDir, err)
	}
	numEqual := 0
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".vdl.go") {
			continue
		}
		testFile := filepath.Join(testDir, entry.Name())
		testBytes, err := ioutil.ReadFile(testFile)
		if err != nil {
			t.Fatalf("ReadFile(%v) failed: %v", testFile, err)
		}
		outFile := filepath.Join(outDir, outPkgPath, entry.Name())
		outBytes, err := ioutil.ReadFile(outFile)
		if err != nil {
			t.Fatalf("ReadFile(%v) failed: %v", outFile, err)
		}
		if !bytes.Equal(outBytes, testBytes) {
			t.Fatalf("GOT:\n%v\n\nWANT:\n%v\n", string(outBytes), string(testBytes))
		}
		numEqual++
	}
	if numEqual == 0 {
		t.Fatalf("testDir %s has no golden files *.vdl.go", testDir)
	}
}

// Compares generated VDL files against the copy in the repo.
func TestVDLGenerator(t *testing.T) {
	testEnv := v23tests.New(t)
	defer testEnv.Cleanup()
	vdlBin := testEnv.BuildGoPkg("v.io/x/ref/cmd/vdl")

	// Use vdl to generate Go code from input, into a temporary directory.
	outDir := testEnv.NewTempDir("")
	// TODO(toddw): test the generated java and javascript files too.
	outOpt := fmt.Sprintf("--go-out-dir=%s", outDir)
	vdlBin.Run("generate", "--lang=go", outOpt, testDir)
	// Check that each *.vdl.go file in the testDir matches the generated output.
	verifyOutput(t, outDir)
}

// Asserts that vdl generation works without VDLROOT or JIRI_ROOT being set.
func TestVDLGeneratorBuiltInVDLRoot(t *testing.T) {
	testEnv := v23tests.New(t)
	defer testEnv.Cleanup()
	vdlBin := testEnv.BuildGoPkg("v.io/x/ref/cmd/vdl")

	outDir := testEnv.NewTempDir("")
	outOpt := fmt.Sprintf("--go-out-dir=%s", outDir)
	env := envvar.SliceToMap(os.Environ())
	delete(env, "JIRI_ROOT")
	delete(env, "VDLROOT")
	vdlBin.WithEnv(envvar.MapToSlice(env)...).Run("-v", "generate", "--lang=go", outOpt, testDir)
	verifyOutput(t, outDir)
}

// Ensures the vdlroot data built-in to the binary matches the current sources.
func TestBuiltInVDLRootDataIsUpToDate(t *testing.T) {
	testEnv := v23tests.New(t)
	defer testEnv.Cleanup()
	dir := testEnv.NewTempDir("")

	if err := extractVDLRootData(dir); err != nil {
		t.Fatalf("Couldn't extract vdlroot: %v", err)
	}
	gotRoot := filepath.Join(dir, "v.io", "v23", "vdlroot")
	wantRoot := filepath.Join("..", "..", "..", "..", "v23", "vdlroot")
	var debug bytes.Buffer
	opts := testutil.FileTreeOpts{
		Debug: &debug,
		FileB: regexp.MustCompile(`((\.vdl)|(vdl\.config))$`),
	}
	switch ok, err := testutil.FileTreeEqual(gotRoot, wantRoot, opts); {
	case err != nil:
		t.Error(err)
	case !ok:
		t.Errorf("%v is not the same as %v\n%v", gotRoot, wantRoot, debug.String())
	}
}
