package main

import (
	"crypto/rand"
	"encoding/base64"
	"flag"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"v.io/lib/cmdline"

	vexec "v.io/core/veyron/lib/exec"
	"v.io/core/veyron/lib/signals"
	_ "v.io/core/veyron/profiles/roaming"
	"v.io/core/veyron/services/mgmt/device/config"
	"v.io/core/veyron/services/mgmt/device/starter"

	"v.io/core/veyron2"
	"v.io/core/veyron2/ipc"
	"v.io/core/veyron2/mgmt"
	"v.io/core/veyron2/vlog"
)

var (
	// TODO(caprita): publishAs and restartExitCode should be provided by the
	// config?
	publishAs       = flag.String("name", "", "name to publish the device manager at")
	restartExitCode = flag.Int("restart_exit_code", 0, "exit code to return when device manager should be restarted")
	nhName          = flag.String("neighborhood_name", "", `if provided, it will enable sharing with the local neighborhood with the provided name. The address of the local mounttable will be published to the neighboorhood and everything in the neighborhood will be visible on the local mounttable.`)
	dmPort          = flag.Int("deviced_port", 0, "the port number of assign to the device manager service. The hostname/IP address part of --veyron.tcp.address is used along with this port. By default, the port is assigned by the OS.")
	usePairingToken = flag.Bool("use_pairing_token", false, "generate a pairing token for the device manager that will need to be provided when a device is claimed")
)

func runServer(*cmdline.Command, []string) error {
	ctx, shutdown := veyron2.Init()
	defer shutdown()

	var testMode bool
	// If this device manager was started by another device manager, it must
	// be part of a self update to test that this binary works. In that
	// case, we need to disable a lot of functionality.
	if handle, err := vexec.GetChildHandle(); err == nil {
		if _, err := handle.Config.Get(mgmt.ParentNameConfigKey); err == nil {
			testMode = true
			vlog.Infof("TEST MODE")
		}
	}
	configState, err := config.Load()
	if err != nil {
		vlog.Errorf("Failed to load config passed from parent: %v", err)
		return err
	}
	mtAclDir := filepath.Join(configState.Root, "mounttable")
	if err := os.MkdirAll(mtAclDir, 0700); err != nil {
		vlog.Errorf("os.MkdirAll(%q) failed: %v", mtAclDir, err)
		return err
	}

	// TODO(ashankar,caprita): Use channels/locks to synchronize the
	// setting and getting of exitErr.
	var exitErr error
	ns := starter.NamespaceArgs{
		ACLFile:      filepath.Join(mtAclDir, "acls"),
		Neighborhood: *nhName,
	}
	if testMode {
		ns.ListenSpec = ipc.ListenSpec{Addrs: ipc.ListenAddrs{{"tcp", "127.0.0.1:0"}}}
	} else {
		ns.ListenSpec = veyron2.GetListenSpec(ctx)
		ns.Name = *publishAs
	}
	var pairingToken string
	if *usePairingToken {
		var token [8]byte
		if _, err := rand.Read(token[:]); err != nil {
			vlog.Errorf("unable to generate pairing token: %v", err)
			return err
		}
		pairingToken = base64.URLEncoding.EncodeToString(token[:])
		vlog.VI(0).Infof("Device manager pairing token: %v", pairingToken)
	}
	dev := starter.DeviceArgs{
		ConfigState:     configState,
		TestMode:        testMode,
		RestartCallback: func() { exitErr = cmdline.ErrExitCode(*restartExitCode) },
		PairingToken:    pairingToken,
	}
	if dev.ListenSpec, err = newDeviceListenSpec(ns.ListenSpec, *dmPort); err != nil {
		return err
	}
	// We grab the shutdown channel at this point in order to ensure that we
	// register a listener for the app cycle manager Stop before we start
	// running the device manager service.  Otherwise, any device manager
	// method that calls Stop on the app cycle manager (e.g. the Stop RPC)
	// will precipitate an immediate process exit.
	shutdownChan := signals.ShutdownOnSignals(ctx)
	stop, err := starter.Start(ctx, starter.Args{Namespace: ns, Device: dev, MountGlobalNamespaceInLocalNamespace: true})
	if err != nil {
		return err
	}
	defer stop()

	// Wait until shutdown.  Ignore duplicate signals (sent by agent and
	// received as part of process group).
	signals.SameSignalTimeWindow = 500 * time.Millisecond
	<-shutdownChan
	return exitErr
}

// newDeviceListenSpec returns a copy of ls, with the ports changed to port.
func newDeviceListenSpec(ls ipc.ListenSpec, port int) (ipc.ListenSpec, error) {
	orig := ls.Addrs
	ls.Addrs = nil
	for _, a := range orig {
		host, _, err := net.SplitHostPort(a.Address)
		if err != nil {
			err = fmt.Errorf("net.SplitHostPort(%v) failed: %v", a.Address, err)
			vlog.Errorf(err.Error())
			return ls, err
		}
		a.Address = net.JoinHostPort(host, strconv.Itoa(port))
		ls.Addrs = append(ls.Addrs, a)
	}
	return ls, nil
}
