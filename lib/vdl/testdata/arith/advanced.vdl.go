// This file was auto-generated by the veyron vdl tool.
// Source: advanced.vdl

package arith

import (
	// VDL system imports
	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/ipc"

	// VDL user imports
	"v.io/x/ref/lib/vdl/testdata/arith/exp"
)

// TrigonometryClientMethods is the client interface
// containing Trigonometry methods.
//
// Trigonometry is an interface that specifies a couple trigonometric functions.
type TrigonometryClientMethods interface {
	Sine(ctx *context.T, angle float64, opts ...ipc.CallOpt) (float64, error)
	Cosine(ctx *context.T, angle float64, opts ...ipc.CallOpt) (float64, error)
}

// TrigonometryClientStub adds universal methods to TrigonometryClientMethods.
type TrigonometryClientStub interface {
	TrigonometryClientMethods
	ipc.UniversalServiceMethods
}

// TrigonometryClient returns a client stub for Trigonometry.
func TrigonometryClient(name string, opts ...ipc.BindOpt) TrigonometryClientStub {
	var client ipc.Client
	for _, opt := range opts {
		if clientOpt, ok := opt.(ipc.Client); ok {
			client = clientOpt
		}
	}
	return implTrigonometryClientStub{name, client}
}

type implTrigonometryClientStub struct {
	name   string
	client ipc.Client
}

func (c implTrigonometryClientStub) c(ctx *context.T) ipc.Client {
	if c.client != nil {
		return c.client
	}
	return v23.GetClient(ctx)
}

func (c implTrigonometryClientStub) Sine(ctx *context.T, i0 float64, opts ...ipc.CallOpt) (o0 float64, err error) {
	var call ipc.ClientCall
	if call, err = c.c(ctx).StartCall(ctx, c.name, "Sine", []interface{}{i0}, opts...); err != nil {
		return
	}
	err = call.Finish(&o0)
	return
}

func (c implTrigonometryClientStub) Cosine(ctx *context.T, i0 float64, opts ...ipc.CallOpt) (o0 float64, err error) {
	var call ipc.ClientCall
	if call, err = c.c(ctx).StartCall(ctx, c.name, "Cosine", []interface{}{i0}, opts...); err != nil {
		return
	}
	err = call.Finish(&o0)
	return
}

// TrigonometryServerMethods is the interface a server writer
// implements for Trigonometry.
//
// Trigonometry is an interface that specifies a couple trigonometric functions.
type TrigonometryServerMethods interface {
	Sine(call ipc.ServerCall, angle float64) (float64, error)
	Cosine(call ipc.ServerCall, angle float64) (float64, error)
}

// TrigonometryServerStubMethods is the server interface containing
// Trigonometry methods, as expected by ipc.Server.
// There is no difference between this interface and TrigonometryServerMethods
// since there are no streaming methods.
type TrigonometryServerStubMethods TrigonometryServerMethods

// TrigonometryServerStub adds universal methods to TrigonometryServerStubMethods.
type TrigonometryServerStub interface {
	TrigonometryServerStubMethods
	// Describe the Trigonometry interfaces.
	Describe__() []ipc.InterfaceDesc
}

// TrigonometryServer returns a server stub for Trigonometry.
// It converts an implementation of TrigonometryServerMethods into
// an object that may be used by ipc.Server.
func TrigonometryServer(impl TrigonometryServerMethods) TrigonometryServerStub {
	stub := implTrigonometryServerStub{
		impl: impl,
	}
	// Initialize GlobState; always check the stub itself first, to handle the
	// case where the user has the Glob method defined in their VDL source.
	if gs := ipc.NewGlobState(stub); gs != nil {
		stub.gs = gs
	} else if gs := ipc.NewGlobState(impl); gs != nil {
		stub.gs = gs
	}
	return stub
}

type implTrigonometryServerStub struct {
	impl TrigonometryServerMethods
	gs   *ipc.GlobState
}

func (s implTrigonometryServerStub) Sine(call ipc.ServerCall, i0 float64) (float64, error) {
	return s.impl.Sine(call, i0)
}

func (s implTrigonometryServerStub) Cosine(call ipc.ServerCall, i0 float64) (float64, error) {
	return s.impl.Cosine(call, i0)
}

func (s implTrigonometryServerStub) Globber() *ipc.GlobState {
	return s.gs
}

func (s implTrigonometryServerStub) Describe__() []ipc.InterfaceDesc {
	return []ipc.InterfaceDesc{TrigonometryDesc}
}

// TrigonometryDesc describes the Trigonometry interface.
var TrigonometryDesc ipc.InterfaceDesc = descTrigonometry

// descTrigonometry hides the desc to keep godoc clean.
var descTrigonometry = ipc.InterfaceDesc{
	Name:    "Trigonometry",
	PkgPath: "v.io/x/ref/lib/vdl/testdata/arith",
	Doc:     "// Trigonometry is an interface that specifies a couple trigonometric functions.",
	Methods: []ipc.MethodDesc{
		{
			Name: "Sine",
			InArgs: []ipc.ArgDesc{
				{"angle", ``}, // float64
			},
			OutArgs: []ipc.ArgDesc{
				{"", ``}, // float64
			},
		},
		{
			Name: "Cosine",
			InArgs: []ipc.ArgDesc{
				{"angle", ``}, // float64
			},
			OutArgs: []ipc.ArgDesc{
				{"", ``}, // float64
			},
		},
	},
}

