// This file was auto-generated by the vanadium vdl tool.
// Source: benchmark.vdl

// package benchmark provides simple tools to measure the performance of the
// IPC system.
package benchmark

import (
	// VDL system imports
	"io"
	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/rpc"
	"v.io/v23/vdl"

	// VDL user imports
	"v.io/v23/services/security/access"
)

// BenchmarkClientMethods is the client interface
// containing Benchmark methods.
type BenchmarkClientMethods interface {
	// Echo returns the payload that it receives.
	Echo(ctx *context.T, Payload []byte, opts ...rpc.CallOpt) ([]byte, error)
	// EchoStream returns the payload that it receives via the stream.
	EchoStream(*context.T, ...rpc.CallOpt) (BenchmarkEchoStreamClientCall, error)
}

// BenchmarkClientStub adds universal methods to BenchmarkClientMethods.
type BenchmarkClientStub interface {
	BenchmarkClientMethods
	rpc.UniversalServiceMethods
}

// BenchmarkClient returns a client stub for Benchmark.
func BenchmarkClient(name string, opts ...rpc.BindOpt) BenchmarkClientStub {
	var client rpc.Client
	for _, opt := range opts {
		if clientOpt, ok := opt.(rpc.Client); ok {
			client = clientOpt
		}
	}
	return implBenchmarkClientStub{name, client}
}

type implBenchmarkClientStub struct {
	name   string
	client rpc.Client
}

func (c implBenchmarkClientStub) c(ctx *context.T) rpc.Client {
	if c.client != nil {
		return c.client
	}
	return v23.GetClient(ctx)
}

func (c implBenchmarkClientStub) Echo(ctx *context.T, i0 []byte, opts ...rpc.CallOpt) (o0 []byte, err error) {
	var call rpc.ClientCall
	if call, err = c.c(ctx).StartCall(ctx, c.name, "Echo", []interface{}{i0}, opts...); err != nil {
		return
	}
	err = call.Finish(&o0)
	return
}

func (c implBenchmarkClientStub) EchoStream(ctx *context.T, opts ...rpc.CallOpt) (ocall BenchmarkEchoStreamClientCall, err error) {
	var call rpc.ClientCall
	if call, err = c.c(ctx).StartCall(ctx, c.name, "EchoStream", nil, opts...); err != nil {
		return
	}
	ocall = &implBenchmarkEchoStreamClientCall{ClientCall: call}
	return
}

// BenchmarkEchoStreamClientStream is the client stream for Benchmark.EchoStream.
type BenchmarkEchoStreamClientStream interface {
	// RecvStream returns the receiver side of the Benchmark.EchoStream client stream.
	RecvStream() interface {
		// Advance stages an item so that it may be retrieved via Value.  Returns
		// true iff there is an item to retrieve.  Advance must be called before
		// Value is called.  May block if an item is not available.
		Advance() bool
		// Value returns the item that was staged by Advance.  May panic if Advance
		// returned false or was not called.  Never blocks.
		Value() []byte
		// Err returns any error encountered by Advance.  Never blocks.
		Err() error
	}
	// SendStream returns the send side of the Benchmark.EchoStream client stream.
	SendStream() interface {
		// Send places the item onto the output stream.  Returns errors
		// encountered while sending, or if Send is called after Close or
		// the stream has been canceled.  Blocks if there is no buffer
		// space; will unblock when buffer space is available or after
		// the stream has been canceled.
		Send(item []byte) error
		// Close indicates to the server that no more items will be sent;
		// server Recv calls will receive io.EOF after all sent items.
		// This is an optional call - e.g. a client might call Close if it
		// needs to continue receiving items from the server after it's
		// done sending.  Returns errors encountered while closing, or if
		// Close is called after the stream has been canceled.  Like Send,
		// blocks if there is no buffer space available.
		Close() error
	}
}

