// This file was auto-generated by the veyron vdl tool.
// Source: node.vdl

// Package node contains implementation of managing a node and
// applications running on the node.
package node

import (
	"veyron2/services/mgmt/node"

	// The non-user imports are prefixed with "_gen_" to prevent collisions.
	_gen_veyron2 "veyron2"
	_gen_context "veyron2/context"
	_gen_ipc "veyron2/ipc"
	_gen_naming "veyron2/naming"
	_gen_rt "veyron2/rt"
	_gen_vdl "veyron2/vdl"
	_gen_wiretype "veyron2/wiretype"
)

// CallbackReceiver can receive callbacks from previously spawned
// processes.
// CallbackReceiver is the interface the client binds and uses.
// CallbackReceiver_ExcludingUniversal is the interface without internal framework-added methods
// to enable embedding without method collisions.  Not to be used directly by clients.
type CallbackReceiver_ExcludingUniversal interface {
	// Callback receives a callback from a process that the callee
	// previously spawned, providing the callee with a name that can be
	// used to communicate with the caller.
	Callback(ctx _gen_context.T, name string, opts ..._gen_ipc.CallOpt) (err error)
}
type CallbackReceiver interface {
	_gen_ipc.UniversalServiceMethods
	CallbackReceiver_ExcludingUniversal
}

// CallbackReceiverService is the interface the server implements.
type CallbackReceiverService interface {

	// Callback receives a callback from a process that the callee
	// previously spawned, providing the callee with a name that can be
	// used to communicate with the caller.
	Callback(context _gen_ipc.ServerContext, name string) (err error)
}

// BindCallbackReceiver returns the client stub implementing the CallbackReceiver
// interface.
//
// If no _gen_ipc.Client is specified, the default _gen_ipc.Client in the
// global Runtime is used.
func BindCallbackReceiver(name string, opts ..._gen_ipc.BindOpt) (CallbackReceiver, error) {
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
			return nil, _gen_vdl.ErrUnrecognizedOption
		}
	default:
		return nil, _gen_vdl.ErrTooManyOptionsToBind
	}
	stub := &clientStubCallbackReceiver{client: client, name: name}

	return stub, nil
}

// NewServerCallbackReceiver creates a new server stub.
//
// It takes a regular server implementing the CallbackReceiverService
// interface, and returns a new server stub.
func NewServerCallbackReceiver(server CallbackReceiverService) interface{} {
	return &ServerStubCallbackReceiver{
		service: server,
	}
}

// clientStubCallbackReceiver implements CallbackReceiver.
type clientStubCallbackReceiver struct {
	client _gen_ipc.Client
	name   string
}

func (__gen_c *clientStubCallbackReceiver) Callback(ctx _gen_context.T, name string, opts ..._gen_ipc.CallOpt) (err error) {
	var call _gen_ipc.Call
	if call, err = __gen_c.client.StartCall(ctx, __gen_c.name, "Callback", []interface{}{name}, opts...); err != nil {
		return
	}
	if ierr := call.Finish(&err); ierr != nil {
		err = ierr
	}
	return
}

func (__gen_c *clientStubCallbackReceiver) UnresolveStep(ctx _gen_context.T, opts ..._gen_ipc.CallOpt) (reply []string, err error) {
	var call _gen_ipc.Call
	if call, err = __gen_c.client.StartCall(ctx, __gen_c.name, "UnresolveStep", nil, opts...); err != nil {
		return
	}
	if ierr := call.Finish(&reply, &err); ierr != nil {
		err = ierr
	}
	return
}

func (__gen_c *clientStubCallbackReceiver) Signature(ctx _gen_context.T, opts ..._gen_ipc.CallOpt) (reply _gen_ipc.ServiceSignature, err error) {
	var call _gen_ipc.Call
	if call, err = __gen_c.client.StartCall(ctx, __gen_c.name, "Signature", nil, opts...); err != nil {
		return
	}
	if ierr := call.Finish(&reply, &err); ierr != nil {
		err = ierr
	}
	return
}

