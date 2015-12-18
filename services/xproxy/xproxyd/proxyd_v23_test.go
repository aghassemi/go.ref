// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main_test

import (
	"fmt"
	"os"
	"testing"
	"time"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/x/lib/gosh"
	"v.io/x/ref/lib/signals"
	"v.io/x/ref/lib/v23test"
	"v.io/x/ref/test/expect"
)

const (
	proxyName   = "proxy"    // Name which the proxy mounts itself at
	serverName  = "server"   // Name which the server mounts itself at
	responseVar = "RESPONSE" // Name of the variable used by client program to output the response
)

func TestV23Proxyd(t *testing.T) {
	sh := v23test.NewShell(t, v23test.Opts{Large: true})
	defer sh.Cleanup()
	sh.StartRootMountTable()

	var (
		proxydCreds = sh.ForkCredentials("proxyd")
		serverCreds = sh.ForkCredentials("server")
		clientCreds = sh.ForkCredentials("client")
		proxyd      = sh.JiriBuildGoPkg("v.io/x/ref/services/xproxy/xproxyd")
	)

	// Start proxyd.
	sh.Cmd(proxyd, "--v23.tcp.address=127.0.0.1:0", "--name="+proxyName, "--access-list", "{\"In\":[\"root:server\"]}").WithCredentials(proxydCreds).Start()

	// Start the server that only listens via the proxy.
	sh.Fn(runServer).WithCredentials(serverCreds).Start()

	// Run the client.
	cmd := sh.Fn(runClient).WithCredentials(clientCreds)
	session := expect.NewSession(t, cmd.StdoutPipe(), time.Minute)
	cmd.Run()
	if got, want := session.ExpectVar(responseVar), "server [root:server] saw client [root:client]"; got != want {
		t.Fatalf("Got %q, want %q", got, want)
	}
}

var runServer = gosh.Register("runServer", func() error {
	ctx, shutdown := v23.Init()
	defer shutdown()
	// Set the listen spec to listen only via the proxy.
	ctx = v23.WithListenSpec(ctx, rpc.ListenSpec{Proxy: proxyName})
	if _, _, err := v23.WithNewServer(ctx, serverName, service{}, security.AllowEveryone()); err != nil {
		return err
	}
	<-signals.ShutdownOnSignals(ctx)
	return nil
})

var runClient = gosh.Register("runClient", func() error {
	ctx, shutdown := v23.Init()
	defer shutdown()
	var response string
	if err := v23.GetClient(ctx).Call(ctx, serverName, "Echo", nil, []interface{}{&response}); err != nil {
		return err
	}
	fmt.Printf("%v=%v\n", responseVar, response)
	return nil
})

type service struct{}

func (service) Echo(ctx *context.T, call rpc.ServerCall) (string, error) {
	client, _ := security.RemoteBlessingNames(ctx, call.Security())
	server := security.LocalBlessingNames(ctx, call.Security())
	return fmt.Sprintf("server %v saw client %v", server, client), nil
}

func TestMain(m *testing.M) {
	os.Exit(v23test.Run(m.Run))
}
