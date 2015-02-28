// A simple command-line tool to run the benchmark server.
package main

import (
	"v.io/v23"
	"v.io/x/lib/vlog"

	"v.io/x/ref/lib/signals"
	"v.io/x/ref/profiles/internal/ipc/benchmark/internal"
	_ "v.io/x/ref/profiles/roaming"
)

func main() {
	ctx, shutdown := v23.Init()
	defer shutdown()

	addr, stop := internal.StartServer(ctx, v23.GetListenSpec(ctx))
	vlog.Infof("Listening on %s", addr)
	defer stop()
	<-signals.ShutdownOnSignals(ctx)
}