func (__gen_c *clientStubCallbackReceiver) GetMethodTags(ctx _gen_context.T, method string, opts ..._gen_ipc.CallOpt) (reply []interface{}, err error) {
	var call _gen_ipc.Call
	if call, err = __gen_c.client.StartCall(ctx, __gen_c.name, "GetMethodTags", []interface{}{method}, opts...); err != nil {
		return
	}
	if ierr := call.Finish(&reply, &err); ierr != nil {
		err = ierr
	}
	return
}

// ServerStubCallbackReceiver wraps a server that implements
// CallbackReceiverService and provides an object that satisfies
// the requirements of veyron2/ipc.ReflectInvoker.
type ServerStubCallbackReceiver struct {
	service CallbackReceiverService
}

func (__gen_s *ServerStubCallbackReceiver) GetMethodTags(call _gen_ipc.ServerCall, method string) ([]interface{}, error) {
	// TODO(bprosnitz) GetMethodTags() will be replaces with Signature().
	// Note: This exhibits some weird behavior like returning a nil error if the method isn't found.
	// This will change when it is replaced with Signature().
	switch method {
	case "Callback":
		return []interface{}{}, nil
	default:
		return nil, nil
	}
}

func (__gen_s *ServerStubCallbackReceiver) Signature(call _gen_ipc.ServerCall) (_gen_ipc.ServiceSignature, error) {
	result := _gen_ipc.ServiceSignature{Methods: make(map[string]_gen_ipc.MethodSignature)}
	result.Methods["Callback"] = _gen_ipc.MethodSignature{
		InArgs: []_gen_ipc.MethodArgument{
			{Name: "name", Type: 3},
		},
		OutArgs: []_gen_ipc.MethodArgument{
			{Name: "", Type: 65},
		},
	}

	result.TypeDefs = []_gen_vdl.Any{
		_gen_wiretype.NamedPrimitiveType{Type: 0x1, Name: "error", Tags: []string(nil)}}

	return result, nil
}

func (__gen_s *ServerStubCallbackReceiver) UnresolveStep(call _gen_ipc.ServerCall) (reply []string, err error) {
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

func (__gen_s *ServerStubCallbackReceiver) Callback(call _gen_ipc.ServerCall, name string) (err error) {
	err = __gen_s.service.Callback(call, name)
	return
}

// Node describes a node manager internally. In addition to the public
// Node interface, it implements the callback functionality.
// Node is the interface the client binds and uses.
// Node_ExcludingUniversal is the interface without internal framework-added methods
// to enable embedding without method collisions.  Not to be used directly by clients.
type Node_ExcludingUniversal interface {
	// Node can be used to manage a node. The idea is that this interace
	// will be invoked using a veyron name that identifies the node.
	node.Node_ExcludingUniversal
	// CallbackReceiver can receive callbacks from previously spawned
	// processes.
	CallbackReceiver_ExcludingUniversal
}
type Node interface {
	_gen_ipc.UniversalServiceMethods
	Node_ExcludingUniversal
}

// NodeService is the interface the server implements.
type NodeService interface {

	// Node can be used to manage a node. The idea is that this interace
	// will be invoked using a veyron name that identifies the node.
	node.NodeService
	// CallbackReceiver can receive callbacks from previously spawned
	// processes.
	CallbackReceiverService
}

// BindNode returns the client stub implementing the Node
// interface.
//
// If no _gen_ipc.Client is specified, the default _gen_ipc.Client in the
// global Runtime is used.
func BindNode(name string, opts ..._gen_ipc.BindOpt) (Node, error) {
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
			return nil, _gen_vdl.ErrUnrecognizedOption
		}
	default:
		return nil, _gen_vdl.ErrTooManyOptionsToBind
	}
	stub := &clientStubNode{client: client, name: name}
	stub.Node_ExcludingUniversal, _ = node.BindNode(name, client)
	stub.CallbackReceiver_ExcludingUniversal, _ = BindCallbackReceiver(name, client)

	return stub, nil
}

