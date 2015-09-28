// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package discovery

import (
	"github.com/pborman/uuid"

	"v.io/v23/context"
)

// Plugin is the basic interface for a plugin to discovery service.
// All implementation should be goroutine-safe.
type Plugin interface {
	// Advertise advertises the advertisement. Advertising will continue until
	// the context is canceled or exceeds its deadline.
	Advertise(ctx *context.T, ad Advertisement) error

	// Scan scans services that match the service uuid and returns scanned
	// advertisements to the channel. A zero-value service uuid means any service.
	// Scanning will continue until the context is canceled or exceeds its
	// deadline.
	//
	// TODO(jhahn): Pass a filter on service attributes.
	Scan(ctx *context.T, serviceUuid uuid.UUID, ch chan<- Advertisement) error
}
