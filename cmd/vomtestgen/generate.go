// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The following enables go generate to generate the doc.go file.
//go:generate go run $V23_ROOT/release/go/src/v.io/x/lib/cmdline/testdata/gendoc.go . -help

package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"v.io/v23/vdl"
	"v.io/v23/vom"
	"v.io/x/lib/cmdline2"
	"v.io/x/ref/lib/vdl/build"
	"v.io/x/ref/lib/vdl/codegen"
	"v.io/x/ref/lib/vdl/codegen/vdlgen"
	"v.io/x/ref/lib/vdl/compile"
	_ "v.io/x/ref/profiles/static"
)

const (
	testpkg          = "v.io/v23/vom/testdata"
	vomdataCanonical = testpkg + "/" + vomdataConfig
	vomdataConfig    = "vomdata.vdl.config"
)

var cmdGenerate = &cmdline2.Command{
	Runner: cmdline2.RunnerFunc(runGenerate),
	Name:   "vomtestgen",
	Short:  "generates test data for the vom wire protocol implementation",
	Long: `
Command vomtestgen generates test data for the vom wire protocol implementation.
It takes as input a vdl config file, and outputs a vdl package with test cases.
Test data is generated using this tool to make it easy to add many test cases,
and verify them in implementations in different languages
`,
	ArgsName: "[vomdata]",
	ArgsLong: `
[vomdata] is the path to the vomdata input file, specified in the vdl config
file format.  It must be of the form "NAME.vdl.config", and the output vdl
file will be generated at "NAME.vdl".

The config file should export a const []any that contains all of the values
that will be tested.  Here's an example:
   config = []any{
     bool(true), uint64(123), string("abc"),
   }

If not specified, we'll try to find the file at its canonical location:
   ` + vomdataCanonical,
}

var (
	optGenMaxErrors int
	optGenExts      string
)

func init() {
	cmdGenerate.Flags.IntVar(&optGenMaxErrors, "max-errors", -1, "Stop processing after this many errors, or -1 for unlimited.")
	cmdGenerate.Flags.StringVar(&optGenExts, "exts", ".vdl", "Comma-separated list of valid VDL file name extensions.")
}

func main() {
	cmdline2.Main(cmdGenerate)
}

func runGenerate(env *cmdline2.Env, args []string) error {
	debug := new(bytes.Buffer)
	defer dumpDebug(env.Stderr, debug)
	compileEnv := compile.NewEnv(optGenMaxErrors)
	// Get the input datafile path.
	var path string
	switch len(args) {
	case 0:
		srcDirs := build.SrcDirs(compileEnv.Errors)
		if err := compileEnv.Errors.ToError(); err != nil {
			return env.UsageErrorf("%v", err)
		}
		path = guessDataFilePath(debug, srcDirs)
		if path == "" {
			return env.UsageErrorf("couldn't find vomdata file in src dirs: %v", srcDirs)
		}
	case 1:
		path = args[0]
	default:
		return env.UsageErrorf("too many args (expecting exactly 1 vomdata file)")
	}
	inName := filepath.Clean(path)
	if !strings.HasSuffix(inName, ".vdl.config") {
		return env.UsageErrorf(`vomdata file doesn't end in ".vdl.config": %s`, inName)
	}
	outName := inName[:len(inName)-len(".config")]
	// Remove the generated file, so that it doesn't interfere with compiling the
	// config.  Ignore errors since it might not exist yet.
	if err := os.Remove(outName); err == nil {
		fmt.Fprintf(debug, "Removed output file %v\n", outName)
	}
	config, err := compileConfig(debug, inName, compileEnv)
	if err != nil {
		return err
	}
	data, err := generate(config)
	if err != nil {
		return err
	}
	if err := writeFile(data, outName); err != nil {
		return err
	}
	debug.Reset() // Don't dump debugging information on success
	fmt.Fprintf(env.Stdout, "Wrote output file %v\n", outName)
	return nil
}

func dumpDebug(stderr io.Writer, debug *bytes.Buffer) {
	if d := debug.Bytes(); len(d) > 0 {
		io.Copy(stderr, debug)
	}
}

func guessDataFilePath(debug io.Writer, srcDirs []string) string {
	// Try to guess the data file path by looking for the canonical vomdata input
	// file in each of our source directories.
	for _, dir := range srcDirs {
		guess := filepath.Join(dir, vomdataCanonical)
		if stat, err := os.Stat(guess); err == nil && !stat.IsDir() {
			fmt.Fprintf(debug, "Found vomdata file %s\n", guess)
			return guess
		}
	}
	return ""
}

