package profiles

import (
	"flag"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/ipc"
	"v.io/x/lib/vlog"

	"v.io/core/veyron/lib/appcycle"
	"v.io/core/veyron/lib/flags"
	"v.io/core/veyron/lib/websocket"
	"v.io/core/veyron/profiles/internal"
	_ "v.io/core/veyron/runtimes/google/ipc/protocols/tcp"
	_ "v.io/core/veyron/runtimes/google/ipc/protocols/ws"
	_ "v.io/core/veyron/runtimes/google/ipc/protocols/wsh"
	grt "v.io/core/veyron/runtimes/google/rt"
)

var commonFlags *flags.Flags

func init() {
	v23.RegisterProfileInit(Init)
	ipc.RegisterUnknownProtocol("wsh", websocket.HybridDial, websocket.HybridListener)
	flags.SetDefaultProtocol("tcp")
	flags.SetDefaultHostPort("127.0.0.1:0")
	commonFlags = flags.CreateAndRegister(flag.CommandLine, flags.Runtime, flags.Listen)
}

func Init(ctx *context.T) (v23.Runtime, *context.T, v23.Shutdown, error) {
	if err := internal.ParseFlags(commonFlags); err != nil {
		return nil, nil, nil, err
	}

	ac := appcycle.New()

	lf := commonFlags.ListenFlags()
	listenSpec := ipc.ListenSpec{
		Addrs:          ipc.ListenAddrs(lf.Addrs),
		AddressChooser: internal.IPAddressChooser,
		Proxy:          lf.ListenProxy,
	}

	runtime, ctx, shutdown, err := grt.Init(ctx,
		ac,
		nil,
		&listenSpec,
		commonFlags.RuntimeFlags(),
		nil)
	if err != nil {
		return nil, nil, nil, err
	}
	vlog.Log.VI(1).Infof("Initializing generic profile.")

	profileShutdown := func() {
		ac.Shutdown()
		shutdown()
	}
	return runtime, ctx, profileShutdown, nil
}