// BenchmarkEchoStreamClientCall represents the call returned from Benchmark.EchoStream.
type BenchmarkEchoStreamClientCall interface {
	BenchmarkEchoStreamClientStream
	// Finish performs the equivalent of SendStream().Close, then blocks until
	// the server is done, and returns the positional return values for the call.
	//
	// Finish returns immediately if the call has been canceled; depending on the
	// timing the output could either be an error signaling cancelation, or the
	// valid positional return values from the server.
	//
	// Calling Finish is mandatory for releasing stream resources, unless the call
	// has been canceled or any of the other methods return an error.  Finish should
	// be called at most once.
	Finish() error
}

type implBenchmarkEchoStreamClientCall struct {
	rpc.ClientCall
	valRecv []byte
	errRecv error
}

func (c *implBenchmarkEchoStreamClientCall) RecvStream() interface {
	Advance() bool
	Value() []byte
	Err() error
} {
	return implBenchmarkEchoStreamClientCallRecv{c}
}

type implBenchmarkEchoStreamClientCallRecv struct {
	c *implBenchmarkEchoStreamClientCall
}

func (c implBenchmarkEchoStreamClientCallRecv) Advance() bool {
	c.c.errRecv = c.c.Recv(&c.c.valRecv)
	return c.c.errRecv == nil
}
func (c implBenchmarkEchoStreamClientCallRecv) Value() []byte {
	return c.c.valRecv
}
func (c implBenchmarkEchoStreamClientCallRecv) Err() error {
	if c.c.errRecv == io.EOF {
		return nil
	}
	return c.c.errRecv
}
func (c *implBenchmarkEchoStreamClientCall) SendStream() interface {
	Send(item []byte) error
	Close() error
} {
	return implBenchmarkEchoStreamClientCallSend{c}
}

type implBenchmarkEchoStreamClientCallSend struct {
	c *implBenchmarkEchoStreamClientCall
}

func (c implBenchmarkEchoStreamClientCallSend) Send(item []byte) error {
	return c.c.Send(item)
}
func (c implBenchmarkEchoStreamClientCallSend) Close() error {
	return c.c.CloseSend()
}
func (c *implBenchmarkEchoStreamClientCall) Finish() (err error) {
	err = c.ClientCall.Finish()
	return
}

// BenchmarkServerMethods is the interface a server writer
// implements for Benchmark.
type BenchmarkServerMethods interface {
	// Echo returns the payload that it receives.
	Echo(call rpc.ServerCall, Payload []byte) ([]byte, error)
	// EchoStream returns the payload that it receives via the stream.
	EchoStream(BenchmarkEchoStreamServerCall) error
}

// BenchmarkServerStubMethods is the server interface containing
// Benchmark methods, as expected by rpc.Server.
// The only difference between this interface and BenchmarkServerMethods
// is the streaming methods.
type BenchmarkServerStubMethods interface {
	// Echo returns the payload that it receives.
	Echo(call rpc.ServerCall, Payload []byte) ([]byte, error)
	// EchoStream returns the payload that it receives via the stream.
	EchoStream(*BenchmarkEchoStreamServerCallStub) error
}

// BenchmarkServerStub adds universal methods to BenchmarkServerStubMethods.
type BenchmarkServerStub interface {
	BenchmarkServerStubMethods
	// Describe the Benchmark interfaces.
	Describe__() []rpc.InterfaceDesc
}