// NewServerNode creates a new server stub.
//
// It takes a regular server implementing the NodeService
// interface, and returns a new server stub.
func NewServerNode(server NodeService) interface{} {
	return &ServerStubNode{
		ServerStubNode:             *node.NewServerNode(server).(*node.ServerStubNode),
		ServerStubCallbackReceiver: *NewServerCallbackReceiver(server).(*ServerStubCallbackReceiver),
		service:                    server,
	}
}

// clientStubNode implements Node.
type clientStubNode struct {
	node.Node_ExcludingUniversal
	CallbackReceiver_ExcludingUniversal

	client _gen_ipc.Client
	name   string
}

func (__gen_c *clientStubNode) UnresolveStep(ctx _gen_context.T, opts ..._gen_ipc.CallOpt) (reply []string, err error) {
	var call _gen_ipc.Call
	if call, err = __gen_c.client.StartCall(ctx, __gen_c.name, "UnresolveStep", nil, opts...); err != nil {
		return
	}
	if ierr := call.Finish(&reply, &err); ierr != nil {
		err = ierr
	}
	return
}

func (__gen_c *clientStubNode) Signature(ctx _gen_context.T, opts ..._gen_ipc.CallOpt) (reply _gen_ipc.ServiceSignature, err error) {
	var call _gen_ipc.Call
	if call, err = __gen_c.client.StartCall(ctx, __gen_c.name, "Signature", nil, opts...); err != nil {
		return
	}
	if ierr := call.Finish(&reply, &err); ierr != nil {
		err = ierr
	}
	return
}

func (__gen_c *clientStubNode) GetMethodTags(ctx _gen_context.T, method string, opts ..._gen_ipc.CallOpt) (reply []interface{}, err error) {
	var call _gen_ipc.Call
	if call, err = __gen_c.client.StartCall(ctx, __gen_c.name, "GetMethodTags", []interface{}{method}, opts...); err != nil {
		return
	}
	if ierr := call.Finish(&reply, &err); ierr != nil {
		err = ierr
	}
	return
}

// ServerStubNode wraps a server that implements
// NodeService and provides an object that satisfies
// the requirements of veyron2/ipc.ReflectInvoker.
type ServerStubNode struct {
	node.ServerStubNode
	ServerStubCallbackReceiver

	service NodeService
}

func (__gen_s *ServerStubNode) GetMethodTags(call _gen_ipc.ServerCall, method string) ([]interface{}, error) {
	// TODO(bprosnitz) GetMethodTags() will be replaces with Signature().
	// Note: This exhibits some weird behavior like returning a nil error if the method isn't found.
	// This will change when it is replaced with Signature().
	if resp, err := __gen_s.ServerStubNode.GetMethodTags(call, method); resp != nil || err != nil {
		return resp, err
	}
	if resp, err := __gen_s.ServerStubCallbackReceiver.GetMethodTags(call, method); resp != nil || err != nil {
		return resp, err
	}
	return nil, nil
}

