// Package starter provides a single function that starts up servers for a
// mounttable and a device manager that is mounted on it.
package starter

import (
	"encoding/base64"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"v.io/core/veyron/lib/netstate"
	"v.io/core/veyron/runtimes/google/ipc/stream/proxy"
	"v.io/core/veyron/services/mgmt/device/config"
	"v.io/core/veyron/services/mgmt/device/impl"
	mounttable "v.io/core/veyron/services/mounttable/lib"

	"v.io/core/veyron2"
	"v.io/core/veyron2/context"
	"v.io/core/veyron2/ipc"
	"v.io/core/veyron2/naming"
	"v.io/core/veyron2/options"
	"v.io/core/veyron2/vlog"
)

type NamespaceArgs struct {
	Name       string         // Name to publish the mounttable service under.
	ListenSpec ipc.ListenSpec // ListenSpec for the server.
	ACLFile    string         // Path to the ACL file used by the mounttable.
	// Name in the local neighborhood on which to make the mounttable
	// visible. If empty, the mounttable will not be visible in the local
	// neighborhood.
	Neighborhood string
}

type DeviceArgs struct {
	Name            string         // Name to publish the device service under.
	ListenSpec      ipc.ListenSpec // ListenSpec for the device server.
	ConfigState     *config.State  // Configuration for the device.
	TestMode        bool           // Whether the device is running in test mode or not.
	RestartCallback func()         // Callback invoked when the device service is restarted.
	PairingToken    string         // PairingToken that a claimer needs to provide.
}

func (d *DeviceArgs) name(mt string) string {
	if d.Name != "" {
		return d.Name
	}
	return naming.Join(mt, "devmgr")
}

type ProxyArgs struct {
	Port int
}

type Args struct {
	Namespace NamespaceArgs
	Device    DeviceArgs
	Proxy     ProxyArgs

	// If true, the global namespace will be made available on the
	// mounttable server under "global/".
	MountGlobalNamespaceInLocalNamespace bool
}

// Start creates servers for the mounttable and device services and links them together.
//
// Returns the callback to be invoked to shutdown the services on success, or
// an error on failure.
func Start(ctx *context.T, args Args) (func(), error) {
	// TODO(caprita): use some mechanism (a file lock or presence of entry
	// in mounttable) to ensure only one device manager is running in an
	// installation?
	mi := &impl.ManagerInfo{
		Pid: os.Getpid(),
	}
	if err := impl.SaveManagerInfo(filepath.Join(args.Device.ConfigState.Root, "device-manager"), mi); err != nil {
		return nil, fmt.Errorf("failed to save info: %v", err)
	}

	// If the device has not yet been claimed, start the mounttable and
	// claimable service and wait for it to be claimed.
	// Once a device is claimed, close any previously running servers and
	// start a new mounttable and device service.
	if claimable, claimed := impl.NewClaimableDispatcher(ctx, args.Device.ConfigState, args.Device.PairingToken); claimable != nil {
		stopClaimable, err := startClaimableDevice(ctx, claimable, args)
		if err != nil {
			return nil, err
		}
		stop := make(chan struct{})
		stopped := make(chan struct{})
		go waitToBeClaimedAndStartClaimedDevice(ctx, stopClaimable, claimed, stop, stopped, args)
		return func() {
			close(stop)
			<-stopped
		}, nil
	}
	return startClaimedDevice(ctx, args)
}

