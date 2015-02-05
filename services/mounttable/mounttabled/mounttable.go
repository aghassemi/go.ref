// mounttabled is a mount table daemon.
package main

import (
	"flag"
	"os"

	"v.io/core/veyron2"
	"v.io/core/veyron2/vlog"

	"v.io/core/veyron/lib/signals"
	_ "v.io/core/veyron/profiles/roaming"
	mounttable "v.io/core/veyron/services/mounttable/lib"
)

var (
	mountName = flag.String("name", "", `<name>, if provided, causes the mount table to mount itself under that name. The name may be absolute for a remote mount table service (e.g., "/<remote mt address>//some/suffix") or could be relative to this process' default mount table (e.g., "some/suffix").`)
	aclFile   = flag.String("acls", "", "ACL file. Default is to allow all access.")
	nhName    = flag.String("neighborhood_name", "", `<nh name>, if provided, will enable sharing with the local neighborhood with the provided name. The address of this mounttable will be published to the neighboorhood and everything in the neighborhood will be visible on this mounttable.`)
)

func main() {
	ctx, shutdown := veyron2.Init()
	defer shutdown()

	_, stop, err := mounttable.StartServers(ctx, veyron2.GetListenSpec(ctx), *mountName, *nhName, *aclFile)
	if err != nil {
		vlog.Errorf("mounttable.StartServers failed: %v", err)
		os.Exit(1)
	}
	defer stop()

	// Wait until signal is received.
	vlog.Info("Received signal ", <-signals.ShutdownOnSignals(ctx))
}
