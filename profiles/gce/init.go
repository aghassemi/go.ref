// +build linux

// Package gce provides a profile for Google Compute Engine and should be
// used by binaries that only ever expect to be run on GCE.
package gce

import (
	"flag"
	"fmt"
	"net"

	"v.io/core/veyron2"
	"v.io/core/veyron2/context"
	"v.io/core/veyron2/ipc"
	"v.io/core/veyron2/ipc/stream"
	"v.io/core/veyron2/vlog"

	"v.io/core/veyron/lib/appcycle"
	"v.io/core/veyron/lib/flags"
	"v.io/core/veyron/lib/netstate"
	"v.io/core/veyron/lib/websocket"
	"v.io/core/veyron/profiles/internal"
	"v.io/core/veyron/profiles/internal/gce"
	_ "v.io/core/veyron/runtimes/google/ipc/protocols/tcp"
	_ "v.io/core/veyron/runtimes/google/ipc/protocols/ws"
	_ "v.io/core/veyron/runtimes/google/ipc/protocols/wsh"
	grt "v.io/core/veyron/runtimes/google/rt"
)

var commonFlags *flags.Flags

func init() {
	veyron2.RegisterProfileInit(Init)
	stream.RegisterUnknownProtocol("wsh", websocket.HybridDial, websocket.HybridListener)
	commonFlags = flags.CreateAndRegister(flag.CommandLine, flags.Runtime, flags.Listen)
}

func Init(ctx *context.T) (veyron2.Runtime, *context.T, veyron2.Shutdown, error) {
	if !gce.RunningOnGCE() {
		return nil, nil, nil, fmt.Errorf("GCE profile used on a non-GCE system")
	}

	if err := internal.ParseFlags(commonFlags); err != nil {
		return nil, nil, nil, err
	}

	ac := appcycle.New()

	lf := commonFlags.ListenFlags()
	listenSpec := ipc.ListenSpec{
		Addrs: ipc.ListenAddrs(lf.Addrs),
		Proxy: lf.ListenProxy,
	}

	if ip, err := gce.ExternalIPAddress(); err != nil {
		return nil, nil, nil, err
	} else {
		listenSpec.AddressChooser = func(network string, addrs []ipc.Address) ([]ipc.Address, error) {
			return []ipc.Address{&netstate.AddrIfc{&net.IPAddr{IP: ip}, "gce-nat", nil}}, nil
		}
	}

	runtime, ctx, shutdown, err := grt.Init(ctx, ac, nil, &listenSpec, commonFlags.RuntimeFlags(), nil)
	if err != nil {
		return nil, nil, shutdown, err
	}

	vlog.Log.VI(1).Infof("Initializing GCE profile.")

	profileShutdown := func() {
		ac.Shutdown()
		shutdown()
	}

	return runtime, ctx, profileShutdown, nil
}