func startClaimableDevice(ctx *context.T, dispatcher ipc.Dispatcher, args Args) (func(), error) {
	// TODO(caprita,ashankar): We create a context that we can cancel once
	// the device has been claimed. This gets around the following issue: if
	// we publish the claimable server to the local mounttable, and then
	// (following claim) we restart the mounttable server on the same port,
	// we fail to publish the device service to the (new) mounttable server
	// (Mount fails with "VC handshake failed: remote end closed VC(VCs not
	// accepted)".  Presumably, something to do with caching connections
	// (following the claim, the mounttable comes back on the same port as
	// before, and the client-side of the mount gets confused trying to
	// reuse the old connection and doesn't attempt to create a new
	// connection).
	// We should get to the bottom of it.
	ctx, cancel := context.WithCancel(ctx)
	mtName, stopMT, err := startMounttable(ctx, args.Namespace)
	if err != nil {
		cancel()
		return nil, err
	}
	server, err := veyron2.NewServer(ctx)
	if err != nil {
		stopMT()
		cancel()
		return nil, err
	}
	shutdown := func() {
		server.Stop()
		stopMT()
		cancel()
	}
	endpoints, err := server.Listen(args.Device.ListenSpec)
	if err != nil {
		shutdown()
		return nil, err
	}
	claimableServerName := args.Device.name(mtName)
	if err := server.ServeDispatcher(claimableServerName, dispatcher); err != nil {
		shutdown()
		return nil, err
	}
	publicKey, err := veyron2.GetPrincipal(ctx).PublicKey().MarshalBinary()
	if err != nil {
		shutdown()
		return nil, err
	}
	vlog.Infof("Unclaimed device manager (%v) published as %v with public_key:%s", endpoints[0].Name(), claimableServerName, base64.URLEncoding.EncodeToString(publicKey))
	return shutdown, nil
}

func waitToBeClaimedAndStartClaimedDevice(ctx *context.T, stopClaimable func(), claimed, stop <-chan struct{}, stopped chan<- struct{}, args Args) {
	// Wait for either the claimable service to complete, or be stopped
	defer close(stopped)
	select {
	case <-claimed:
		stopClaimable()
	case <-stop:
		stopClaimable()
		return
	}
	shutdown, err := startClaimedDevice(ctx, args)
	if err != nil {
		vlog.Errorf("Failed to start device service after it was claimed: %v", err)
		veyron2.GetAppCycle(ctx).Stop()
		return
	}
	defer shutdown()
	<-stop // Wait to be stopped
}

func startClaimedDevice(ctx *context.T, args Args) (func(), error) {
	mtName, stopMT, err := startMounttable(ctx, args.Namespace)
	if err != nil {
		vlog.Errorf("Failed to start mounttable service: %v", err)
		return nil, err
	}
	// TODO(caprita): We link in a proxy server into the device manager so
	// that we can bootstrap with install-local before we can install an
	// actual proxy app.  Once support is added to the IPC layer to allow
	// install-local to serve on the same connection it established to the
	// device manager (see TODO in
	// veyron/tools/mgmt/device/impl/local_install.go), we can get rid of
	// this local proxy altogether.
	stopProxy, err := startProxyServer(ctx, args.Proxy, mtName)
	if err != nil {
		vlog.Errorf("Failed to start proxy service: %v", err)
		stopMT()
		return nil, err
	}
	stopDevice, err := startDeviceServer(ctx, args.Device, mtName)
	if err != nil {
		vlog.Errorf("Failed to start device service: %v", err)
		stopProxy()
		stopMT()
		return nil, err
	}
	if args.MountGlobalNamespaceInLocalNamespace {
		mountGlobalNamespaceInLocalNamespace(ctx, mtName)
	}

	impl.InvokeCallback(ctx, args.Device.ConfigState.Name)

	return func() {
		stopDevice()
		stopProxy()
		stopMT()
	}, nil
}

func startProxyServer(ctx *context.T, p ProxyArgs, localMT string) (func(), error) {
	switch port := p.Port; {
	case port == 0:
		return func() {}, nil
	case port < 0:
		return nil, fmt.Errorf("invalid port: %v", port)
	}
	port := strconv.Itoa(p.Port)
	rid, err := naming.NewRoutingID()
	if err != nil {
		return nil, fmt.Errorf("Failed to get new routing id: %v", err)
	}
	protocol, addr := "tcp", net.JoinHostPort("", port)
	// Attempt to get a publicly accessible address for the proxy to publish
	// under.
	var publishAddr string
	ls := veyron2.GetListenSpec(ctx)
	if addrs, err := netstate.GetAccessibleIPs(); err == nil {
		if ac := ls.AddressChooser; ac != nil {
			if a, err := ac(protocol, addrs); err == nil && len(a) > 0 {
				addrs = a
			}
		}
		publishAddr = net.JoinHostPort(addrs[0].Address().String(), port)
	}
	proxy, err := proxy.New(rid, veyron2.GetPrincipal(ctx), protocol, addr, publishAddr)
	if err != nil {
		return nil, fmt.Errorf("Failed to create proxy: %v", err)
	}
	vlog.Infof("Local proxy (%v)", proxy.Endpoint().Name())
	return proxy.Shutdown, nil
}

