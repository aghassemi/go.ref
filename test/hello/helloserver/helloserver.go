// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Daemon helloserver is the simplest possible server.  It is mainly
// used in simple regression tests.
package main

import (
	"flag"
	"fmt"
	"os"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/x/ref/lib/signals"
	_ "v.io/x/ref/profiles"
)

var name *string = flag.String("name", "", "Name to publish under")

type helloServer struct{}

func (*helloServer) Hello(ctx *context.T, call rpc.ServerCall) (string, error) {
	return "hello", nil
}

func run() error {
	ctx, shutdown := v23.Init()
	defer shutdown()

	server, err := v23.NewServer(ctx)
	if err != nil {
		return fmt.Errorf("NewServer: %v", err)
	}
	eps, err := server.Listen(v23.GetListenSpec(ctx))
	if err != nil {
		return fmt.Errorf("Listen: %v", err)
	}
	if len(eps) > 0 {
		fmt.Printf("SERVER_NAME=%s\n", eps[0].Name())
	} else {
		fmt.Println("SERVER_NAME=proxy")
	}
	if err := server.Serve(*name, &helloServer{}, security.AllowEveryone()); err != nil {
		return fmt.Errorf("Serve: %v", err)
	}
	<-signals.ShutdownOnSignals(ctx)
	return nil
}

func main() {
	if err := run(); err != nil {
		os.Exit(1)
	}
}