// BenchmarkServer returns a server stub for Benchmark.
// It converts an implementation of BenchmarkServerMethods into
// an object that may be used by rpc.Server.
func BenchmarkServer(impl BenchmarkServerMethods) BenchmarkServerStub {
	stub := implBenchmarkServerStub{
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

type implBenchmarkServerStub struct {
	impl BenchmarkServerMethods
	gs   *rpc.GlobState
}

func (s implBenchmarkServerStub) Echo(call rpc.ServerCall, i0 []byte) ([]byte, error) {
	return s.impl.Echo(call, i0)
}

func (s implBenchmarkServerStub) EchoStream(call *BenchmarkEchoStreamServerCallStub) error {
	return s.impl.EchoStream(call)
}

func (s implBenchmarkServerStub) Globber() *rpc.GlobState {
	return s.gs
}

func (s implBenchmarkServerStub) Describe__() []rpc.InterfaceDesc {
	return []rpc.InterfaceDesc{BenchmarkDesc}
}

// BenchmarkDesc describes the Benchmark interface.
var BenchmarkDesc rpc.InterfaceDesc = descBenchmark

// descBenchmark hides the desc to keep godoc clean.
var descBenchmark = rpc.InterfaceDesc{
	Name:    "Benchmark",
	PkgPath: "v.io/x/ref/profiles/internal/rpc/benchmark",
	Methods: []rpc.MethodDesc{
		{
			Name: "Echo",
			Doc:  "// Echo returns the payload that it receives.",
			InArgs: []rpc.ArgDesc{
				{"Payload", ``}, // []byte
			},
			OutArgs: []rpc.ArgDesc{
				{"", ``}, // []byte
			},
			Tags: []*vdl.Value{vdl.ValueOf(access.Tag("Read"))},
		},
		{
			Name: "EchoStream",
			Doc:  "// EchoStream returns the payload that it receives via the stream.",
			Tags: []*vdl.Value{vdl.ValueOf(access.Tag("Read"))},
		},
	},
}

// BenchmarkEchoStreamServerStream is the server stream for Benchmark.EchoStream.
type BenchmarkEchoStreamServerStream interface {
	// RecvStream returns the receiver side of the Benchmark.EchoStream server stream.
	RecvStream() interface {
		// Advance stages an item so that it may be retrieved via Value.  Returns
		// true iff there is an item to retrieve.  Advance must be called before
		// Value is called.  May block if an item is not available.
		Advance() bool
		// Value returns the item that was staged by Advance.  May panic if Advance
		// returned false or was not called.  Never blocks.
		Value() []byte
		// Err returns any error encountered by Advance.  Never blocks.
		Err() error
	}
	// SendStream returns the send side of the Benchmark.EchoStream server stream.
	SendStream() interface {
		// Send places the item onto the output stream.  Returns errors encountered
		// while sending.  Blocks if there is no buffer space; will unblock when
		// buffer space is available.
		Send(item []byte) error
	}
}

// BenchmarkEchoStreamServerCall represents the context passed to Benchmark.EchoStream.
type BenchmarkEchoStreamServerCall interface {
	rpc.ServerCall
	BenchmarkEchoStreamServerStream
}

// BenchmarkEchoStreamServerCallStub is a wrapper that converts rpc.StreamServerCall into
// a typesafe stub that implements BenchmarkEchoStreamServerCall.
type BenchmarkEchoStreamServerCallStub struct {
	rpc.StreamServerCall
	valRecv []byte
	errRecv error
}

// Init initializes BenchmarkEchoStreamServerCallStub from rpc.StreamServerCall.
func (s *BenchmarkEchoStreamServerCallStub) Init(call rpc.StreamServerCall) {
	s.StreamServerCall = call
}

// RecvStream returns the receiver side of the Benchmark.EchoStream server stream.
func (s *BenchmarkEchoStreamServerCallStub) RecvStream() interface {
	Advance() bool
	Value() []byte
	Err() error
} {
	return implBenchmarkEchoStreamServerCallRecv{s}
}

type implBenchmarkEchoStreamServerCallRecv struct {
	s *BenchmarkEchoStreamServerCallStub
}

func (s implBenchmarkEchoStreamServerCallRecv) Advance() bool {
	s.s.errRecv = s.s.Recv(&s.s.valRecv)
	return s.s.errRecv == nil
}
func (s implBenchmarkEchoStreamServerCallRecv) Value() []byte {
	return s.s.valRecv
}
func (s implBenchmarkEchoStreamServerCallRecv) Err() error {
	if s.s.errRecv == io.EOF {
		return nil
	}
	return s.s.errRecv
}

// SendStream returns the send side of the Benchmark.EchoStream server stream.
func (s *BenchmarkEchoStreamServerCallStub) SendStream() interface {
	Send(item []byte) error
} {
	return implBenchmarkEchoStreamServerCallSend{s}
}

type implBenchmarkEchoStreamServerCallSend struct {
	s *BenchmarkEchoStreamServerCallStub
}

func (s implBenchmarkEchoStreamServerCallSend) Send(item []byte) error {
	return s.s.Send(item)
}