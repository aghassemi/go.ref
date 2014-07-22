// This file was auto-generated by the veyron vdl tool.
// Source: test_base.vdl

package test_base

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

type Struct struct {
	X int32
	Y int32
}

// TODO(bprosnitz) Remove this line once signatures are updated to use typevals.
// It corrects a bug where _gen_wiretype is unused in VDL pacakges where only bootstrap types are used on interfaces.
const _ = _gen_wiretype.TypeIDInvalid

// TypeTester is the interface the client binds and uses.
// TypeTester_ExcludingUniversal is the interface without internal framework-added methods
// to enable embedding without method collisions.  Not to be used directly by clients.
type TypeTester_ExcludingUniversal interface {
	// Methods to test support for generic types.
	EchoBool(ctx _gen_context.T, I1 bool, opts ..._gen_ipc.CallOpt) (reply bool, err error)
	EchoFloat32(ctx _gen_context.T, I1 float32, opts ..._gen_ipc.CallOpt) (reply float32, err error)
	EchoFloat64(ctx _gen_context.T, I1 float64, opts ..._gen_ipc.CallOpt) (reply float64, err error)
	EchoInt32(ctx _gen_context.T, I1 int32, opts ..._gen_ipc.CallOpt) (reply int32, err error)
	EchoInt64(ctx _gen_context.T, I1 int64, opts ..._gen_ipc.CallOpt) (reply int64, err error)
	EchoString(ctx _gen_context.T, I1 string, opts ..._gen_ipc.CallOpt) (reply string, err error)
	EchoByte(ctx _gen_context.T, I1 byte, opts ..._gen_ipc.CallOpt) (reply byte, err error)
	EchoUInt32(ctx _gen_context.T, I1 uint32, opts ..._gen_ipc.CallOpt) (reply uint32, err error)
	EchoUInt64(ctx _gen_context.T, I1 uint64, opts ..._gen_ipc.CallOpt) (reply uint64, err error)
	// Methods to test support for composite types.
	InputArray(ctx _gen_context.T, I1 [2]byte, opts ..._gen_ipc.CallOpt) (err error)
	InputMap(ctx _gen_context.T, I1 map[byte]byte, opts ..._gen_ipc.CallOpt) (err error)
	InputSlice(ctx _gen_context.T, I1 []byte, opts ..._gen_ipc.CallOpt) (err error)
	InputStruct(ctx _gen_context.T, I1 Struct, opts ..._gen_ipc.CallOpt) (err error)
	OutputArray(ctx _gen_context.T, opts ..._gen_ipc.CallOpt) (reply [2]byte, err error)
	OutputMap(ctx _gen_context.T, opts ..._gen_ipc.CallOpt) (reply map[byte]byte, err error)
	OutputSlice(ctx _gen_context.T, opts ..._gen_ipc.CallOpt) (reply []byte, err error)
	OutputStruct(ctx _gen_context.T, opts ..._gen_ipc.CallOpt) (reply Struct, err error)
	// Methods to test support for different number of arguments.
	NoArguments(ctx _gen_context.T, opts ..._gen_ipc.CallOpt) (err error)
	MultipleArguments(ctx _gen_context.T, I1 int32, I2 int32, opts ..._gen_ipc.CallOpt) (O1 int32, O2 int32, err error)
	// Methods to test support for streaming.
	StreamingOutput(ctx _gen_context.T, NumStreamItems int32, StreamItem bool, opts ..._gen_ipc.CallOpt) (reply TypeTesterStreamingOutputStream, err error)
}
type TypeTester interface {
	_gen_ipc.UniversalServiceMethods
	TypeTester_ExcludingUniversal
}

