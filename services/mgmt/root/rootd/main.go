package main

import (
	"veyron.io/veyron/veyron2/rt"
	"veyron.io/veyron/veyron2/vlog"

	"veyron.io/veyron/veyron/lib/signals"
	"veyron.io/veyron/veyron/profiles/roaming"
	"veyron.io/veyron/veyron/services/mgmt/root/impl"
)

func main() {
	r := rt.Init()
	defer r.Cleanup()
	server, err := r.NewServer()
	if err != nil {
		vlog.Errorf("NewServer() failed: %v", err)
		return
	}
	defer server.Stop()
	dispatcher := impl.NewDispatcher()
	ep, err := server.ListenX(roaming.ListenSpec)
	if err != nil {
		vlog.Errorf("Listen(%s) failed: %v", roaming.ListenSpec, err)
		return
	}
	vlog.VI(0).Infof("Listening on %v", ep)
	name := ""
	if err := server.Serve(name, dispatcher); err != nil {
		vlog.Errorf("Serve(%v) failed: %v", name, err)
		return
	}

	// Wait until shutdown.
	<-signals.ShutdownOnSignals()
}
