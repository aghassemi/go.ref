// The following enables go generate to generate the doc.go file.
//go:generate go run $VEYRON_ROOT/veyron/go/src/veyron.io/lib/cmdline/testdata/gendoc.go . -help

package main

func main() {
	cmdGCLogs.Main()
}