// TypeTesterService is the interface the server implements.
type TypeTesterService interface {

	// Methods to test support for generic types.
	EchoBool(context _gen_ipc.ServerContext, I1 bool) (reply bool, err error)
	EchoFloat32(context _gen_ipc.ServerContext, I1 float32) (reply float32, err error)
	EchoFloat64(context _gen_ipc.ServerContext, I1 float64) (reply float64, err error)
	EchoInt32(context _gen_ipc.ServerContext, I1 int32) (reply int32, err error)
	EchoInt64(context _gen_ipc.ServerContext, I1 int64) (reply int64, err error)
	EchoString(context _gen_ipc.ServerContext, I1 string) (reply string, err error)
	EchoByte(context _gen_ipc.ServerContext, I1 byte) (reply byte, err error)
	EchoUInt32(context _gen_ipc.ServerContext, I1 uint32) (reply uint32, err error)
	EchoUInt64(context _gen_ipc.ServerContext, I1 uint64) (reply uint64, err error)
	// Methods to test support for composite types.
	InputArray(context _gen_ipc.ServerContext, I1 [2]byte) (err error)
	InputMap(context _gen_ipc.ServerContext, I1 map[byte]byte) (err error)
	InputSlice(context _gen_ipc.ServerContext, I1 []byte) (err error)
	InputStruct(context _gen_ipc.ServerContext, I1 Struct) (err error)
	OutputArray(context _gen_ipc.ServerContext) (reply [2]byte, err error)
	OutputMap(context _gen_ipc.ServerContext) (reply map[byte]byte, err error)
	OutputSlice(context _gen_ipc.ServerContext) (reply []byte, err error)
	OutputStruct(context _gen_ipc.ServerContext) (reply Struct, err error)
	// Methods to test support for different number of arguments.
	NoArguments(context _gen_ipc.ServerContext) (err error)
	MultipleArguments(context _gen_ipc.ServerContext, I1 int32, I2 int32) (O1 int32, O2 int32, err error)
	// Methods to test support for streaming.
	StreamingOutput(context _gen_ipc.ServerContext, NumStreamItems int32, StreamItem bool, stream TypeTesterServiceStreamingOutputStream) (err error)
}

// TypeTesterStreamingOutputStream is the interface for streaming responses of the method
// StreamingOutput in the service interface TypeTester.
type TypeTesterStreamingOutputStream interface {

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
	Value() bool

	// Err returns a non-nil error iff the stream encountered
	// any errors.  Err does not block.
	Err() error

	// Finish blocks until the server is done and returns the positional
	// return values for call.
	//
	// If Cancel has been called, Finish will return immediately; the output of
	// Finish could either be an error signalling cancelation, or the correct
	// positional return values from the server depending on the timing of the
	// call.
	//
	// Calling Finish is mandatory for releasing stream resources, unless Cancel
	// has been called or any of the other methods return a non-EOF error.
	// Finish should be called at most once.
	Finish() (err error)

	// Cancel cancels the RPC, notifying the server to stop processing.  It
	// is safe to call Cancel concurrently with any of the other stream methods.
	// Calling Cancel after Finish has returned is a no-op.
	Cancel()
}

// Implementation of the TypeTesterStreamingOutputStream interface that is not exported.
type implTypeTesterStreamingOutputStream struct {
	clientCall _gen_ipc.Call
	val        bool
	err        error
}

func (c *implTypeTesterStreamingOutputStream) Advance() bool {
	c.err = c.clientCall.Recv(&c.val)
	return c.err == nil
}

func (c *implTypeTesterStreamingOutputStream) Value() bool {
	return c.val
}

func (c *implTypeTesterStreamingOutputStream) Err() error {
	if c.err == _gen_io.EOF {
		return nil
	}
	return c.err
}

func (c *implTypeTesterStreamingOutputStream) Finish() (err error) {
	if ierr := c.clientCall.Finish(&err); ierr != nil {
		err = ierr
	}
	return
}

func (c *implTypeTesterStreamingOutputStream) Cancel() {
	c.clientCall.Cancel()
}

// TypeTesterServiceStreamingOutputStream is the interface for streaming responses of the method
// StreamingOutput in the service interface TypeTester.
type TypeTesterServiceStreamingOutputStream interface {
	// Send places the item onto the output stream, blocking if there is no buffer
	// space available.  If the client has canceled, an error is returned.
	Send(item bool) error
}

// Implementation of the TypeTesterServiceStreamingOutputStream interface that is not exported.
type implTypeTesterServiceStreamingOutputStream struct {
	serverCall _gen_ipc.ServerCall
}

func (s *implTypeTesterServiceStreamingOutputStream) Send(item bool) error {
	return s.serverCall.Send(item)
}

// BindTypeTester returns the client stub implementing the TypeTester
// interface.
//
// If no _gen_ipc.Client is specified, the default _gen_ipc.Client in the
// global Runtime is used.
func BindTypeTester(name string, opts ..._gen_ipc.BindOpt) (TypeTester, error) {
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
	stub := &clientStubTypeTester{client: client, name: name}

	return stub, nil
}

