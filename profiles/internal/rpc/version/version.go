// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package version

import (
	"fmt"

	inaming "v.io/x/ref/profiles/internal/naming"

	"v.io/v23/naming"
	"v.io/v23/rpc/version"
)

// Range represents a range of RPC versions.
type Range struct {
	Min, Max version.RPCVersion
}

var (
	// SupportedRange represents the range of protocol verions supported by this
	// implementation.
	// Max should be incremented whenever we make a protocol
	// change that's not both forward and backward compatible.
	// Min should be incremented whenever we want to remove
	// support for old protocol versions.
	SupportedRange = &Range{Min: version.RPCVersion5, Max: version.RPCVersion8}

	// Export the methods on supportedRange.
	Endpoint           = SupportedRange.Endpoint
	ProxiedEndpoint    = SupportedRange.ProxiedEndpoint
	CommonVersion      = SupportedRange.CommonVersion
	CheckCompatibility = SupportedRange.CheckCompatibility
)

var (
	NoCompatibleVersionErr = fmt.Errorf("No compatible RPC version available")
	UnknownVersionErr      = fmt.Errorf("There was not enough information to determine a version.")
)

// IsVersionError returns true if err is a versioning related error.
func IsVersionError(err error) bool {
	return err == NoCompatibleVersionErr || err == UnknownVersionErr
}

// Endpoint returns an endpoint with the Min/MaxRPCVersion properly filled in
// to match this implementations supported protocol versions.
func (r *Range) Endpoint(protocol, address string, rid naming.RoutingID) *inaming.Endpoint {
	return &inaming.Endpoint{
		Protocol:      protocol,
		Address:       address,
		RID:           rid,
		MinRPCVersion: r.Min,
		MaxRPCVersion: r.Max,
	}
}

// intersectRanges finds the intersection between ranges
// supported by two endpoints.  We make an assumption here that if one
// of the endpoints has an UnknownVersion we assume it has the same
// extent as the other endpoint. If both endpoints have Unknown for a
// version number, an error is produced.
// For example:
//   a == (2, 4) and b == (Unknown, Unknown), intersect(a,b) == (2, 4)
//   a == (2, Unknown) and b == (3, 4), intersect(a,b) == (3, 4)
func intersectRanges(amin, amax, bmin, bmax version.RPCVersion) (min, max version.RPCVersion, err error) {
	u := version.UnknownRPCVersion

	min = amin
	if min == u || (bmin != u && bmin > min) {
		min = bmin
	}
	max = amax
	if max == u || (bmax != u && bmax < max) {
		max = bmax
	}

	if min == u || max == u {
		err = UnknownVersionErr
	} else if min > max {
		err = NoCompatibleVersionErr
	}
	return
}

func intersectEndpoints(a, b *inaming.Endpoint) (min, max version.RPCVersion, err error) {
	return intersectRanges(a.MinRPCVersion, a.MaxRPCVersion, b.MinRPCVersion, b.MaxRPCVersion)
}

func (r1 *Range) Intersect(r2 *Range) (*Range, error) {
	min, max, err := intersectRanges(r1.Min, r1.Max, r2.Min, r2.Max)
	if err != nil {
		return nil, err
	}
	r := &Range{Min: min, Max: max}
	return r, nil
}

// ProxiedEndpoint returns an endpoint with the Min/MaxRPCVersion properly filled in
// to match the intersection of capabilities of this process and the proxy.
func (r *Range) ProxiedEndpoint(rid naming.RoutingID, proxy naming.Endpoint) (*inaming.Endpoint, error) {
	proxyEP, ok := proxy.(*inaming.Endpoint)
	if !ok {
		return nil, fmt.Errorf("unrecognized naming.Endpoint type %T", proxy)
	}

	ep := &inaming.Endpoint{
		Protocol:      proxyEP.Protocol,
		Address:       proxyEP.Address,
		RID:           rid,
		MinRPCVersion: r.Min,
		MaxRPCVersion: r.Max,
	}

	// This is the endpoint we are going to advertise.  It should only claim to support versions in
	// the intersection of those we support and those the proxy supports.
	var err error
	ep.MinRPCVersion, ep.MaxRPCVersion, err = intersectEndpoints(ep, proxyEP)
	if err != nil {
		return nil, fmt.Errorf("attempting to register with incompatible proxy: %s", proxy)
	}
	return ep, nil
}

// CommonVersion determines which version of the RPC protocol should be used
// between two endpoints.  Returns an error if the resulting version is incompatible
// with this RPC implementation.
func (r *Range) CommonVersion(a, b naming.Endpoint) (version.RPCVersion, error) {
	aEP, ok := a.(*inaming.Endpoint)
	if !ok {
		return 0, fmt.Errorf("Unrecognized naming.Endpoint type: %T", a)
	}
	bEP, ok := b.(*inaming.Endpoint)
	if !ok {
		return 0, fmt.Errorf("Unrecognized naming.Endpoint type: %T", b)
	}

	_, max, err := intersectEndpoints(aEP, bEP)
	if err != nil {
		return 0, err
	}

	// We want to use the maximum common version of the protocol.  We just
	// need to make sure that it is supported by this RPC implementation.
	if max < r.Min || max > r.Max {
		return version.UnknownRPCVersion, NoCompatibleVersionErr
	}
	return max, nil
}

// CheckCompatibility returns an error if the given endpoint is incompatible
// with this RPC implementation.  It returns nil otherwise.
func (r *Range) CheckCompatibility(remote naming.Endpoint) error {
	remoteEP, ok := remote.(*inaming.Endpoint)
	if !ok {
		return fmt.Errorf("Unrecognized naming.Endpoint type: %T", remote)
	}

	_, _, err := intersectRanges(r.Min, r.Max,
		remoteEP.MinRPCVersion, remoteEP.MaxRPCVersion)

	return err
}
