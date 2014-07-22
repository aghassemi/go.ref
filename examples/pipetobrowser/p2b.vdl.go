// This file was auto-generated by the veyron vdl tool.
// Source: p2b.vdl

package pipetobrowser

import (
	// The non-user imports are prefixed with "_gen_" to prevent collisions.
	_gen_io "io"
	_gen_veyron2 "veyron2"
	_gen_context "veyron2/context"
	_gen_ipc "veyron2/ipc"
	_gen_naming "veyron2/naming"
	_gen_rt "veyron2/rt"
	_gen_vdlutil "veyron2/vdl/vdlutil"
	_gen_wiretype "veyron2/wiretype"
)

// TODO(bprosnitz) Remove this line once signatures are updated to use typevals.
// It corrects a bug where _gen_wiretype is unused in VDL pacakges where only bootstrap types are used on interfaces.
const _ = _gen_wiretype.TypeIDInvalid

// Viewer allows clients to stream data to it and to request a particular viewer to format and display the data.
// Viewer is the interface the client binds and uses.
// Viewer_ExcludingUniversal is the interface without internal framework-added methods
// to enable embedding without method collisions.  Not to be used directly by clients.
type Viewer_ExcludingUniversal interface {
	// Pipe creates a bidirectional pipe between client and viewer service, returns total number of bytes received by the service after streaming ends
	Pipe(ctx _gen_context.T, opts ..._gen_ipc.CallOpt) (reply ViewerPipeStream, err error)
}
type Viewer interface {
	_gen_ipc.UniversalServiceMethods
	Viewer_ExcludingUniversal
}

// ViewerService is the interface the server implements.
type ViewerService interface {

	// Pipe creates a bidirectional pipe between client and viewer service, returns total number of bytes received by the service after streaming ends
	Pipe(context _gen_ipc.ServerContext, stream ViewerServicePipeStream) (reply _gen_vdlutil.Any, err error)
}

// ViewerPipeStream is the interface for streaming responses of the method
// Pipe in the service interface Viewer.
type ViewerPipeStream interface {

	// Send places the item onto the output stream, blocking if there is no
	// buffer space available.  Calls to Send after having called CloseSend
	// or Cancel will fail.  Any blocked Send calls will be unblocked upon
	// calling Cancel.
	Send(item []byte) error

	// CloseSend indicates to the server that no more items will be sent;
	// server Recv calls will receive io.EOF after all sent items.  This is
	// an optional call - it's used by streaming clients that need the
	// server to receive the io.EOF terminator before the client calls
	// Finish (for example, if the client needs to continue receiving items
	// from the server after having finished sending).
	// Calls to CloseSend after having called Cancel will fail.
	// Like Send, CloseSend blocks when there's no buffer space available.
	CloseSend() error

	// Finish performs the equivalent of CloseSend, then blocks until the server
	// is done, and returns the positional return values for call.
	//
	// If Cancel has been called, Finish will return immediately; the output of
	// Finish could either be an error signalling cancelation, or the correct
	// positional return values from the server depending on the timing of the
	// call.
	//
	// Calling Finish is mandatory for releasing stream resources, unless Cancel
	// has been called or any of the other methods return a non-EOF error.
	// Finish should be called at most once.
	Finish() (reply _gen_vdlutil.Any, err error)

	// Cancel cancels the RPC, notifying the server to stop processing.  It
	// is safe to call Cancel concurrently with any of the other stream methods.
	// Calling Cancel after Finish has returned is a no-op.
	Cancel()
}

// Implementation of the ViewerPipeStream interface that is not exported.
type implViewerPipeStream struct {
	clientCall _gen_ipc.Call
}

func (c *implViewerPipeStream) Send(item []byte) error {
	return c.clientCall.Send(item)
}

func (c *implViewerPipeStream) CloseSend() error {
	return c.clientCall.CloseSend()
}

func (c *implViewerPipeStream) Finish() (reply _gen_vdlutil.Any, err error) {
	if ierr := c.clientCall.Finish(&reply, &err); ierr != nil {
		err = ierr
	}
	return
}

func (c *implViewerPipeStream) Cancel() {
	c.clientCall.Cancel()
}

// ViewerServicePipeStream is the interface for streaming responses of the method
// Pipe in the service interface Viewer.
type ViewerServicePipeStream interface {

	// Advance stages an element so the client can retrieve it
	// with Value.  Advance returns true iff there is an
	// element to retrieve.  The client must call Advance before
	// calling Value.  The client must call Cancel if it does
	// not iterate through all elements (i.e. until Advance
	// returns false).  Advance may block if an element is not
	// immediately available.
	Advance() bool

	// Value returns the element that was staged by Advance.
	// Value may panic if Advance returned false or was not
	// called at all.  Value does not block.
	//
	// In general, Value is undefined if the underlying collection
	// of elements changes while iteration is in progress.  If
	// <DataProvider> supports concurrent modification, it should
	// document its behavior.
	Value() []byte

	// Err returns a non-nil error iff the stream encountered
	// any errors.  Err does not block.
	Err() error
}

// Implementation of the ViewerServicePipeStream interface that is not exported.
type implViewerServicePipeStream struct {
	serverCall _gen_ipc.ServerCall
	val        []byte
	err        error
}

func (s *implViewerServicePipeStream) Advance() bool {
	s.err = s.serverCall.Recv(&s.val)
	return s.err == nil
}

func (s *implViewerServicePipeStream) Value() []byte {
	return s.val
}

func (s *implViewerServicePipeStream) Err() error {
	if s.err == _gen_io.EOF {
		return nil
	}
	return s.err
}