// NewServerTypeTester creates a new server stub.
//
// It takes a regular server implementing the TypeTesterService
// interface, and returns a new server stub.
func NewServerTypeTester(server TypeTesterService) interface{} {
	return &ServerStubTypeTester{
		service: server,
	}
}

// clientStubTypeTester implements TypeTester.
type clientStubTypeTester struct {
	client _gen_ipc.Client
	name   string
}

func (__gen_c *clientStubTypeTester) EchoBool(ctx _gen_context.T, I1 bool, opts ..._gen_ipc.CallOpt) (reply bool, err error) {
	var call _gen_ipc.Call
	if call, err = __gen_c.client.StartCall(ctx, __gen_c.name, "EchoBool", []interface{}{I1}, opts...); err != nil {
		return
	}
	if ierr := call.Finish(&reply, &err); ierr != nil {
		err = ierr
	}
	return
}

func (__gen_c *clientStubTypeTester) EchoFloat32(ctx _gen_context.T, I1 float32, opts ..._gen_ipc.CallOpt) (reply float32, err error) {
	var call _gen_ipc.Call
	if call, err = __gen_c.client.StartCall(ctx, __gen_c.name, "EchoFloat32", []interface{}{I1}, opts...); err != nil {
		return
	}
	if ierr := call.Finish(&reply, &err); ierr != nil {
		err = ierr
	}
	return
}

func (__gen_c *clientStubTypeTester) EchoFloat64(ctx _gen_context.T, I1 float64, opts ..._gen_ipc.CallOpt) (reply float64, err error) {
	var call _gen_ipc.Call
	if call, err = __gen_c.client.StartCall(ctx, __gen_c.name, "EchoFloat64", []interface{}{I1}, opts...); err != nil {
		return
	}
	if ierr := call.Finish(&reply, &err); ierr != nil {
		err = ierr
	}
	return
}

func (__gen_c *clientStubTypeTester) EchoInt32(ctx _gen_context.T, I1 int32, opts ..._gen_ipc.CallOpt) (reply int32, err error) {
	var call _gen_ipc.Call
	if call, err = __gen_c.client.StartCall(ctx, __gen_c.name, "EchoInt32", []interface{}{I1}, opts...); err != nil {
		return
	}
	if ierr := call.Finish(&reply, &err); ierr != nil {
		err = ierr
	}
	return
}

func (__gen_c *clientStubTypeTester) EchoInt64(ctx _gen_context.T, I1 int64, opts ..._gen_ipc.CallOpt) (reply int64, err error) {
	var call _gen_ipc.Call
	if call, err = __gen_c.client.StartCall(ctx, __gen_c.name, "EchoInt64", []interface{}{I1}, opts...); err != nil {
		return
	}
	if ierr := call.Finish(&reply, &err); ierr != nil {
		err = ierr
	}
	return
}

func (__gen_c *clientStubTypeTester) EchoString(ctx _gen_context.T, I1 string, opts ..._gen_ipc.CallOpt) (reply string, err error) {
	var call _gen_ipc.Call
	if call, err = __gen_c.client.StartCall(ctx, __gen_c.name, "EchoString", []interface{}{I1}, opts...); err != nil {
		return
	}
	if ierr := call.Finish(&reply, &err); ierr != nil {
		err = ierr
	}
	return
}

func (__gen_c *clientStubTypeTester) EchoByte(ctx _gen_context.T, I1 byte, opts ..._gen_ipc.CallOpt) (reply byte, err error) {
	var call _gen_ipc.Call
	if call, err = __gen_c.client.StartCall(ctx, __gen_c.name, "EchoByte", []interface{}{I1}, opts...); err != nil {
		return
	}
	if ierr := call.Finish(&reply, &err); ierr != nil {
		err = ierr
	}
	return
}

func (__gen_c *clientStubTypeTester) EchoUInt32(ctx _gen_context.T, I1 uint32, opts ..._gen_ipc.CallOpt) (reply uint32, err error) {
	var call _gen_ipc.Call
	if call, err = __gen_c.client.StartCall(ctx, __gen_c.name, "EchoUInt32", []interface{}{I1}, opts...); err != nil {
		return
	}
	if ierr := call.Finish(&reply, &err); ierr != nil {
		err = ierr
	}
	return
}

