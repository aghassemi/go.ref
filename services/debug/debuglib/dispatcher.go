// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package debuglib

import (
	"strings"
	"time"

	"v.io/v23/rpc"
	"v.io/v23/security"

	"v.io/x/ref/services/logreader/logreaderlib"
	"v.io/x/ref/services/pprof/pproflib"
	"v.io/x/ref/services/stats/statslib"
	"v.io/x/ref/services/vtrace/vtracelib"
)

// dispatcher holds the state of the debug dispatcher.
type dispatcher struct {
	logsDirFunc func() string // The function returns the root of the logs directory.
	auth        security.Authorizer
}

var _ rpc.Dispatcher = (*dispatcher)(nil)

func NewDispatcher(logsDirFunc func() string, authorizer security.Authorizer) rpc.Dispatcher {
	return &dispatcher{logsDirFunc, authorizer}
}

// The first part of the names of the objects served by this dispatcher.
var rootName = "__debug"

func (d *dispatcher) Lookup(suffix string) (interface{}, security.Authorizer, error) {
	if suffix == "" {
		return rpc.ChildrenGlobberInvoker(rootName), d.auth, nil
	}
	if !strings.HasPrefix(suffix, rootName) {
		return nil, nil, nil
	}
	suffix = strings.TrimPrefix(suffix, rootName)
	suffix = strings.TrimLeft(suffix, "/")

	if suffix == "" {
		return rpc.ChildrenGlobberInvoker("logs", "pprof", "stats", "vtrace"), d.auth, nil
	}
	parts := strings.SplitN(suffix, "/", 2)
	if len(parts) == 2 {
		suffix = parts[1]
	} else {
		suffix = ""
	}
	switch parts[0] {
	case "logs":
		return logreaderlib.NewLogFileService(d.logsDirFunc(), suffix), d.auth, nil
	case "pprof":
		return pproflib.NewPProfService(), d.auth, nil
	case "stats":
		return statslib.NewStatsService(suffix, 10*time.Second), d.auth, nil
	case "vtrace":
		return vtracelib.NewVtraceService(), d.auth, nil
	}
	return nil, d.auth, nil
}
