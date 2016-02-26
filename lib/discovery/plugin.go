// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package discovery

import (
	"v.io/v23/context"
)

// Plugin is the basic interface for discovery plugins.
//
// All implementation should be goroutine-safe.
type Plugin interface {
	// Advertise advertises the advertisement.
	//
	// The advertisement will not be changed while it is being advertised.
	//
	// If the advertisement is too large, the plugin may drop any information
	// except Id, InterfaceName, Hash, and DirAddrs.
	//
	// Advertising should continue until the context is canceled or exceeds
	// its deadline. done should be called once when advertising is done or
	// canceled.
	Advertise(ctx *context.T, adinfo *AdInfo, done func()) error

	// Scan scans advertisements that match the interface name and returns scanned
	// advertisements to the channel.
	//
	// An empty interface name means any advertisements.
	//
	// Advertisements that are returned through the channel can be changed. The plugin
	// should not reuse the returned advertisement.
	//
	// Scanning should continue until the context is canceled or exceeds its
	// deadline. done should be called once when scanning is done or canceled.
	Scan(ctx *context.T, interfaceName string, ch chan<- *AdInfo, done func()) error
}
