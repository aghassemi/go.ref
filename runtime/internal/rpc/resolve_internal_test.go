// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rpc

import (
	"v.io/v23/context"
	"v.io/v23/rpc"
	"v.io/x/ref"
)

func InternalServerResolveToEndpoint(ctx *context.T, s rpc.Server, name string) (string, error) {
	if ref.RPCTransitionState() == ref.XServers {
		ep, err := s.(*xserver).resolveToEndpoint(ctx, name)
		if err != nil {
			return "", err
		}
		return ep.String(), nil
	}
	return s.(*server).resolveToEndpoint(name)
}
