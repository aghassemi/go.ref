package core

import (
	"flag"
	"fmt"
	"strings"

	"v.io/core/veyron2/ipc"

	"v.io/core/veyron/lib/flags"
)

func parseFlags(fl *flags.Flags, args []string) error {
	if len(args) == 0 {
		return nil
	}
	return fl.Parse(args[1:], nil)
}

// parseListenFlags parses the given args using just the flags and env vars
// defined in the veyron/lib/flags package.
func parseListenFlags(args []string) (*flags.Flags, []string, error) {
	fs := flag.NewFlagSet("modules/core", flag.ContinueOnError)
	fl := flags.CreateAndRegister(fs, flags.Listen)
	err := parseFlags(fl, args)
	return fl, fl.Args(), err
}

func initListenSpec(fl *flags.Flags) ipc.ListenSpec {
	lf := fl.ListenFlags()
	return ipc.ListenSpec{
		Addrs: ipc.ListenAddrs(lf.Addrs),
		Proxy: lf.ListenProxy,
	}
}

// checkArgs checks for the expected number of args in args. A negative
// value means at least that number of args are expected.
func checkArgs(args []string, expected int, usage string) error {
	got := len(args)
	if expected < 0 {
		expected = -expected
		if got < expected {
			return fmt.Errorf("wrong # args (got %d, expected >=%d) expected: %q got: %v", got, expected, usage, args)
		}
	} else {
		if got != expected {
			return fmt.Errorf("wrong # args (got %d, expected %d) expected: %q got: %v", got, expected, usage, args)
		}
	}
	return nil
}

// usage generates a usage string based on the flags in a flagset.
func usage(fs *flag.FlagSet) string {
	res := []string{}
	fs.VisitAll(func(f *flag.Flag) {
		format := "  -%s=%s: %s"
		if getter, ok := f.Value.(flag.Getter); ok {
			if _, ok := getter.Get().(string); ok {
				// put quotes on the value
				format = "  -%s=%q: %s"
			}
		}
		res = append(res, fmt.Sprintf(format, f.Name, f.DefValue, f.Usage))
	})
	return strings.Join(res, "\n") + "\n"
}
