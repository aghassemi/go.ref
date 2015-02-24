// A simple command-line tool to run the benchmark server.
package main

import (
	"v.io/core/veyron/lib/signals"
	_ "v.io/core/veyron/profiles/roaming"
	"v.io/core/veyron/runtimes/google/ipc/benchmark"

	"v.io/v23"
	"v.io/v23/vlog"
)

func main() {
	ctx, shutdown := v23.Init()
	defer shutdown()

	addr, stop := benchmark.StartServer(ctx, v23.GetListenSpec(ctx))
	vlog.Infof("Listening on %s", addr)
	defer stop()
	<-signals.ShutdownOnSignals(ctx)
}
