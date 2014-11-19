// The following enables go generate to generate the doc.go file.
//go:generate go run $VEYRON_ROOT/tools/go/src/tools/lib/cmdline/testdata/gendoc.go .

package main

import (
	"veyron.io/veyron/veyron2/rt"

	_ "veyron.io/veyron/veyron/profiles"
)

func main() {
	defer rt.Init().Cleanup()
	root().Main()
}
