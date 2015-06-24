// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// syncbased is a syncbase daemon.
package main

// Example invocation:
// syncbased --veyron.tcp.address="127.0.0.1:0" --name=syncbased

import (
	"flag"

	"v.io/v23"
	"v.io/v23/security"
	"v.io/v23/security/access"
	"v.io/x/lib/vlog"

	"v.io/syncbase/x/ref/services/syncbase/server"
	"v.io/x/ref/lib/security/securityflag"
	"v.io/x/ref/lib/signals"
	"v.io/x/ref/lib/xrpc"
	_ "v.io/x/ref/runtime/factories/generic"
)

var (
	name    = flag.String("name", "", "Name to mount at.")
	rootDir = flag.String("root-dir", "/var/lib/syncbase", "Root dir for storage engines and other data")
	engine  = flag.String("engine", "leveldb", "Storage engine to use. Currently supported: memstore and leveldb.")
)

// defaultPerms returns a permissions object that grants all permissions to the
// provided blessing patterns.
func defaultPerms(blessingPatterns []security.BlessingPattern) access.Permissions {
	perms := access.Permissions{}
	for _, tag := range access.AllTypicalTags() {
		for _, bp := range blessingPatterns {
			perms.Add(bp, string(tag))
		}
	}
	return perms
}

func main() {
	ctx, shutdown := v23.Init()
	defer shutdown()

	perms, err := securityflag.PermissionsFromFlag()
	if err != nil {
		vlog.Fatal("securityflag.PermissionsFromFlag() failed: ", err)
	}
	if perms != nil {
		vlog.Info("Using perms from command line flag.")
	} else {
		vlog.Info("Perms flag not set. Giving local principal all perms.")
		perms = defaultPerms(security.DefaultBlessingPatterns(v23.GetPrincipal(ctx)))
	}
	vlog.Infof("Perms: %v", perms)
	service, err := server.NewService(nil, nil, server.ServiceOptions{
		Perms:   perms,
		RootDir: *rootDir,
		Engine:  *engine,
	})
	if err != nil {
		vlog.Fatal("server.NewService() failed: ", err)
	}
	d := server.NewDispatcher(service)

	if _, err = xrpc.NewDispatchingServer(ctx, *name, d); err != nil {
		vlog.Fatal("xrpc.NewDispatchingServer() failed: ", err)
	}
	vlog.Info("Mounted at: ", *name)

	// Wait forever.
	<-signals.ShutdownOnSignals(ctx)
}