func (__gen_c *clientStubTypeTester) EchoUInt64(ctx _gen_context.T, I1 uint64, opts ..._gen_ipc.CallOpt) (reply uint64, err error) {
	var call _gen_ipc.Call
	if call, err = __gen_c.client.StartCall(ctx, __gen_c.name, "EchoUInt64", []interface{}{I1}, opts...); err != nil {
		return
	}
	if ierr := call.Finish(&reply, &err); ierr != nil {
		err = ierr
	}
	return
}

func (__gen_c *clientStubTypeTester) InputArray(ctx _gen_context.T, I1 [2]byte, opts ..._gen_ipc.CallOpt) (err error) {
	var call _gen_ipc.Call
	if call, err = __gen_c.client.StartCall(ctx, __gen_c.name, "InputArray", []interface{}{I1}, opts...); err != nil {
		return
	}
	if ierr := call.Finish(&err); ierr != nil {
		err = ierr
	}
	return
}

func (__gen_c *clientStubTypeTester) InputMap(ctx _gen_context.T, I1 map[byte]byte, opts ..._gen_ipc.CallOpt) (err error) {
	var call _gen_ipc.Call
	if call, err = __gen_c.client.StartCall(ctx, __gen_c.name, "InputMap", []interface{}{I1}, opts...); err != nil {
		return
	}
	if ierr := call.Finish(&err); ierr != nil {
		err = ierr
	}
	return
}

func (__gen_c *clientStubTypeTester) InputSlice(ctx _gen_context.T, I1 []byte, opts ..._gen_ipc.CallOpt) (err error) {
	var call _gen_ipc.Call
	if call, err = __gen_c.client.StartCall(ctx, __gen_c.name, "InputSlice", []interface{}{I1}, opts...); err != nil {
		return
	}
	if ierr := call.Finish(&err); ierr != nil {
		err = ierr
	}
	return
}

func (__gen_c *clientStubTypeTester) InputStruct(ctx _gen_context.T, I1 Struct, opts ..._gen_ipc.CallOpt) (err error) {
	var call _gen_ipc.Call
	if call, err = __gen_c.client.StartCall(ctx, __gen_c.name, "InputStruct", []interface{}{I1}, opts...); err != nil {
		return
	}
	if ierr := call.Finish(&err); ierr != nil {
		err = ierr
	}
	return
}

func (__gen_c *clientStubTypeTester) OutputArray(ctx _gen_context.T, opts ..._gen_ipc.CallOpt) (reply [2]byte, err error) {
	var call _gen_ipc.Call
	if call, err = __gen_c.client.StartCall(ctx, __gen_c.name, "OutputArray", nil, opts...); err != nil {
		return
	}
	if ierr := call.Finish(&reply, &err); ierr != nil {
		err = ierr
	}
	return
}

func (__gen_c *clientStubTypeTester) OutputMap(ctx _gen_context.T, opts ..._gen_ipc.CallOpt) (reply map[byte]byte, err error) {
	var call _gen_ipc.Call
	if call, err = __gen_c.client.StartCall(ctx, __gen_c.name, "OutputMap", nil, opts...); err != nil {
		return
	}
	if ierr := call.Finish(&reply, &err); ierr != nil {
		err = ierr
	}
	return
}

func (__gen_c *clientStubTypeTester) OutputSlice(ctx _gen_context.T, opts ..._gen_ipc.CallOpt) (reply []byte, err error) {
	var call _gen_ipc.Call
	if call, err = __gen_c.client.StartCall(ctx, __gen_c.name, "OutputSlice", nil, opts...); err != nil {
		return
	}
	if ierr := call.Finish(&reply, &err); ierr != nil {
		err = ierr
	}
	return
}

func (__gen_c *clientStubTypeTester) OutputStruct(ctx _gen_context.T, opts ..._gen_ipc.CallOpt) (reply Struct, err error) {
	var call _gen_ipc.Call
	if call, err = __gen_c.client.StartCall(ctx, __gen_c.name, "OutputStruct", nil, opts...); err != nil {
		return
	}
	if ierr := call.Finish(&reply, &err); ierr != nil {
		err = ierr
	}
	return
}