func compileConfig(debug io.Writer, inName string, env *compile.Env) (*vdl.Value, error) {
	basename := filepath.Base(inName)
	data, err := os.Open(inName)
	if err != nil {
		return nil, fmt.Errorf("couldn't open vomdata file %s: %v", inName, err)
	}
	defer func() { data.Close() }() // make defer use the latest value of data
	if stat, err := data.Stat(); err != nil || stat.IsDir() {
		return nil, fmt.Errorf("vomdata file %s is a directory", inName)
	}
	var opts build.Opts
	opts.Extensions = strings.Split(optGenExts, ",")
	// Compile package dependencies in transitive order.
	deps := build.TransitivePackagesForConfig(basename, data, opts, env.Errors)
	for _, dep := range deps {
		if pkg := build.BuildPackage(dep, env); pkg != nil {
			fmt.Fprintf(debug, "Built package %s\n", pkg.Path)
		}
	}
	// Try to seek back to the beginning of the data file, or if that fails try to
	// open the data file again.
	if off, err := data.Seek(0, 0); off != 0 || err != nil {
		if err := data.Close(); err != nil {
			return nil, fmt.Errorf("couldn't close vomdata file %s: %v", inName, err)
		}
		data, err = os.Open(inName)
		if err != nil {
			return nil, fmt.Errorf("couldn't re-open vomdata file %s: %v", inName, err)
		}
	}
	// Compile config into our test values.
	config := build.BuildConfig(basename, data, nil, nil, env)
	if err := env.Errors.ToError(); err != nil {
		return nil, err
	}
	fmt.Fprintf(debug, "Compiled vomdata file %s\n", inName)
	return config, err
}

func generate(config *vdl.Value) ([]byte, error) {
	// This config needs to have a specific struct format. See @testdata/vomtype.vdl.
	// TODO(alexfandrianto): Instead of this, we should have separate generator
	// functions that switch off of the vomdata config filename. That way, we can
	// have 1 config each for encode/decode, compatibility, and convertibility.
	buf := new(bytes.Buffer)
	fmt.Fprintf(buf, `// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file was auto-generated via "vomtest generate".
// DO NOT UPDATE MANUALLY; read the comments in `+vomdataConfig+`.

package testdata
`)
	imports := codegen.ImportsForValue(config, testpkg)
	if len(imports) > 0 {
		fmt.Fprintf(buf, "\n%s\n", vdlgen.Imports(imports))
	}
	fmt.Fprintf(buf, `
// TestCase represents an individual testcase for vom encoding and decoding.
type TestCase struct {
	Name       string // Name of the testcase
	Value      any    // Value to test
	TypeString string // The string representation of the Type
	Hex        string // Hex pattern representing vom encoding
	HexVersion string // Hex pattern representing vom encoding of Version
	HexType    string // Hex pattern representing vom encoding of Type
	HexValue   string // Hex pattern representing vom encoding of Value
}

// Tests contains the testcases to use to test vom encoding and decoding.
const Tests = []TestCase {`)
	// The vom encode-decode test cases need to be of type []any.
	encodeDecodeTests := config.StructField(0)
	if got, want := encodeDecodeTests.Type(), vdl.ListType(vdl.AnyType); got != want {
		return nil, fmt.Errorf("got encodeDecodeTests type %v, want %v", got, want)
	}

	for ix := 0; ix < encodeDecodeTests.Len(); ix++ {
		value := encodeDecodeTests.Index(ix)
		if !value.IsNil() {
			// The encodeDecodeTests has type []any, and there's no need for our values to
			// include the "any" type explicitly, so we descend into the elem value.
			value = value.Elem()
		}
		valstr := vdlgen.TypedConst(value, testpkg, imports)
		hexversion, hextype, hexvalue, vomdump, err := toVomHex(value)
		if err != nil {
			return nil, err
		}
		fmt.Fprintf(buf, `
%[7]s
	{
		%#[1]q,
		%[1]s,
		%[2]q,
		%[3]q,
		%[4]q, %[5]q, %[6]q,
	},`, valstr, value.Type().String(), hexversion+hextype+hexvalue, hexversion, hextype, hexvalue, vomdump)
	}
	fmt.Fprintf(buf, `
}
`)

	// The vom compatibility tests need to be of type map[string][]typeobject
	// Each of the []typeobject are a slice of inter-compatible typeobjects.
	// However, the typeobjects are not compatible with any other []typeobject.
	// Note: any and optional should be tested separately.
	compatTests := config.StructField(1)
	if got, want := compatTests.Type(), vdl.MapType(vdl.StringType, vdl.ListType(vdl.TypeObjectType)); got != want {
		return nil, fmt.Errorf("got compatTests type %v, want %v", got, want)
	}
	fmt.Fprintf(buf, `
// CompatTests contains the testcases to use to test vom type compatibility.
// CompatTests maps TestName (string) to CompatibleTypeSet ([]typeobject)
// Each CompatibleTypeSet contains types compatible with each other. However,
// types from different CompatibleTypeSets are incompatible.
const CompatTests = map[string][]typeobject{`)

	for _, testName := range vdl.SortValuesAsString(compatTests.Keys()) {
		compatibleTypeSet := compatTests.MapIndex(testName)
		valstr := vdlgen.TypedConst(compatibleTypeSet, testpkg, imports)
		fmt.Fprintf(buf, `
	%[1]q: %[2]s,`, testName.RawString(), valstr)
	}
	fmt.Fprintf(buf, `
}
`)

	// The vom conversion tests need to be a map[string][]ConvertGroup
	// See vom/testdata/vomtype.vdl
	convertTests := config.StructField(2)
	fmt.Fprintf(buf, `
// ConvertTests contains the testcases to check vom value convertibility.
// ConvertTests maps TestName (string) to ConvertGroups ([]ConvertGroup)
// Each ConvertGroup is a struct with 'Name', 'PrimaryType', and 'Values'.
// The values within a ConvertGroup can convert between themselves w/o error.
// However, values in higher-indexed ConvertGroups will error when converting up
// to the primary type of the lower-indexed ConvertGroups.
const ConvertTests = map[string][]ConvertGroup{`)
	for _, testName := range vdl.SortValuesAsString(convertTests.Keys()) {
		fmt.Fprintf(buf, `
	%[1]q: {`, testName.RawString())

		convertTest := convertTests.MapIndex(testName)
		for ix := 0; ix < convertTest.Len(); ix++ {
			convertGroup := convertTest.Index(ix)

			fmt.Fprintf(buf, `
		{
			%[1]q,
			%[2]s,
			{ `, convertGroup.StructField(0).RawString(), vdlgen.TypedConst(convertGroup.StructField(1), testpkg, imports))

			values := convertGroup.StructField(2)
			for iy := 0; iy < values.Len(); iy++ {
				value := values.Index(iy)
				if !value.IsNil() {
					// The value is any, and there's no need for our values to include
					// the "any" type explicitly, so we descend into the elem value.
					value = value.Elem()
				}
				valstr := vdlgen.TypedConst(value, testpkg, imports)
				fmt.Fprintf(buf, `%[1]s, `, valstr)
			}

			fmt.Fprintf(buf, `},
		},`)
		}

		fmt.Fprintf(buf, `
	},`)
	}
	fmt.Fprintf(buf, `
}
`)

	return buf.Bytes(), nil
}

