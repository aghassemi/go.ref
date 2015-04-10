// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file was auto-generated by the vanadium vdl tool.
// Source: identity.vdl

// Package identity defines interfaces for Vanadium identity providers.
package identity

import (
	// VDL system imports
	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/rpc"
	"v.io/v23/vdl"

	// VDL user imports
	"v.io/v23/security"
)

// BlessingRootResponse is the struct representing the JSON response provided
// by the "blessing-root" route of the identity service.
type BlessingRootResponse struct {
	// Names of the blessings.
	Names []string
	// Base64 der-encoded public key.
	PublicKey string
}

func (BlessingRootResponse) __VDLReflect(struct {
	Name string "v.io/x/ref/services/identity.BlessingRootResponse"
}) {
}

func init() {
	vdl.Register((*BlessingRootResponse)(nil))
}

// OAuthBlesserClientMethods is the client interface
// containing OAuthBlesser methods.
//
// OAuthBlesser exchanges OAuth access tokens for
// an email address from an OAuth-based identity provider and uses the email
// address obtained to bless the client.
//
// OAuth is described in RFC 6749 (http://tools.ietf.org/html/rfc6749),
// though the Google implementation also has informative documentation at
// https://developers.google.com/accounts/docs/OAuth2
//
// WARNING: There is no binding between the channel over which the access token
// was obtained (typically https) and the channel used to make the RPC (a
// vanadium virtual circuit).
// Thus, if Mallory possesses the access token associated with Alice's account,
// she may be able to obtain a blessing with Alice's name on it.
type OAuthBlesserClientMethods interface {
	// BlessUsingAccessToken uses the provided access token to obtain the email
	// address and returns a blessing along with the email address.
	BlessUsingAccessToken(ctx *context.T, token string, opts ...rpc.CallOpt) (blessing security.Blessings, email string, err error)
}

// OAuthBlesserClientStub adds universal methods to OAuthBlesserClientMethods.
type OAuthBlesserClientStub interface {
	OAuthBlesserClientMethods
	rpc.UniversalServiceMethods
}

// OAuthBlesserClient returns a client stub for OAuthBlesser.
func OAuthBlesserClient(name string) OAuthBlesserClientStub {
	return implOAuthBlesserClientStub{name}
}

type implOAuthBlesserClientStub struct {
	name string
}

func (c implOAuthBlesserClientStub) BlessUsingAccessToken(ctx *context.T, i0 string, opts ...rpc.CallOpt) (o0 security.Blessings, o1 string, err error) {
	err = v23.GetClient(ctx).Call(ctx, c.name, "BlessUsingAccessToken", []interface{}{i0}, []interface{}{&o0, &o1}, opts...)
	return
}

// OAuthBlesserServerMethods is the interface a server writer
// implements for OAuthBlesser.
//
// OAuthBlesser exchanges OAuth access tokens for
// an email address from an OAuth-based identity provider and uses the email
// address obtained to bless the client.
//
// OAuth is described in RFC 6749 (http://tools.ietf.org/html/rfc6749),
// though the Google implementation also has informative documentation at
// https://developers.google.com/accounts/docs/OAuth2
//
// WARNING: There is no binding between the channel over which the access token
// was obtained (typically https) and the channel used to make the RPC (a
// vanadium virtual circuit).
// Thus, if Mallory possesses the access token associated with Alice's account,
// she may be able to obtain a blessing with Alice's name on it.
type OAuthBlesserServerMethods interface {
	// BlessUsingAccessToken uses the provided access token to obtain the email
	// address and returns a blessing along with the email address.
	BlessUsingAccessToken(call rpc.ServerCall, token string) (blessing security.Blessings, email string, err error)
}

// OAuthBlesserServerStubMethods is the server interface containing
// OAuthBlesser methods, as expected by rpc.Server.
// There is no difference between this interface and OAuthBlesserServerMethods
// since there are no streaming methods.
type OAuthBlesserServerStubMethods OAuthBlesserServerMethods

// OAuthBlesserServerStub adds universal methods to OAuthBlesserServerStubMethods.
type OAuthBlesserServerStub interface {
	OAuthBlesserServerStubMethods
	// Describe the OAuthBlesser interfaces.
	Describe__() []rpc.InterfaceDesc
}

// OAuthBlesserServer returns a server stub for OAuthBlesser.
// It converts an implementation of OAuthBlesserServerMethods into
// an object that may be used by rpc.Server.
func OAuthBlesserServer(impl OAuthBlesserServerMethods) OAuthBlesserServerStub {
	stub := implOAuthBlesserServerStub{
		impl: impl,
	}
	// Initialize GlobState; always check the stub itself first, to handle the
	// case where the user has the Glob method defined in their VDL source.
	if gs := rpc.NewGlobState(stub); gs != nil {
		stub.gs = gs
	} else if gs := rpc.NewGlobState(impl); gs != nil {
		stub.gs = gs
	}
	return stub
}

type implOAuthBlesserServerStub struct {
	impl OAuthBlesserServerMethods
	gs   *rpc.GlobState
}

func (s implOAuthBlesserServerStub) BlessUsingAccessToken(call rpc.ServerCall, i0 string) (security.Blessings, string, error) {
	return s.impl.BlessUsingAccessToken(call, i0)
}

func (s implOAuthBlesserServerStub) Globber() *rpc.GlobState {
	return s.gs
}

func (s implOAuthBlesserServerStub) Describe__() []rpc.InterfaceDesc {
	return []rpc.InterfaceDesc{OAuthBlesserDesc}
}

// OAuthBlesserDesc describes the OAuthBlesser interface.
var OAuthBlesserDesc rpc.InterfaceDesc = descOAuthBlesser

