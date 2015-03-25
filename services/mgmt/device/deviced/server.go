// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

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

	"v.io/x/lib/cmdline"

	vexec "v.io/x/ref/lib/exec"
	"v.io/x/ref/lib/signals"
	_ "v.io/x/ref/profiles/roaming"
	"v.io/x/ref/services/mgmt/device/config"
	"v.io/x/ref/services/mgmt/device/starter"

	"v.io/v23"
	"v.io/v23/mgmt"
	"v.io/v23/rpc"
	"v.io/x/lib/vlog"
)

var (
	// TODO(caprita): publishAs and restartExitCode should be provided by the
	// config?
	publishAs       = flag.String("name", "", "name to publish the device manager at")
	restartExitCode = flag.Int("restart_exit_code", 0, "exit code to return when device manager should be restarted")
	nhName          = flag.String("neighborhood_name", "", `if provided, it will enable sharing with the local neighborhood with the provided name. The address of the local mounttable will be published to the neighboorhood and everything in the neighborhood will be visible on the local mounttable.`)
	dmPort          = flag.Int("deviced_port", 0, "the port number of assign to the device manager service. The hostname/IP address part of --veyron.tcp.address is used along with this port. By default, the port is assigned by the OS.")
	proxyPort       = flag.Int("proxy_port", 0, "the port number to assign to the proxy service. 0 means no proxy service.")
	usePairingToken = flag.Bool("use_pairing_token", false, "generate a pairing token for the device manager that will need to be provided when a device is claimed")
)

func runServer(*cmdline.Command, []string) error {
	ctx, shutdown := v23.Init()
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
		AccessListFile: filepath.Join(mtAclDir, "acls"),
	}
	if testMode {
		ns.ListenSpec = rpc.ListenSpec{Addrs: rpc.ListenAddrs{{"tcp", "127.0.0.1:0"}}}
	} else {
		ns.ListenSpec = v23.GetListenSpec(ctx)
		ns.Name = *publishAs
		ns.Neighborhood = *nhName
	}
	// TODO(caprita): Move pairing token generation and printing into the
	// claimable service setup.
	var pairingToken string
	if *usePairingToken {
		var token [8]byte
		if _, err := rand.Read(token[:]); err != nil {
			vlog.Errorf("unable to generate pairing token: %v", err)
			return err
		}
		pairingToken = base64.URLEncoding.EncodeToString(token[:])
		vlog.VI(0).Infof("Device manager pairing token: %v", pairingToken)
		vlog.FlushLog()
	}
	dev := starter.DeviceArgs{
		ConfigState:     configState,
		TestMode:        testMode,
		RestartCallback: func() { exitErr = cmdline.ErrExitCode(*restartExitCode) },
		PairingToken:    pairingToken,
	}
	if testMode {
		dev.ListenSpec = rpc.ListenSpec{Addrs: rpc.ListenAddrs{{"tcp", "127.0.0.1:0"}}}
	} else {
		if dev.ListenSpec, err = derivedListenSpec(ns.ListenSpec, *dmPort); err != nil {
			return err
		}
	}
	var proxy starter.ProxyArgs
	if !testMode {
		proxy.Port = *proxyPort
	}
	// We grab the shutdown channel at this point in order to ensure that we
	// register a listener for the app cycle manager Stop before we start
	// running the device manager service.  Otherwise, any device manager
	// method that calls Stop on the app cycle manager (e.g. the Stop RPC)
	// will precipitate an immediate process exit.
	shutdownChan := signals.ShutdownOnSignals(ctx)
	stop, err := starter.Start(ctx, starter.Args{Namespace: ns, Device: dev, MountGlobalNamespaceInLocalNamespace: true, Proxy: proxy})
	if err != nil {
		return err
	}
	defer stop()

	// Wait until shutdown.  Ignore duplicate signals (sent by agent and
	// received as part of process group).
	signals.SameSignalTimeWindow = 500 * time.Millisecond
	vlog.Info("Shutting down due to: ", <-shutdownChan)
	return exitErr
}

// derivedListenSpec returns a copy of ls, with the ports changed to port.
func derivedListenSpec(ls rpc.ListenSpec, port int) (rpc.ListenSpec, error) {
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