func (__gen_c *clientStubTypeTester) NoArguments(ctx _gen_context.T, opts ..._gen_ipc.CallOpt) (err error) {
	var call _gen_ipc.Call
	if call, err = __gen_c.client.StartCall(ctx, __gen_c.name, "NoArguments", nil, opts...); err != nil {
		return
	}
	if ierr := call.Finish(&err); ierr != nil {
		err = ierr
	}
	return
}

func (__gen_c *clientStubTypeTester) MultipleArguments(ctx _gen_context.T, I1 int32, I2 int32, opts ..._gen_ipc.CallOpt) (O1 int32, O2 int32, err error) {
	var call _gen_ipc.Call
	if call, err = __gen_c.client.StartCall(ctx, __gen_c.name, "MultipleArguments", []interface{}{I1, I2}, opts...); err != nil {
		return
	}
	if ierr := call.Finish(&O1, &O2, &err); ierr != nil {
		err = ierr
	}
	return
}

func (__gen_c *clientStubTypeTester) StreamingOutput(ctx _gen_context.T, NumStreamItems int32, StreamItem bool, opts ..._gen_ipc.CallOpt) (reply TypeTesterStreamingOutputStream, err error) {
	var call _gen_ipc.Call
	if call, err = __gen_c.client.StartCall(ctx, __gen_c.name, "StreamingOutput", []interface{}{NumStreamItems, StreamItem}, opts...); err != nil {
		return
	}
	reply = &implTypeTesterStreamingOutputStream{clientCall: call}
	return
}

func (__gen_c *clientStubTypeTester) UnresolveStep(ctx _gen_context.T, opts ..._gen_ipc.CallOpt) (reply []string, err error) {
	var call _gen_ipc.Call
	if call, err = __gen_c.client.StartCall(ctx, __gen_c.name, "UnresolveStep", nil, opts...); err != nil {
		return
	}
	if ierr := call.Finish(&reply, &err); ierr != nil {
		err = ierr
	}
	return
}

func (__gen_c *clientStubTypeTester) Signature(ctx _gen_context.T, opts ..._gen_ipc.CallOpt) (reply _gen_ipc.ServiceSignature, err error) {
	var call _gen_ipc.Call
	if call, err = __gen_c.client.StartCall(ctx, __gen_c.name, "Signature", nil, opts...); err != nil {
		return
	}
	if ierr := call.Finish(&reply, &err); ierr != nil {
		err = ierr
	}
	return
}

func (__gen_c *clientStubTypeTester) GetMethodTags(ctx _gen_context.T, method string, opts ..._gen_ipc.CallOpt) (reply []interface{}, err error) {
	var call _gen_ipc.Call
	if call, err = __gen_c.client.StartCall(ctx, __gen_c.name, "GetMethodTags", []interface{}{method}, opts...); err != nil {
		return
	}
	if ierr := call.Finish(&reply, &err); ierr != nil {
		err = ierr
	}
	return
}

// ServerStubTypeTester wraps a server that implements
// TypeTesterService and provides an object that satisfies
// the requirements of veyron2/ipc.ReflectInvoker.
type ServerStubTypeTester struct {
	service TypeTesterService
}

func (__gen_s *ServerStubTypeTester) GetMethodTags(call _gen_ipc.ServerCall, method string) ([]interface{}, error) {
	// TODO(bprosnitz) GetMethodTags() will be replaces with Signature().
	// Note: This exhibits some weird behavior like returning a nil error if the method isn't found.
	// This will change when it is replaced with Signature().
	switch method {
	case "EchoBool":
		return []interface{}{}, nil
	case "EchoFloat32":
		return []interface{}{}, nil
	case "EchoFloat64":
		return []interface{}{}, nil
	case "EchoInt32":
		return []interface{}{}, nil
	case "EchoInt64":
		return []interface{}{}, nil
	case "EchoString":
		return []interface{}{}, nil
	case "EchoByte":
		return []interface{}{}, nil
	case "EchoUInt32":
		return []interface{}{}, nil
	case "EchoUInt64":
		return []interface{}{}, nil
	case "InputArray":
		return []interface{}{}, nil
	case "InputMap":
		return []interface{}{}, nil
	case "InputSlice":
		return []interface{}{}, nil
	case "InputStruct":
		return []interface{}{}, nil
	case "OutputArray":
		return []interface{}{}, nil
	case "OutputMap":
		return []interface{}{}, nil
	case "OutputSlice":
		return []interface{}{}, nil
	case "OutputStruct":
		return []interface{}{}, nil
	case "NoArguments":
		return []interface{}{}, nil
	case "MultipleArguments":
		return []interface{}{}, nil
	case "StreamingOutput":
		return []interface{}{}, nil
	default:
		return nil, nil
	}
}

