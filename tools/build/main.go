// The following enables go generate to generate the doc.go file.
//go:generate go run $VANADIUM_ROOT/release/go/src/v.io/lib/cmdline/testdata/gendoc.go .

package main

import (
	"os"

	"v.io/core/veyron2"
	"v.io/core/veyron2/rt"

	_ "v.io/core/veyron/profiles"
)

var runtime veyron2.Runtime

func main() {
	var err error
	runtime, err = rt.New()
	if err != nil {
		panic(err)
	}

	exitCode := root().Main()
	runtime.Cleanup()

	os.Exit(exitCode)
}
