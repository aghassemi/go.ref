// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file was auto-generated by the vanadium vdl tool.
// Source: repository.vdl

// Package repository augments the v.io/v23/services/repository interfaces with
// implementation-specific configuration methods.
package repository

import (
	// VDL system imports
	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/rpc"
	"v.io/v23/vdl"

	// VDL user imports
	"v.io/v23/security/access"
	"v.io/v23/services/application"
	"v.io/v23/services/permissions"
	"v.io/v23/services/repository"
	"v.io/x/ref/services/profile"
)

// ApplicationClientMethods is the client interface
// containing Application methods.
//
// Application describes an application repository internally. Besides
// the public Application interface, it allows to add and remove
// application envelopes.
type ApplicationClientMethods interface {
	// Application provides access to application envelopes. An
	// application envelope is identified by an application name and an
	// application version, which are specified through the object name,
	// and a profile name, which is specified using a method argument.
	//
	// Example:
	// /apps/search/v1.Match([]string{"base", "media"})
	//   returns an application envelope that can be used for downloading
	//   and executing the "search" application, version "v1", runnable
	//   on either the "base" or "media" profile.
	repository.ApplicationClientMethods
	// Put adds the given tuple of application version (specified
	// through the object name suffix) and application envelope to all
	// of the given application profiles.
	Put(ctx *context.T, Profiles []string, Envelope application.Envelope, opts ...rpc.CallOpt) error
	// Remove removes the application envelope for the given profile
	// name and application version (specified through the object name
	// suffix). If no version is specified as part of the suffix, the
	// method removes all versions for the given profile.
	//
	// TODO(jsimsa): Add support for using "*" to specify all profiles
	// when Matt implements Globing (or Ken implements querying).
	Remove(ctx *context.T, Profile string, opts ...rpc.CallOpt) error
}

// ApplicationClientStub adds universal methods to ApplicationClientMethods.
type ApplicationClientStub interface {
	ApplicationClientMethods
	rpc.UniversalServiceMethods
}

// ApplicationClient returns a client stub for Application.
func ApplicationClient(name string) ApplicationClientStub {
	return implApplicationClientStub{name, repository.ApplicationClient(name)}
}

type implApplicationClientStub struct {
	name string

	repository.ApplicationClientStub
}

func (c implApplicationClientStub) Put(ctx *context.T, i0 []string, i1 application.Envelope, opts ...rpc.CallOpt) (err error) {
	err = v23.GetClient(ctx).Call(ctx, c.name, "Put", []interface{}{i0, i1}, nil, opts...)
	return
}

func (c implApplicationClientStub) Remove(ctx *context.T, i0 string, opts ...rpc.CallOpt) (err error) {
	err = v23.GetClient(ctx).Call(ctx, c.name, "Remove", []interface{}{i0}, nil, opts...)
	return
}

// ApplicationServerMethods is the interface a server writer
// implements for Application.
//
// Application describes an application repository internally. Besides
// the public Application interface, it allows to add and remove
// application envelopes.
type ApplicationServerMethods interface {
	// Application provides access to application envelopes. An
	// application envelope is identified by an application name and an
	// application version, which are specified through the object name,
	// and a profile name, which is specified using a method argument.
	//
	// Example:
	// /apps/search/v1.Match([]string{"base", "media"})
	//   returns an application envelope that can be used for downloading
	//   and executing the "search" application, version "v1", runnable
	//   on either the "base" or "media" profile.
	repository.ApplicationServerMethods
	// Put adds the given tuple of application version (specified
	// through the object name suffix) and application envelope to all
	// of the given application profiles.
	Put(call rpc.ServerCall, Profiles []string, Envelope application.Envelope) error
	// Remove removes the application envelope for the given profile
	// name and application version (specified through the object name
	// suffix). If no version is specified as part of the suffix, the
	// method removes all versions for the given profile.
	//
	// TODO(jsimsa): Add support for using "*" to specify all profiles
	// when Matt implements Globing (or Ken implements querying).
	Remove(call rpc.ServerCall, Profile string) error
}

// ApplicationServerStubMethods is the server interface containing
// Application methods, as expected by rpc.Server.
// There is no difference between this interface and ApplicationServerMethods
// since there are no streaming methods.
type ApplicationServerStubMethods ApplicationServerMethods

// ApplicationServerStub adds universal methods to ApplicationServerStubMethods.
type ApplicationServerStub interface {
	ApplicationServerStubMethods
	// Describe the Application interfaces.
	Describe__() []rpc.InterfaceDesc
}