func (__gen_s *ServerStubTypeTester) Signature(call _gen_ipc.ServerCall) (_gen_ipc.ServiceSignature, error) {
	result := _gen_ipc.ServiceSignature{Methods: make(map[string]_gen_ipc.MethodSignature)}
	result.Methods["EchoBool"] = _gen_ipc.MethodSignature{
		InArgs: []_gen_ipc.MethodArgument{
			{Name: "I1", Type: 2},
		},
		OutArgs: []_gen_ipc.MethodArgument{
			{Name: "O1", Type: 2},
			{Name: "E", Type: 65},
		},
	}
	result.Methods["EchoByte"] = _gen_ipc.MethodSignature{
		InArgs: []_gen_ipc.MethodArgument{
			{Name: "I1", Type: 66},
		},
		OutArgs: []_gen_ipc.MethodArgument{
			{Name: "O1", Type: 66},
			{Name: "E", Type: 65},
		},
	}
	result.Methods["EchoFloat32"] = _gen_ipc.MethodSignature{
		InArgs: []_gen_ipc.MethodArgument{
			{Name: "I1", Type: 25},
		},
		OutArgs: []_gen_ipc.MethodArgument{
			{Name: "O1", Type: 25},
			{Name: "E", Type: 65},
		},
	}
	result.Methods["EchoFloat64"] = _gen_ipc.MethodSignature{
		InArgs: []_gen_ipc.MethodArgument{
			{Name: "I1", Type: 26},
		},
		OutArgs: []_gen_ipc.MethodArgument{
			{Name: "O1", Type: 26},
			{Name: "E", Type: 65},
		},
	}
	result.Methods["EchoInt32"] = _gen_ipc.MethodSignature{
		InArgs: []_gen_ipc.MethodArgument{
			{Name: "I1", Type: 36},
		},
		OutArgs: []_gen_ipc.MethodArgument{
			{Name: "O1", Type: 36},
			{Name: "E", Type: 65},
		},
	}
	result.Methods["EchoInt64"] = _gen_ipc.MethodSignature{
		InArgs: []_gen_ipc.MethodArgument{
			{Name: "I1", Type: 37},
		},
		OutArgs: []_gen_ipc.MethodArgument{
			{Name: "O1", Type: 37},
			{Name: "E", Type: 65},
		},
	}
	result.Methods["EchoString"] = _gen_ipc.MethodSignature{
		InArgs: []_gen_ipc.MethodArgument{
			{Name: "I1", Type: 3},
		},
		OutArgs: []_gen_ipc.MethodArgument{
			{Name: "O1", Type: 3},
			{Name: "E", Type: 65},
		},
	}
	result.Methods["EchoUInt32"] = _gen_ipc.MethodSignature{
		InArgs: []_gen_ipc.MethodArgument{
			{Name: "I1", Type: 52},
		},
		OutArgs: []_gen_ipc.MethodArgument{
			{Name: "O1", Type: 52},
			{Name: "E", Type: 65},
		},
	}
	result.Methods["EchoUInt64"] = _gen_ipc.MethodSignature{
		InArgs: []_gen_ipc.MethodArgument{
			{Name: "I1", Type: 53},
		},
		OutArgs: []_gen_ipc.MethodArgument{
			{Name: "O1", Type: 53},
			{Name: "E", Type: 65},
		},
	}
	result.Methods["InputArray"] = _gen_ipc.MethodSignature{
		InArgs: []_gen_ipc.MethodArgument{
			{Name: "I1", Type: 67},
		},
		OutArgs: []_gen_ipc.MethodArgument{
			{Name: "E", Type: 65},
		},
	}
	result.Methods["InputMap"] = _gen_ipc.MethodSignature{
		InArgs: []_gen_ipc.MethodArgument{
			{Name: "I1", Type: 68},
		},
		OutArgs: []_gen_ipc.MethodArgument{
			{Name: "E", Type: 65},
		},
	}
	result.Methods["InputSlice"] = _gen_ipc.MethodSignature{
		InArgs: []_gen_ipc.MethodArgument{
			{Name: "I1", Type: 69},
		},
		OutArgs: []_gen_ipc.MethodArgument{
			{Name: "E", Type: 65},
		},
	}
	result.Methods["InputStruct"] = _gen_ipc.MethodSignature{
		InArgs: []_gen_ipc.MethodArgument{
			{Name: "I1", Type: 70},
		},
		OutArgs: []_gen_ipc.MethodArgument{
			{Name: "E", Type: 65},
		},
	}
	result.Methods["MultipleArguments"] = _gen_ipc.MethodSignature{
		InArgs: []_gen_ipc.MethodArgument{
			{Name: "I1", Type: 36},
			{Name: "I2", Type: 36},
		},
		OutArgs: []_gen_ipc.MethodArgument{
			{Name: "O1", Type: 36},
			{Name: "O2", Type: 36},
			{Name: "E", Type: 65},
		},
	}
	result.Methods["NoArguments"] = _gen_ipc.MethodSignature{
		InArgs: []_gen_ipc.MethodArgument{},
		OutArgs: []_gen_ipc.MethodArgument{
			{Name: "", Type: 65},
		},
	}
	result.Methods["OutputArray"] = _gen_ipc.MethodSignature{
		InArgs: []_gen_ipc.MethodArgument{},
		OutArgs: []_gen_ipc.MethodArgument{
			{Name: "O1", Type: 67},
			{Name: "E", Type: 65},
		},
	}
	result.Methods["OutputMap"] = _gen_ipc.MethodSignature{
		InArgs: []_gen_ipc.MethodArgument{},
		OutArgs: []_gen_ipc.MethodArgument{
			{Name: "O1", Type: 68},
			{Name: "E", Type: 65},
		},
	}
	result.Methods["OutputSlice"] = _gen_ipc.MethodSignature{
		InArgs: []_gen_ipc.MethodArgument{},
		OutArgs: []_gen_ipc.MethodArgument{
			{Name: "O1", Type: 69},
			{Name: "E", Type: 65},
		},
	}
	result.Methods["OutputStruct"] = _gen_ipc.MethodSignature{
		InArgs: []_gen_ipc.MethodArgument{},
		OutArgs: []_gen_ipc.MethodArgument{
			{Name: "O1", Type: 70},
			{Name: "E", Type: 65},
		},
	}
	result.Methods["StreamingOutput"] = _gen_ipc.MethodSignature{
		InArgs: []_gen_ipc.MethodArgument{
			{Name: "NumStreamItems", Type: 36},
			{Name: "StreamItem", Type: 2},
		},
		OutArgs: []_gen_ipc.MethodArgument{
			{Name: "", Type: 65},
		},

		OutStream: 2,
	}

	result.TypeDefs = []_gen_vdlutil.Any{
		_gen_wiretype.NamedPrimitiveType{Type: 0x1, Name: "error", Tags: []string(nil)}, _gen_wiretype.NamedPrimitiveType{Type: 0x32, Name: "byte", Tags: []string(nil)}, _gen_wiretype.ArrayType{Elem: 0x42, Len: 0x2, Name: "", Tags: []string(nil)}, _gen_wiretype.MapType{Key: 0x42, Elem: 0x42, Name: "", Tags: []string(nil)}, _gen_wiretype.SliceType{Elem: 0x42, Name: "", Tags: []string(nil)}, _gen_wiretype.StructType{
			[]_gen_wiretype.FieldType{
				_gen_wiretype.FieldType{Type: 0x24, Name: "X"},
				_gen_wiretype.FieldType{Type: 0x24, Name: "Y"},
			},
			"veyron/tools/vrpc/test_base.Struct", []string(nil)},
	}

	return result, nil
}