func startMounttable(ctx *context.T, n NamespaceArgs) (string, func(), error) {
	mtName, stopMT, err := mounttable.StartServers(ctx, n.ListenSpec, n.Name, n.Neighborhood, n.ACLFile)
	if err != nil {
		vlog.Errorf("mounttable.StartServers(%#v) failed: %v", n, err)
	} else {
		vlog.Infof("Local mounttable (%v) published as %q", mtName, n.Name)
	}
	return mtName, stopMT, err
}

// startDeviceServer creates an ipc.Server and sets it up to server the Device service.
//
// ls: ListenSpec for the server
// configState: configuration for the Device service dispatcher
// mt: Object address of the mounttable
// dm: Name to publish the device service under
// testMode: whether the service is to be run in test mode
// restarted: callback invoked when the device manager is restarted.
//
// Returns:
// (1) Function to be called to force the service to shutdown
// (2) Any errors in starting the service (in which case, (1) will be nil)
func startDeviceServer(ctx *context.T, args DeviceArgs, mt string) (shutdown func(), err error) {
	server, err := veyron2.NewServer(ctx)
	if err != nil {
		return nil, err
	}
	shutdown = func() { server.Stop() }
	endpoints, err := server.Listen(args.ListenSpec)
	if err != nil {
		shutdown()
		return nil, err
	}
	args.ConfigState.Name = endpoints[0].Name()
	vlog.Infof("Device manager (%v) published as %v", args.ConfigState.Name, args.name(mt))

	dispatcher, err := impl.NewDispatcher(ctx, args.ConfigState, mt, args.TestMode, args.RestartCallback)
	if err != nil {
		shutdown()
		return nil, err
	}

	shutdown = func() {
		server.Stop()
		impl.Shutdown(dispatcher)
	}
	if err := server.ServeDispatcher(args.name(mt), dispatcher); err != nil {
		shutdown()
		return nil, err
	}
	return shutdown, nil
}

func mountGlobalNamespaceInLocalNamespace(ctx *context.T, localMT string) {
	ns := veyron2.GetNamespace(ctx)
	for _, root := range ns.Roots() {
		go func(r string) {
			var blessings []string
			for {
				var err error
				// TODO(rthellend,ashankar): This is temporary until the blessings of
				// our namespace roots are set along side their addresses.
				if blessings, err = findServerBlessings(ctx, r); err == nil {
					break
				}
				vlog.Infof("findServerBlessings(%q) failed: %v", r, err)
				time.Sleep(time.Second)
			}
			vlog.VI(2).Infof("Blessings for %q: %q", r, blessings)
			for {
				err := ns.Mount(ctx, naming.Join(localMT, "global"), r, 0 /* forever */, naming.ServesMountTableOpt(true), naming.MountedServerBlessingsOpt(blessings))
				if err == nil {
					break
				}
				vlog.Infof("Failed to Mount global namespace: %v", err)
				time.Sleep(time.Second)
			}
		}(root)
	}
}

func findServerBlessings(ctx *context.T, server string) ([]string, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	client := veyron2.GetClient(ctx)
	call, err := client.StartCall(ctx, server, ipc.ReservedSignature, nil, options.NoResolve{})
	if err != nil {
		return nil, err
	}
	remoteBlessings, _ := call.RemoteBlessings()
	return remoteBlessings, nil
}