func (__gen_s *ServerStubNode) Signature(call _gen_ipc.ServerCall) (_gen_ipc.ServiceSignature, error) {
	result := _gen_ipc.ServiceSignature{Methods: make(map[string]_gen_ipc.MethodSignature)}

	result.TypeDefs = []_gen_vdl.Any{}
	var ss _gen_ipc.ServiceSignature
	var firstAdded int
	ss, _ = __gen_s.ServerStubNode.Signature(call)
	firstAdded = len(result.TypeDefs)
	for k, v := range ss.Methods {
		for i, _ := range v.InArgs {
			if v.InArgs[i].Type >= _gen_wiretype.TypeIDFirst {
				v.InArgs[i].Type += _gen_wiretype.TypeID(firstAdded)
			}
		}
		for i, _ := range v.OutArgs {
			if v.OutArgs[i].Type >= _gen_wiretype.TypeIDFirst {
				v.OutArgs[i].Type += _gen_wiretype.TypeID(firstAdded)
			}
		}
		if v.InStream >= _gen_wiretype.TypeIDFirst {
			v.InStream += _gen_wiretype.TypeID(firstAdded)
		}
		if v.OutStream >= _gen_wiretype.TypeIDFirst {
			v.OutStream += _gen_wiretype.TypeID(firstAdded)
		}
		result.Methods[k] = v
	}
	//TODO(bprosnitz) combine type definitions from embeded interfaces in a way that doesn't cause duplication.
	for _, d := range ss.TypeDefs {
		switch wt := d.(type) {
		case _gen_wiretype.SliceType:
			if wt.Elem >= _gen_wiretype.TypeIDFirst {
				wt.Elem += _gen_wiretype.TypeID(firstAdded)
			}
			d = wt
		case _gen_wiretype.ArrayType:
			if wt.Elem >= _gen_wiretype.TypeIDFirst {
				wt.Elem += _gen_wiretype.TypeID(firstAdded)
			}
			d = wt
		case _gen_wiretype.MapType:
			if wt.Key >= _gen_wiretype.TypeIDFirst {
				wt.Key += _gen_wiretype.TypeID(firstAdded)
			}
			if wt.Elem >= _gen_wiretype.TypeIDFirst {
				wt.Elem += _gen_wiretype.TypeID(firstAdded)
			}
			d = wt
		case _gen_wiretype.StructType:
			for _, fld := range wt.Fields {
				if fld.Type >= _gen_wiretype.TypeIDFirst {
					fld.Type += _gen_wiretype.TypeID(firstAdded)
				}
			}
			d = wt
		}
		result.TypeDefs = append(result.TypeDefs, d)
	}
	ss, _ = __gen_s.ServerStubCallbackReceiver.Signature(call)
	firstAdded = len(result.TypeDefs)
	for k, v := range ss.Methods {
		for i, _ := range v.InArgs {
			if v.InArgs[i].Type >= _gen_wiretype.TypeIDFirst {
				v.InArgs[i].Type += _gen_wiretype.TypeID(firstAdded)
			}
		}
		for i, _ := range v.OutArgs {
			if v.OutArgs[i].Type >= _gen_wiretype.TypeIDFirst {
				v.OutArgs[i].Type += _gen_wiretype.TypeID(firstAdded)
			}
		}
		if v.InStream >= _gen_wiretype.TypeIDFirst {
			v.InStream += _gen_wiretype.TypeID(firstAdded)
		}
		if v.OutStream >= _gen_wiretype.TypeIDFirst {
			v.OutStream += _gen_wiretype.TypeID(firstAdded)
		}
		result.Methods[k] = v
	}
	//TODO(bprosnitz) combine type definitions from embeded interfaces in a way that doesn't cause duplication.
	for _, d := range ss.TypeDefs {
		switch wt := d.(type) {
		case _gen_wiretype.SliceType:
			if wt.Elem >= _gen_wiretype.TypeIDFirst {
				wt.Elem += _gen_wiretype.TypeID(firstAdded)
			}
			d = wt
		case _gen_wiretype.ArrayType:
			if wt.Elem >= _gen_wiretype.TypeIDFirst {
				wt.Elem += _gen_wiretype.TypeID(firstAdded)
			}
			d = wt
		case _gen_wiretype.MapType:
			if wt.Key >= _gen_wiretype.TypeIDFirst {
				wt.Key += _gen_wiretype.TypeID(firstAdded)
			}
			if wt.Elem >= _gen_wiretype.TypeIDFirst {
				wt.Elem += _gen_wiretype.TypeID(firstAdded)
			}
			d = wt
		case _gen_wiretype.StructType:
			for _, fld := range wt.Fields {
				if fld.Type >= _gen_wiretype.TypeIDFirst {
					fld.Type += _gen_wiretype.TypeID(firstAdded)
				}
			}
			d = wt
		}
		result.TypeDefs = append(result.TypeDefs, d)
	}

	return result, nil
}

func (__gen_s *ServerStubNode) UnresolveStep(call _gen_ipc.ServerCall) (reply []string, err error) {
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
