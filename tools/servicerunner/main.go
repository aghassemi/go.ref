// This binary starts several services (mount table, proxy, wspr), then prints a
// JSON map with their vars to stdout (as a single line), then waits forever.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"veyron.io/veyron/veyron2/rt"

	"veyron.io/veyron/veyron/lib/expect"
	"veyron.io/veyron/veyron/lib/flags/consts"
	"veyron.io/veyron/veyron/lib/modules"
	"veyron.io/veyron/veyron/lib/modules/core"
	_ "veyron.io/veyron/veyron/profiles"
)

func panicOnError(err error) {
	if err != nil {
		panic(err)
	}
}

// updateVars captures the vars from the given Handle's stdout and adds them to
// the given vars map, overwriting existing entries.
func updateVars(h modules.Handle, vars map[string]string, varNames ...string) error {
	varsToAdd := map[string]bool{}
	for _, v := range varNames {
		varsToAdd[v] = true
	}
	numLeft := len(varsToAdd)

	s := expect.NewSession(nil, h.Stdout(), 10*time.Second)
	for {
		l := s.ReadLine()
		if err := s.OriginalError(); err != nil {
			return err // EOF or otherwise
		}
		parts := strings.Split(l, "=")
		if len(parts) != 2 {
			return fmt.Errorf("Unexpected line: %s", l)
		}
		if _, ok := varsToAdd[parts[0]]; ok {
			numLeft--
			vars[parts[0]] = parts[1]
			if numLeft == 0 {
				break
			}
		}
	}
	return nil
}

func main() {
	rt.Init()

	// TODO(sadovsky): It would be better if Dispatch() itself performed the env
	// check.
	if os.Getenv(modules.ShellEntryPoint) != "" {
		panicOnError(modules.Dispatch())
		return
	}

	sh := modules.NewShell()
	defer sh.Cleanup(os.Stderr, os.Stderr)
	// TODO(sadovsky): Shell only does this for tests. It would be better if it
	// either always did it or never did it.
	if os.Getenv(consts.VeyronCredentials) == "" {
		panicOnError(sh.CreateAndUseNewCredentials())
	}
	// TODO(sadovsky): The following line will not be needed if the modules
	// library is restructured per my proposal.
	core.Install(sh)

	vars := map[string]string{}

	h, err := sh.Start("root", nil, "--", "--veyron.tcp.address=127.0.0.1:0")
	panicOnError(err)
	updateVars(h, vars, "MT_NAME")

	// Set consts.NamespaceRootPrefix env var, consumed downstream by proxyd
	// among others.
	// NOTE(sadovsky): If this is not set, proxyd takes several seconds to
	// start; if it is set, proxyd starts instantly. Fun!
	sh.SetVar(consts.NamespaceRootPrefix, vars["MT_NAME"])

	// NOTE(sadovsky): The proxyd binary requires --protocol and --address flags
	// while the proxyd command instead uses ListenSpec flags.
	h, err = sh.Start("proxyd", nil, "--", "--veyron.tcp.address=127.0.0.1:0", "p")
	panicOnError(err)
	updateVars(h, vars, "PROXY_ADDR")

	// TODO(sadovsky): Which identd should we be using?
	h, err = sh.Start("wsprd", nil, "--", "--veyron.proxy="+vars["PROXY_ADDR"], "--identd=/proxy.envyor.com:8101/identity/veyron-test/google")
	panicOnError(err)
	updateVars(h, vars, "WSPR_ADDR")

	bytes, err := json.Marshal(vars)
	panicOnError(err)
	fmt.Println(string(bytes))

	// Wait to be killed.
	select {}
}
