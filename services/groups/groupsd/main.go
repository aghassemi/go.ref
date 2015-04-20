// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Daemon groupsd implements the v.io/v23/services/groups interfaces for
// managing access control groups.
package main

// Example invocation:
// groupsd --v23.tcp.address="127.0.0.1:0" --name=groupsd

import (
	"flag"

	"v.io/v23"
	"v.io/v23/security/access"
	"v.io/x/lib/vlog"

	"v.io/x/ref/lib/signals"
	_ "v.io/x/ref/profiles/roaming"
	"v.io/x/ref/services/groups/internal/memstore"
	"v.io/x/ref/services/groups/internal/server"
)

// TODO(sadovsky): Perhaps this should be one of the standard Vanadium flags.
var (
	name = flag.String("name", "", "Name to mount at.")
)

func main() {
	ctx, shutdown := v23.Init()
	defer shutdown()

	s, err := v23.NewServer(ctx)
	if err != nil {
		vlog.Fatal("v23.NewServer() failed: ", err)
	}
	if _, err := s.Listen(v23.GetListenSpec(ctx)); err != nil {
		vlog.Fatal("s.Listen() failed: ", err)
	}

	// TODO(sadovsky): Switch to using NewAuthorizerOrDie.
	perms := access.Permissions{}
	m := server.NewManager(memstore.New(), perms)

	// Publish the service in the mount table.
	if err := s.ServeDispatcher(*name, m); err != nil {
		vlog.Fatal("s.ServeDispatcher() failed: ", err)
	}
	vlog.Info("Mounted at: ", *name)

	// Wait forever.
	<-signals.ShutdownOnSignals(ctx)
}
