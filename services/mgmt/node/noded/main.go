package main

import (
	"flag"
	"os"

	"veyron.io/veyron/veyron2/naming"
	"veyron.io/veyron/veyron2/rt"
	"veyron.io/veyron/veyron2/vlog"

	"veyron.io/veyron/veyron/lib/signals"
	"veyron.io/veyron/veyron/profiles/roaming"
	"veyron.io/veyron/veyron/services/mgmt/node/config"
	"veyron.io/veyron/veyron/services/mgmt/node/impl"
)

var (
	publishAs   = flag.String("name", "", "name to publish the node manager at")
	installSelf = flag.Bool("install_self", false, "perform installation using environment and command-line flags")
	installFrom = flag.String("install_from", "", "if not-empty, perform installation from the provided application envelope object name")
)

func main() {
	flag.Parse()
	runtime, err := rt.New()
	if err != nil {
		vlog.Fatalf("Could not initialize runtime: %v", err)
	}
	defer runtime.Cleanup()

	if len(*installFrom) > 0 {
		if err := impl.InstallFrom(*installFrom); err != nil {
			vlog.Errorf("InstallFrom failed: %v", err)
			os.Exit(1)
		}
		return
	}

	if *installSelf {
		// TODO(caprita): Make the flags survive updates.
		if err := impl.SelfInstall(flag.Args(), os.Environ()); err != nil {
			vlog.Errorf("SelfInstall failed: %v", err)
			os.Exit(1)
		}
		return
	}

	server, err := runtime.NewServer()
	if err != nil {
		vlog.Fatalf("NewServer() failed: %v", err)
	}
	defer server.Stop()
	endpoint, err := server.Listen(roaming.ListenSpec)
	if err != nil {
		vlog.Fatalf("Listen(%s) failed: %v", roaming.ListenSpec, err)
	}
	name := naming.JoinAddressName(endpoint.String(), "")
	vlog.VI(0).Infof("Node manager object name: %v", name)
	configState, err := config.Load()
	if err != nil {
		vlog.Fatalf("Failed to load config passed from parent: %v", err)
		return
	}
	configState.Name = name
	// TODO(caprita): We need a way to set config fields outside of the
	// update mechanism (since that should ideally be an opaque
	// implementation detail).
	dispatcher, err := impl.NewDispatcher(runtime.Principal(), configState)
	if err != nil {
		vlog.Fatalf("Failed to create dispatcher: %v", err)
	}
	if err := server.ServeDispatcher(*publishAs, dispatcher); err != nil {
		vlog.Fatalf("Serve(%v) failed: %v", *publishAs, err)
	}
	vlog.VI(0).Infof("Node manager published as: %v", *publishAs)
	impl.InvokeCallback(runtime.NewContext(), name)

	// Wait until shutdown.
	<-signals.ShutdownOnSignals(runtime)
}