// ApplicationServer returns a server stub for Application.
// It converts an implementation of ApplicationServerMethods into
// an object that may be used by rpc.Server.
func ApplicationServer(impl ApplicationServerMethods) ApplicationServerStub {
	stub := implApplicationServerStub{
		impl: impl,
		ApplicationServerStub: repository.ApplicationServer(impl),
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

type implApplicationServerStub struct {
	impl ApplicationServerMethods
	repository.ApplicationServerStub
	gs *rpc.GlobState
}

func (s implApplicationServerStub) Put(call rpc.ServerCall, i0 []string, i1 application.Envelope) error {
	return s.impl.Put(call, i0, i1)
}

func (s implApplicationServerStub) Remove(call rpc.ServerCall, i0 string) error {
	return s.impl.Remove(call, i0)
}

func (s implApplicationServerStub) Globber() *rpc.GlobState {
	return s.gs
}

func (s implApplicationServerStub) Describe__() []rpc.InterfaceDesc {
	return []rpc.InterfaceDesc{ApplicationDesc, repository.ApplicationDesc, permissions.ObjectDesc}
}

// ApplicationDesc describes the Application interface.
var ApplicationDesc rpc.InterfaceDesc = descApplication

// descApplication hides the desc to keep godoc clean.
var descApplication = rpc.InterfaceDesc{
	Name:    "Application",
	PkgPath: "v.io/x/ref/services/repository",
	Doc:     "// Application describes an application repository internally. Besides\n// the public Application interface, it allows to add and remove\n// application envelopes.",
	Embeds: []rpc.EmbedDesc{
		{"Application", "v.io/v23/services/repository", "// Application provides access to application envelopes. An\n// application envelope is identified by an application name and an\n// application version, which are specified through the object name,\n// and a profile name, which is specified using a method argument.\n//\n// Example:\n// /apps/search/v1.Match([]string{\"base\", \"media\"})\n//   returns an application envelope that can be used for downloading\n//   and executing the \"search\" application, version \"v1\", runnable\n//   on either the \"base\" or \"media\" profile."},
	},
	Methods: []rpc.MethodDesc{
		{
			Name: "Put",
			Doc:  "// Put adds the given tuple of application version (specified\n// through the object name suffix) and application envelope to all\n// of the given application profiles.",
			InArgs: []rpc.ArgDesc{
				{"Profiles", ``}, // []string
				{"Envelope", ``}, // application.Envelope
			},
			Tags: []*vdl.Value{vdl.ValueOf(access.Tag("Write"))},
		},
		{
			Name: "Remove",
			Doc:  "// Remove removes the application envelope for the given profile\n// name and application version (specified through the object name\n// suffix). If no version is specified as part of the suffix, the\n// method removes all versions for the given profile.\n//\n// TODO(jsimsa): Add support for using \"*\" to specify all profiles\n// when Matt implements Globing (or Ken implements querying).",
			InArgs: []rpc.ArgDesc{
				{"Profile", ``}, // string
			},
			Tags: []*vdl.Value{vdl.ValueOf(access.Tag("Write"))},
		},
	},
}

// ProfileClientMethods is the client interface
// containing Profile methods.
//
// Profile describes a profile internally. Besides the public Profile
// interface, it allows to add and remove profile specifications.
type ProfileClientMethods interface {
	// Profile abstracts a device's ability to run binaries, and hides
	// specifics such as the operating system, hardware architecture, and
	// the set of installed libraries. Profiles describe binaries and
	// devices, and are used to match them.
	repository.ProfileClientMethods
	// Specification returns the profile specification for the profile
	// identified through the object name suffix.
	Specification(*context.T, ...rpc.CallOpt) (profile.Specification, error)
	// Put sets the profile specification for the profile identified
	// through the object name suffix.
	Put(ctx *context.T, Specification profile.Specification, opts ...rpc.CallOpt) error
	// Remove removes the profile specification for the profile
	// identified through the object name suffix.
	Remove(*context.T, ...rpc.CallOpt) error
}

// ProfileClientStub adds universal methods to ProfileClientMethods.
type ProfileClientStub interface {
	ProfileClientMethods
	rpc.UniversalServiceMethods
}

// ProfileClient returns a client stub for Profile.
func ProfileClient(name string) ProfileClientStub {
	return implProfileClientStub{name, repository.ProfileClient(name)}
}

type implProfileClientStub struct {
	name string

	repository.ProfileClientStub
}

func (c implProfileClientStub) Specification(ctx *context.T, opts ...rpc.CallOpt) (o0 profile.Specification, err error) {
	err = v23.GetClient(ctx).Call(ctx, c.name, "Specification", nil, []interface{}{&o0}, opts...)
	return
}

func (c implProfileClientStub) Put(ctx *context.T, i0 profile.Specification, opts ...rpc.CallOpt) (err error) {
	err = v23.GetClient(ctx).Call(ctx, c.name, "Put", []interface{}{i0}, nil, opts...)
	return
}

func (c implProfileClientStub) Remove(ctx *context.T, opts ...rpc.CallOpt) (err error) {
	err = v23.GetClient(ctx).Call(ctx, c.name, "Remove", nil, nil, opts...)
	return
}

// ProfileServerMethods is the interface a server writer
// implements for Profile.
//
// Profile describes a profile internally. Besides the public Profile
// interface, it allows to add and remove profile specifications.
type ProfileServerMethods interface {
	// Profile abstracts a device's ability to run binaries, and hides
	// specifics such as the operating system, hardware architecture, and
	// the set of installed libraries. Profiles describe binaries and
	// devices, and are used to match them.
	repository.ProfileServerMethods
	// Specification returns the profile specification for the profile
	// identified through the object name suffix.
	Specification(rpc.ServerCall) (profile.Specification, error)
	// Put sets the profile specification for the profile identified
	// through the object name suffix.
	Put(call rpc.ServerCall, Specification profile.Specification) error
	// Remove removes the profile specification for the profile
	// identified through the object name suffix.
	Remove(rpc.ServerCall) error
}

// ProfileServerStubMethods is the server interface containing
// Profile methods, as expected by rpc.Server.
// There is no difference between this interface and ProfileServerMethods
// since there are no streaming methods.
type ProfileServerStubMethods ProfileServerMethods

// ProfileServerStub adds universal methods to ProfileServerStubMethods.
type ProfileServerStub interface {
	ProfileServerStubMethods
	// Describe the Profile interfaces.
	Describe__() []rpc.InterfaceDesc
}

// ProfileServer returns a server stub for Profile.
// It converts an implementation of ProfileServerMethods into
// an object that may be used by rpc.Server.
func ProfileServer(impl ProfileServerMethods) ProfileServerStub {
	stub := implProfileServerStub{
		impl:              impl,
		ProfileServerStub: repository.ProfileServer(impl),
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

type implProfileServerStub struct {
	impl ProfileServerMethods
	repository.ProfileServerStub
	gs *rpc.GlobState
}

func (s implProfileServerStub) Specification(call rpc.ServerCall) (profile.Specification, error) {
	return s.impl.Specification(call)
}

func (s implProfileServerStub) Put(call rpc.ServerCall, i0 profile.Specification) error {
	return s.impl.Put(call, i0)
}

func (s implProfileServerStub) Remove(call rpc.ServerCall) error {
	return s.impl.Remove(call)
}

func (s implProfileServerStub) Globber() *rpc.GlobState {
	return s.gs
}

func (s implProfileServerStub) Describe__() []rpc.InterfaceDesc {
	return []rpc.InterfaceDesc{ProfileDesc, repository.ProfileDesc}
}

// ProfileDesc describes the Profile interface.
var ProfileDesc rpc.InterfaceDesc = descProfile

// descProfile hides the desc to keep godoc clean.
var descProfile = rpc.InterfaceDesc{
	Name:    "Profile",
	PkgPath: "v.io/x/ref/services/repository",
	Doc:     "// Profile describes a profile internally. Besides the public Profile\n// interface, it allows to add and remove profile specifications.",
	Embeds: []rpc.EmbedDesc{
		{"Profile", "v.io/v23/services/repository", "// Profile abstracts a device's ability to run binaries, and hides\n// specifics such as the operating system, hardware architecture, and\n// the set of installed libraries. Profiles describe binaries and\n// devices, and are used to match them."},
	},
	Methods: []rpc.MethodDesc{
		{
			Name: "Specification",
			Doc:  "// Specification returns the profile specification for the profile\n// identified through the object name suffix.",
			OutArgs: []rpc.ArgDesc{
				{"", ``}, // profile.Specification
			},
			Tags: []*vdl.Value{vdl.ValueOf(access.Tag("Read"))},
		},
		{
			Name: "Put",
			Doc:  "// Put sets the profile specification for the profile identified\n// through the object name suffix.",
			InArgs: []rpc.ArgDesc{
				{"Specification", ``}, // profile.Specification
			},
			Tags: []*vdl.Value{vdl.ValueOf(access.Tag("Write"))},
		},
		{
			Name: "Remove",
			Doc:  "// Remove removes the profile specification for the profile\n// identified through the object name suffix.",
			Tags: []*vdl.Value{vdl.ValueOf(access.Tag("Write"))},
		},
	},
}