// descOAuthBlesser hides the desc to keep godoc clean.
var descOAuthBlesser = rpc.InterfaceDesc{
	Name:    "OAuthBlesser",
	PkgPath: "v.io/x/ref/services/identity",
	Doc:     "// OAuthBlesser exchanges OAuth access tokens for\n// an email address from an OAuth-based identity provider and uses the email\n// address obtained to bless the client.\n//\n// OAuth is described in RFC 6749 (http://tools.ietf.org/html/rfc6749),\n// though the Google implementation also has informative documentation at\n// https://developers.google.com/accounts/docs/OAuth2\n//\n// WARNING: There is no binding between the channel over which the access token\n// was obtained (typically https) and the channel used to make the RPC (a\n// vanadium virtual circuit).\n// Thus, if Mallory possesses the access token associated with Alice's account,\n// she may be able to obtain a blessing with Alice's name on it.",
	Methods: []rpc.MethodDesc{
		{
			Name: "BlessUsingAccessToken",
			Doc:  "// BlessUsingAccessToken uses the provided access token to obtain the email\n// address and returns a blessing along with the email address.",
			InArgs: []rpc.ArgDesc{
				{"token", ``}, // string
			},
			OutArgs: []rpc.ArgDesc{
				{"blessing", ``}, // security.Blessings
				{"email", ``},    // string
			},
		},
	},
}

// MacaroonBlesserClientMethods is the client interface
// containing MacaroonBlesser methods.
//
// MacaroonBlesser returns a blessing given the provided macaroon string.
type MacaroonBlesserClientMethods interface {
	// Bless uses the provided macaroon (which contains email and caveats)
	// to return a blessing for the client.
	Bless(ctx *context.T, macaroon string, opts ...rpc.CallOpt) (blessing security.Blessings, err error)
}

// MacaroonBlesserClientStub adds universal methods to MacaroonBlesserClientMethods.
type MacaroonBlesserClientStub interface {
	MacaroonBlesserClientMethods
	rpc.UniversalServiceMethods
}

// MacaroonBlesserClient returns a client stub for MacaroonBlesser.
func MacaroonBlesserClient(name string) MacaroonBlesserClientStub {
	return implMacaroonBlesserClientStub{name}
}

type implMacaroonBlesserClientStub struct {
	name string
}

func (c implMacaroonBlesserClientStub) Bless(ctx *context.T, i0 string, opts ...rpc.CallOpt) (o0 security.Blessings, err error) {
	err = v23.GetClient(ctx).Call(ctx, c.name, "Bless", []interface{}{i0}, []interface{}{&o0}, opts...)
	return
}

// MacaroonBlesserServerMethods is the interface a server writer
// implements for MacaroonBlesser.
//
// MacaroonBlesser returns a blessing given the provided macaroon string.
type MacaroonBlesserServerMethods interface {
	// Bless uses the provided macaroon (which contains email and caveats)
	// to return a blessing for the client.
	Bless(call rpc.ServerCall, macaroon string) (blessing security.Blessings, err error)
}

// MacaroonBlesserServerStubMethods is the server interface containing
// MacaroonBlesser methods, as expected by rpc.Server.
// There is no difference between this interface and MacaroonBlesserServerMethods
// since there are no streaming methods.
type MacaroonBlesserServerStubMethods MacaroonBlesserServerMethods

// MacaroonBlesserServerStub adds universal methods to MacaroonBlesserServerStubMethods.
type MacaroonBlesserServerStub interface {
	MacaroonBlesserServerStubMethods
	// Describe the MacaroonBlesser interfaces.
	Describe__() []rpc.InterfaceDesc
}

// MacaroonBlesserServer returns a server stub for MacaroonBlesser.
// It converts an implementation of MacaroonBlesserServerMethods into
// an object that may be used by rpc.Server.
func MacaroonBlesserServer(impl MacaroonBlesserServerMethods) MacaroonBlesserServerStub {
	stub := implMacaroonBlesserServerStub{
		impl: impl,
	}
	// Initialize GlobState; always check the stub itself first, to handle the
	// case where the user has the Glob method defined in their VDL source.
	if gs := rpc.NewGlobState(stub); gs != nil {
		stub.gs = gs
	} else if gs := rpc.NewGlobState(impl); gs != nil {
		stub.gs = gs
	}
	return stub
}

type implMacaroonBlesserServerStub struct {
	impl MacaroonBlesserServerMethods
	gs   *rpc.GlobState
}

func (s implMacaroonBlesserServerStub) Bless(call rpc.ServerCall, i0 string) (security.Blessings, error) {
	return s.impl.Bless(call, i0)
}

func (s implMacaroonBlesserServerStub) Globber() *rpc.GlobState {
	return s.gs
}

func (s implMacaroonBlesserServerStub) Describe__() []rpc.InterfaceDesc {
	return []rpc.InterfaceDesc{MacaroonBlesserDesc}
}

// MacaroonBlesserDesc describes the MacaroonBlesser interface.
var MacaroonBlesserDesc rpc.InterfaceDesc = descMacaroonBlesser

// descMacaroonBlesser hides the desc to keep godoc clean.
var descMacaroonBlesser = rpc.InterfaceDesc{
	Name:    "MacaroonBlesser",
	PkgPath: "v.io/x/ref/services/identity",
	Doc:     "// MacaroonBlesser returns a blessing given the provided macaroon string.",
	Methods: []rpc.MethodDesc{
		{
			Name: "Bless",
			Doc:  "// Bless uses the provided macaroon (which contains email and caveats)\n// to return a blessing for the client.",
			InArgs: []rpc.ArgDesc{
				{"macaroon", ``}, // string
			},
			OutArgs: []rpc.ArgDesc{
				{"blessing", ``}, // security.Blessings
			},
		},
	},
}