func toVomHex(value *vdl.Value) (string, string, string, string, error) {
	var buf, typebuf bytes.Buffer
	encoder := vom.NewEncoderWithTypeEncoder(&buf, vom.NewTypeEncoder(&typebuf))
	if err := encoder.Encode(value); err != nil {
		return "", "", "", "", fmt.Errorf("vom.Encode(%v) failed: %v", value, err)
	}
	version, _ := buf.ReadByte() // Read the version byte.
	if typebuf.Len() > 0 {
		typebuf.ReadByte() // Remove the version byte.
	}
	vombytes := append(append([]byte{version}, typebuf.Bytes()...), buf.Bytes()...)
	const pre = "\t// "
	vomdump := pre + strings.Replace(vom.Dump(vombytes), "\n", "\n"+pre, -1)
	if strings.HasSuffix(vomdump, "\n"+pre) {
		vomdump = vomdump[:len(vomdump)-len("\n"+pre)]
	}
	// TODO(toddw): Add hex pattern bracketing for map and set.
	return fmt.Sprintf("%x", version), fmt.Sprintf("%x", typebuf.Bytes()), fmt.Sprintf("%x", buf.Bytes()), vomdump, nil
}

func writeFile(data []byte, outName string) error {
	// Create containing directory and write the file.
	dirName := filepath.Dir(outName)
	if err := os.MkdirAll(dirName, os.FileMode(0777)); err != nil {
		return fmt.Errorf("couldn't create directory %s: %v", dirName, err)
	}
	if err := ioutil.WriteFile(outName, data, os.FileMode(0666)); err != nil {
		return fmt.Errorf("couldn't write file %s: %v", outName, err)
	}
	return nil
}