func (__gen_s *ServerStubTypeTester) UnresolveStep(call _gen_ipc.ServerCall) (reply []string, err error) {
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

func (__gen_s *ServerStubTypeTester) EchoBool(call _gen_ipc.ServerCall, I1 bool) (reply bool, err error) {
	reply, err = __gen_s.service.EchoBool(call, I1)
	return
}

func (__gen_s *ServerStubTypeTester) EchoFloat32(call _gen_ipc.ServerCall, I1 float32) (reply float32, err error) {
	reply, err = __gen_s.service.EchoFloat32(call, I1)
	return
}

func (__gen_s *ServerStubTypeTester) EchoFloat64(call _gen_ipc.ServerCall, I1 float64) (reply float64, err error) {
	reply, err = __gen_s.service.EchoFloat64(call, I1)
	return
}

func (__gen_s *ServerStubTypeTester) EchoInt32(call _gen_ipc.ServerCall, I1 int32) (reply int32, err error) {
	reply, err = __gen_s.service.EchoInt32(call, I1)
	return
}

func (__gen_s *ServerStubTypeTester) EchoInt64(call _gen_ipc.ServerCall, I1 int64) (reply int64, err error) {
	reply, err = __gen_s.service.EchoInt64(call, I1)
	return
}

func (__gen_s *ServerStubTypeTester) EchoString(call _gen_ipc.ServerCall, I1 string) (reply string, err error) {
	reply, err = __gen_s.service.EchoString(call, I1)
	return
}

func (__gen_s *ServerStubTypeTester) EchoByte(call _gen_ipc.ServerCall, I1 byte) (reply byte, err error) {
	reply, err = __gen_s.service.EchoByte(call, I1)
	return
}

func (__gen_s *ServerStubTypeTester) EchoUInt32(call _gen_ipc.ServerCall, I1 uint32) (reply uint32, err error) {
	reply, err = __gen_s.service.EchoUInt32(call, I1)
	return
}

func (__gen_s *ServerStubTypeTester) EchoUInt64(call _gen_ipc.ServerCall, I1 uint64) (reply uint64, err error) {
	reply, err = __gen_s.service.EchoUInt64(call, I1)
	return
}

func (__gen_s *ServerStubTypeTester) InputArray(call _gen_ipc.ServerCall, I1 [2]byte) (err error) {
	err = __gen_s.service.InputArray(call, I1)
	return
}

func (__gen_s *ServerStubTypeTester) InputMap(call _gen_ipc.ServerCall, I1 map[byte]byte) (err error) {
	err = __gen_s.service.InputMap(call, I1)
	return
}

func (__gen_s *ServerStubTypeTester) InputSlice(call _gen_ipc.ServerCall, I1 []byte) (err error) {
	err = __gen_s.service.InputSlice(call, I1)
	return
}

func (__gen_s *ServerStubTypeTester) InputStruct(call _gen_ipc.ServerCall, I1 Struct) (err error) {
	err = __gen_s.service.InputStruct(call, I1)
	return
}

func (__gen_s *ServerStubTypeTester) OutputArray(call _gen_ipc.ServerCall) (reply [2]byte, err error) {
	reply, err = __gen_s.service.OutputArray(call)
	return
}

func (__gen_s *ServerStubTypeTester) OutputMap(call _gen_ipc.ServerCall) (reply map[byte]byte, err error) {
	reply, err = __gen_s.service.OutputMap(call)
	return
}

func (__gen_s *ServerStubTypeTester) OutputSlice(call _gen_ipc.ServerCall) (reply []byte, err error) {
	reply, err = __gen_s.service.OutputSlice(call)
	return
}

func (__gen_s *ServerStubTypeTester) OutputStruct(call _gen_ipc.ServerCall) (reply Struct, err error) {
	reply, err = __gen_s.service.OutputStruct(call)
	return
}

func (__gen_s *ServerStubTypeTester) NoArguments(call _gen_ipc.ServerCall) (err error) {
	err = __gen_s.service.NoArguments(call)
	return
}

func (__gen_s *ServerStubTypeTester) MultipleArguments(call _gen_ipc.ServerCall, I1 int32, I2 int32) (O1 int32, O2 int32, err error) {
	O1, O2, err = __gen_s.service.MultipleArguments(call, I1, I2)
	return
}

func (__gen_s *ServerStubTypeTester) StreamingOutput(call _gen_ipc.ServerCall, NumStreamItems int32, StreamItem bool) (err error) {
	stream := &implTypeTesterServiceStreamingOutputStream{serverCall: call}
	err = __gen_s.service.StreamingOutput(call, NumStreamItems, StreamItem, stream)
	return
}