// AdvancedMathClientMethods is the client interface
// containing AdvancedMath methods.
//
// AdvancedMath is an interface for more advanced math than arith.  It embeds
// interfaces defined both in the same file and in an external package; and in
// turn it is embedded by arith.Calculator (which is in the same package but
// different file) to verify that embedding works in all these scenarios.
type AdvancedMathClientMethods interface {
	// Trigonometry is an interface that specifies a couple trigonometric functions.
	TrigonometryClientMethods
	exp.ExpClientMethods
}

// AdvancedMathClientStub adds universal methods to AdvancedMathClientMethods.
type AdvancedMathClientStub interface {
	AdvancedMathClientMethods
	ipc.UniversalServiceMethods
}

// AdvancedMathClient returns a client stub for AdvancedMath.
func AdvancedMathClient(name string, opts ...ipc.BindOpt) AdvancedMathClientStub {
	var client ipc.Client
	for _, opt := range opts {
		if clientOpt, ok := opt.(ipc.Client); ok {
			client = clientOpt
		}
	}
	return implAdvancedMathClientStub{name, client, TrigonometryClient(name, client), exp.ExpClient(name, client)}
}

type implAdvancedMathClientStub struct {
	name   string
	client ipc.Client

	TrigonometryClientStub
	exp.ExpClientStub
}

func (c implAdvancedMathClientStub) c(ctx *context.T) ipc.Client {
	if c.client != nil {
		return c.client
	}
	return v23.GetClient(ctx)
}

// AdvancedMathServerMethods is the interface a server writer
// implements for AdvancedMath.
//
// AdvancedMath is an interface for more advanced math than arith.  It embeds
// interfaces defined both in the same file and in an external package; and in
// turn it is embedded by arith.Calculator (which is in the same package but
// different file) to verify that embedding works in all these scenarios.
type AdvancedMathServerMethods interface {
	// Trigonometry is an interface that specifies a couple trigonometric functions.
	TrigonometryServerMethods
	exp.ExpServerMethods
}

// AdvancedMathServerStubMethods is the server interface containing
// AdvancedMath methods, as expected by ipc.Server.
// There is no difference between this interface and AdvancedMathServerMethods
// since there are no streaming methods.
type AdvancedMathServerStubMethods AdvancedMathServerMethods

// AdvancedMathServerStub adds universal methods to AdvancedMathServerStubMethods.
type AdvancedMathServerStub interface {
	AdvancedMathServerStubMethods
	// Describe the AdvancedMath interfaces.
	Describe__() []ipc.InterfaceDesc
}

// AdvancedMathServer returns a server stub for AdvancedMath.
// It converts an implementation of AdvancedMathServerMethods into
// an object that may be used by ipc.Server.
func AdvancedMathServer(impl AdvancedMathServerMethods) AdvancedMathServerStub {
	stub := implAdvancedMathServerStub{
		impl: impl,
		TrigonometryServerStub: TrigonometryServer(impl),
		ExpServerStub:          exp.ExpServer(impl),
	}
	// Initialize GlobState; always check the stub itself first, to handle the
	// case where the user has the Glob method defined in their VDL source.
	if gs := ipc.NewGlobState(stub); gs != nil {
		stub.gs = gs
	} else if gs := ipc.NewGlobState(impl); gs != nil {
		stub.gs = gs
	}
	return stub
}

type implAdvancedMathServerStub struct {
	impl AdvancedMathServerMethods
	TrigonometryServerStub
	exp.ExpServerStub
	gs *ipc.GlobState
}

func (s implAdvancedMathServerStub) Globber() *ipc.GlobState {
	return s.gs
}

func (s implAdvancedMathServerStub) Describe__() []ipc.InterfaceDesc {
	return []ipc.InterfaceDesc{AdvancedMathDesc, TrigonometryDesc, exp.ExpDesc}
}

// AdvancedMathDesc describes the AdvancedMath interface.
var AdvancedMathDesc ipc.InterfaceDesc = descAdvancedMath

// descAdvancedMath hides the desc to keep godoc clean.
var descAdvancedMath = ipc.InterfaceDesc{
	Name:    "AdvancedMath",
	PkgPath: "v.io/x/ref/lib/vdl/testdata/arith",
	Doc:     "// AdvancedMath is an interface for more advanced math than arith.  It embeds\n// interfaces defined both in the same file and in an external package; and in\n// turn it is embedded by arith.Calculator (which is in the same package but\n// different file) to verify that embedding works in all these scenarios.",
	Embeds: []ipc.EmbedDesc{
		{"Trigonometry", "v.io/x/ref/lib/vdl/testdata/arith", "// Trigonometry is an interface that specifies a couple trigonometric functions."},
		{"Exp", "v.io/x/ref/lib/vdl/testdata/arith/exp", ``},
	},
}