// BindViewer returns the client stub implementing the Viewer
// interface.
//
// If no _gen_ipc.Client is specified, the default _gen_ipc.Client in the
// global Runtime is used.
func BindViewer(name string, opts ..._gen_ipc.BindOpt) (Viewer, error) {
	var client _gen_ipc.Client
	switch len(opts) {
	case 0:
		client = _gen_rt.R().Client()
	case 1:
		switch o := opts[0].(type) {
		case _gen_veyron2.Runtime:
			client = o.Client()
		case _gen_ipc.Client:
			client = o
		default:
			return nil, _gen_vdlutil.ErrUnrecognizedOption
		}
	default:
		return nil, _gen_vdlutil.ErrTooManyOptionsToBind
	}
	stub := &clientStubViewer{client: client, name: name}

	return stub, nil
}

// NewServerViewer creates a new server stub.
//
// It takes a regular server implementing the ViewerService
// interface, and returns a new server stub.
func NewServerViewer(server ViewerService) interface{} {
	return &ServerStubViewer{
		service: server,
	}
}

// clientStubViewer implements Viewer.
type clientStubViewer struct {
	client _gen_ipc.Client
	name   string
}

func (__gen_c *clientStubViewer) Pipe(ctx _gen_context.T, opts ..._gen_ipc.CallOpt) (reply ViewerPipeStream, err error) {
	var call _gen_ipc.Call
	if call, err = __gen_c.client.StartCall(ctx, __gen_c.name, "Pipe", nil, opts...); err != nil {
		return
	}
	reply = &implViewerPipeStream{clientCall: call}
	return
}

func (__gen_c *clientStubViewer) UnresolveStep(ctx _gen_context.T, opts ..._gen_ipc.CallOpt) (reply []string, err error) {
	var call _gen_ipc.Call
	if call, err = __gen_c.client.StartCall(ctx, __gen_c.name, "UnresolveStep", nil, opts...); err != nil {
		return
	}
	if ierr := call.Finish(&reply, &err); ierr != nil {
		err = ierr
	}
	return
}

func (__gen_c *clientStubViewer) Signature(ctx _gen_context.T, opts ..._gen_ipc.CallOpt) (reply _gen_ipc.ServiceSignature, err error) {
	var call _gen_ipc.Call
	if call, err = __gen_c.client.StartCall(ctx, __gen_c.name, "Signature", nil, opts...); err != nil {
		return
	}
	if ierr := call.Finish(&reply, &err); ierr != nil {
		err = ierr
	}
	return
}

func (__gen_c *clientStubViewer) GetMethodTags(ctx _gen_context.T, method string, opts ..._gen_ipc.CallOpt) (reply []interface{}, err error) {
	var call _gen_ipc.Call
	if call, err = __gen_c.client.StartCall(ctx, __gen_c.name, "GetMethodTags", []interface{}{method}, opts...); err != nil {
		return
	}
	if ierr := call.Finish(&reply, &err); ierr != nil {
		err = ierr
	}
	return
}

// ServerStubViewer wraps a server that implements
// ViewerService and provides an object that satisfies
// the requirements of veyron2/ipc.ReflectInvoker.
type ServerStubViewer struct {
	service ViewerService
}

func (__gen_s *ServerStubViewer) GetMethodTags(call _gen_ipc.ServerCall, method string) ([]interface{}, error) {
	// TODO(bprosnitz) GetMethodTags() will be replaces with Signature().
	// Note: This exhibits some weird behavior like returning a nil error if the method isn't found.
	// This will change when it is replaced with Signature().
	switch method {
	case "Pipe":
		return []interface{}{}, nil
	default:
		return nil, nil
	}
}

func (__gen_s *ServerStubViewer) Signature(call _gen_ipc.ServerCall) (_gen_ipc.ServiceSignature, error) {
	result := _gen_ipc.ServiceSignature{Methods: make(map[string]_gen_ipc.MethodSignature)}
	result.Methods["Pipe"] = _gen_ipc.MethodSignature{
		InArgs: []_gen_ipc.MethodArgument{},
		OutArgs: []_gen_ipc.MethodArgument{
			{Name: "", Type: 65},
			{Name: "", Type: 66},
		},
		InStream: 68,
	}

	result.TypeDefs = []_gen_vdlutil.Any{
		_gen_wiretype.NamedPrimitiveType{Type: 0x1, Name: "anydata", Tags: []string(nil)}, _gen_wiretype.NamedPrimitiveType{Type: 0x1, Name: "error", Tags: []string(nil)}, _gen_wiretype.NamedPrimitiveType{Type: 0x32, Name: "byte", Tags: []string(nil)}, _gen_wiretype.SliceType{Elem: 0x43, Name: "", Tags: []string(nil)}}

	return result, nil
}

func (__gen_s *ServerStubViewer) UnresolveStep(call _gen_ipc.ServerCall) (reply []string, err error) {
	if unresolver, ok := __gen_s.service.(_gen_ipc.Unresolver); ok {
		return unresolver.UnresolveStep(call)
	}
	if call.Server() == nil {
		return
	}
	var published []string
	if published, err = call.Server().Published(); err != nil || published == nil {
		return
	}
	reply = make([]string, len(published))
	for i, p := range published {
		reply[i] = _gen_naming.Join(p, call.Name())
	}
	return
}

func (__gen_s *ServerStubViewer) Pipe(call _gen_ipc.ServerCall) (reply _gen_vdlutil.Any, err error) {
	stream := &implViewerServicePipeStream{serverCall: call}
	reply, err = __gen_s.service.Pipe(call, stream)
	return
}
