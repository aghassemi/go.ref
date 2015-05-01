// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The app package contains the struct that keeps per javascript app state and handles translating
// javascript requests to vanadium requests and vice versa.
package app

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"reflect"
	"sync"
	"time"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/i18n"
	"v.io/v23/naming"
	"v.io/v23/options"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/v23/vdl"
	"v.io/v23/vdlroot/signature"
	"v.io/v23/verror"
	"v.io/v23/vom"
	"v.io/v23/vtrace"
	"v.io/x/lib/vlog"
	"v.io/x/ref/services/wspr/internal/lib"
	"v.io/x/ref/services/wspr/internal/namespace"
	"v.io/x/ref/services/wspr/internal/principal"
	"v.io/x/ref/services/wspr/internal/rpc/server"
)

const (
	// pkgPath is the prefix os errors in this package.
	pkgPath = "v.io/x/ref/services/wspr/internal/app"
)

// Errors
var (
	marshallingError       = verror.Register(pkgPath+".marshallingError", verror.NoRetry, "{1} {2} marshalling error {_}")
	noResults              = verror.Register(pkgPath+".noResults", verror.NoRetry, "{1} {2} no results from call {_}")
	badCaveatType          = verror.Register(pkgPath+".badCaveatType", verror.NoRetry, "{1} {2} bad caveat type {_}")
	unknownBlessings       = verror.Register(pkgPath+".unknownBlessings", verror.NoRetry, "{1} {2} unknown public id {_}")
	invalidBlessingsHandle = verror.Register(pkgPath+".invalidBlessingsHandle", verror.NoRetry, "{1} {2} invalid blessings handle {3} {_}")
)

type outstandingRequest struct {
	stream *outstandingStream
	cancel context.CancelFunc
}

// Controller represents all the state of a Vanadium Web App.  This is the struct
// that is in charge performing all the vanadium options.
type Controller struct {
	// Protects everything.
	// TODO(bjornick): We need to split this up.
	sync.Mutex

	// The context of this controller.
	ctx *context.T

	// The cleanup function for this controller.
	cancel context.CancelFunc

	// The rpc.ListenSpec to use with server.Listen
	listenSpec *rpc.ListenSpec

	// Used to generate unique ids for requests initiated by the proxy.
	// These ids will be even so they don't collide with the ids generated
	// by the client.
	lastGeneratedId int32

	// Used to keep track of data (streams and cancellation functions) for
	// outstanding requests.
	outstandingRequests map[int32]*outstandingRequest

	// Maps flowids to the server that owns them.
	flowMap map[int32]interface{}

	// A manager that Handles fetching and caching signature of remote services
	signatureManager lib.SignatureManager

	// We maintain multiple Vanadium server per pipe for serving JavaScript
	// services.
	servers map[uint32]*server.Server

	// Creates a client writer for a given flow.  This is a member so that tests can override
	// the default implementation.
	writerCreator func(id int32) lib.ClientWriter

	// Cache for all the Blessings that javascript has a handle to.
	blessingsCache *principal.JSBlessingsHandles

	// reservedServices contains a map of reserved service names.  These
	// are objects that serve requests in wspr without actually making
	// an outgoing rpc call.
	reservedServices map[string]rpc.Invoker
}

// NewController creates a new Controller.  writerCreator will be used to create a new flow for rpcs to
// javascript server.
func NewController(ctx *context.T, writerCreator func(id int32) lib.ClientWriter, listenSpec *rpc.ListenSpec, namespaceRoots []string, p security.Principal) (*Controller, error) {
	ctx, cancel := context.WithCancel(ctx)

	if namespaceRoots != nil {
		var err error
		ctx, _, err = v23.WithNewNamespace(ctx, namespaceRoots...)
		if err != nil {
			return nil, err
		}
	}

	ctx, _ = vtrace.WithNewTrace(ctx)

	ctx, err := v23.WithPrincipal(ctx, p)
	if err != nil {
		return nil, err
	}

	controller := &Controller{
		ctx:            ctx,
		cancel:         cancel,
		writerCreator:  writerCreator,
		listenSpec:     listenSpec,
		blessingsCache: principal.NewJSBlessingsHandles(),
	}

	controllerInvoker, err := rpc.ReflectInvoker(ControllerServer(controller))
	if err != nil {
		return nil, err
	}
	namespaceInvoker, err := rpc.ReflectInvoker(namespace.New(ctx))
	if err != nil {
		return nil, err
	}
	controller.reservedServices = map[string]rpc.Invoker{
		"__controller": controllerInvoker,
		"__namespace":  namespaceInvoker,
	}

	controller.setup()
	return controller, nil
}

// finishCall waits for the call to finish and write out the response to w.
func (c *Controller) finishCall(ctx *context.T, w lib.ClientWriter, clientCall rpc.ClientCall, msg *RpcRequest, span vtrace.Span) {
	if msg.IsStreaming {
		for {
			var item interface{}
			if err := clientCall.Recv(&item); err != nil {
				if err == io.EOF {
					break
				}
				w.Error(err) // Send streaming error as is
				return
			}
			if blessings, ok := item.(security.Blessings); ok {
				item = principal.ConvertBlessingsToHandle(blessings, c.blessingsCache.GetOrAddHandle(blessings))
			}
			vomItem, err := lib.VomEncode(item)
			if err != nil {
				w.Error(verror.New(marshallingError, ctx, item, err))
				continue
			}
			if err := w.Send(lib.ResponseStream, vomItem); err != nil {
				w.Error(verror.New(marshallingError, ctx, item))
			}
		}
		if err := w.Send(lib.ResponseStreamClose, nil); err != nil {
			w.Error(verror.New(marshallingError, ctx, "ResponseStreamClose"))
		}
	}
	results := make([]*vdl.Value, msg.NumOutArgs)
	wireBlessingsType := vdl.TypeOf(security.WireBlessings{})
	// This array will have pointers to the values in results.
	resultptrs := make([]interface{}, msg.NumOutArgs)
	for i := range results {
		resultptrs[i] = &results[i]
	}
	if err := clientCall.Finish(resultptrs...); err != nil {
		// return the call system error as is
		w.Error(err)
		return
	}
	for i, val := range results {
		if val.Type() == wireBlessingsType {
			var blessings security.Blessings
			if err := vdl.Convert(&blessings, val); err != nil {
				w.Error(err)
				return
			}
			results[i] = vdl.ValueOf(principal.ConvertBlessingsToHandle(blessings, c.blessingsCache.GetOrAddHandle(blessings)))
		}
	}
	c.sendRPCResponse(ctx, w, span, results)
}

func (c *Controller) sendRPCResponse(ctx *context.T, w lib.ClientWriter, span vtrace.Span, results []*vdl.Value) {
	span.Finish()
	response := RpcResponse{
		OutArgs:       results,
		TraceResponse: vtrace.GetResponse(ctx),
	}
	encoded, err := lib.VomEncode(response)
	if err != nil {
		w.Error(err)
		return
	}
	if err := w.Send(lib.ResponseFinal, encoded); err != nil {
		w.Error(verror.Convert(marshallingError, ctx, err))
	}
}

// callOpts turns a slice of type []RpcCallOption object into an array of rpc.CallOpt.
func (c *Controller) callOpts(opts []RpcCallOption) ([]rpc.CallOpt, error) {
	var callOpts []rpc.CallOpt

	for _, opt := range opts {
		switch v := opt.(type) {
		case RpcCallOptionAllowedServersPolicy:
			callOpts = append(callOpts, options.AllowedServersPolicy(v.Value))
		case RpcCallOptionRetryTimeout:
			callOpts = append(callOpts, options.RetryTimeout(v.Value))
		case RpcCallOptionGranter:
			callOpts = append(callOpts, &jsGranter{c, v.Value})
		default:
			return nil, fmt.Errorf("Unknown RpcCallOption type %T", v)
		}
	}

	return callOpts, nil
}

// serverOpts turns a slice of type []RpcServerOptions object into an array of rpc.ServerOpt.
func (c *Controller) serverOpts(opts []RpcServerOption) ([]rpc.ServerOpt, error) {
	var serverOpts []rpc.ServerOpt

	for _, opt := range opts {
		switch v := opt.(type) {
		case RpcServerOptionIsLeaf:
			serverOpts = append(serverOpts, options.IsLeaf(v.Value))
		case RpcServerOptionServesMountTable:
			serverOpts = append(serverOpts, options.ServesMountTable(v.Value))
		default:
			return nil, fmt.Errorf("Unknown RpcServerOption type %T", v)
		}
	}

	return serverOpts, nil
}

func (c *Controller) startCall(ctx *context.T, w lib.ClientWriter, msg *RpcRequest, inArgs []interface{}) (rpc.ClientCall, error) {
	methodName := lib.UppercaseFirstCharacter(msg.Method)
	callOpts, err := c.callOpts(msg.CallOptions)
	if err != nil {
		return nil, err
	}
	clientCall, err := v23.GetClient(ctx).StartCall(ctx, msg.Name, methodName, inArgs, callOpts...)
	if err != nil {
		return nil, fmt.Errorf("error starting call (name: %v, method: %v, args: %v): %v", msg.Name, methodName, inArgs, err)
	}

	return clientCall, nil
}

// Implements the serverHelper interface

// CreateNewFlow creats a new server flow that will be used to write out
// streaming messages to Javascript.
func (c *Controller) CreateNewFlow(s interface{}, stream rpc.Stream) *server.Flow {
	c.Lock()
	defer c.Unlock()
	id := c.lastGeneratedId
	c.lastGeneratedId += 2
	c.flowMap[id] = s
	os := newStream(c.blessingsCache)
	os.init(stream)
	c.outstandingRequests[id] = &outstandingRequest{
		stream: os,
	}
	return &server.Flow{ID: id, Writer: c.writerCreator(id)}
}

// CleanupFlow removes the bookkeeping for a previously created flow.
func (c *Controller) CleanupFlow(id int32) {
	c.Lock()
	request := c.outstandingRequests[id]
	delete(c.outstandingRequests, id)
	delete(c.flowMap, id)
	c.Unlock()
	if request != nil && request.stream != nil {
		request.stream.end()
		request.stream.waitUntilDone()
	}
}

// RT returns the runtime of the app.
func (c *Controller) Context() *context.T {
	return c.ctx
}

// GetOrAddBlessingsHandle adds the Blessings to the local blessings store if they
// don't already existand returns the handle to it.  This function exists
// because JS only has a handle to the blessings to avoid shipping the
// certificate forest to JS and back.
func (c *Controller) GetOrAddBlessingsHandle(blessings security.Blessings) principal.BlessingsHandle {
	return c.blessingsCache.GetOrAddHandle(blessings)
}

// GetBlessings gets blessings for a given blessings handle.
func (c *Controller) GetBlessings(handle principal.BlessingsHandle) security.Blessings {
	return c.blessingsCache.GetBlessings(handle)
}

// Cleanup cleans up any outstanding rpcs.
func (c *Controller) Cleanup() {
	vlog.VI(0).Info("Cleaning up controller")
	c.Lock()

	for _, request := range c.outstandingRequests {
		if request.cancel != nil {
			request.cancel()
		}
		if request.stream != nil {
			request.stream.end()
		}
	}

	servers := []*server.Server{}
	for _, server := range c.servers {
		servers = append(servers, server)
	}

	c.Unlock()

	// We must unlock before calling server.Stop otherwise it can deadlock.
	for _, server := range servers {
		server.Stop()
	}

	c.cancel()
}

func (c *Controller) setup() {
	c.signatureManager = lib.NewSignatureManager()
	c.outstandingRequests = make(map[int32]*outstandingRequest)
	c.flowMap = make(map[int32]interface{})
	c.servers = make(map[uint32]*server.Server)
}

// SendOnStream writes data on id's stream.  The actual network write will be
// done asynchronously.  If there is an error, it will be sent to w.
func (c *Controller) SendOnStream(id int32, data string, w lib.ClientWriter) {
	c.Lock()
	request := c.outstandingRequests[id]
	if request == nil || request.stream == nil {
		vlog.Errorf("unknown stream: %d", id)
		c.Unlock()
		return
	}
	stream := request.stream
	c.Unlock()
	stream.send(data, w)
}

// SendVeyronRequest makes a vanadium request for the given flowId.  If signal is non-nil, it will receive
// the call object after it has been constructed.
func (c *Controller) sendVeyronRequest(ctx *context.T, id int32, msg *RpcRequest, inArgs []interface{}, w lib.ClientWriter, stream *outstandingStream, span vtrace.Span) {
	sig, err := c.getSignature(ctx, msg.Name)
	if err != nil {
		w.Error(err)
		return
	}
	methName := lib.UppercaseFirstCharacter(msg.Method)
	methSig, ok := signature.FirstMethod(sig, methName)
	if !ok {
		w.Error(fmt.Errorf("method %q not found in signature: %#v", methName, sig))
		return
	}
	if len(methSig.InArgs) != len(inArgs) {
		w.Error(fmt.Errorf("invalid number of arguments, expected: %v, got:%v", methSig, *msg))
		return
	}

	for i, arg := range inArgs {
		if jsBlessings, ok := arg.(principal.JsBlessings); ok {
			inArgs[i] = c.blessingsCache.GetBlessings(jsBlessings.Handle)
		}
	}
	// We have to make the start call synchronous so we can make sure that we populate
	// the call map before we can Handle a recieve call.
	call, err := c.startCall(ctx, w, msg, inArgs)
	if err != nil {
		w.Error(verror.Convert(verror.ErrInternal, ctx, err))
		return
	}

	if stream != nil {
		stream.init(call)
	}

	c.finishCall(ctx, w, call, msg, span)
	c.Lock()
	if request, ok := c.outstandingRequests[id]; ok {
		delete(c.outstandingRequests, id)
		if request.cancel != nil {
			request.cancel()
		}
	}
	c.Unlock()
}

// TODO(mattr): This is a very limited implementation of ServerCall,
// but currently none of the methods the controller exports require
// any of this context information.
type localCall struct {
	ctx  *context.T
	vrpc *RpcRequest
	tags []*vdl.Value
	w    lib.ClientWriter
}

var (
	_ rpc.StreamServerCall = (*localCall)(nil)
	_ security.Call        = (*localCall)(nil)
)

func (l *localCall) Send(item interface{}) error {
	vomItem, err := lib.VomEncode(item)
	if err != nil {
		err = verror.New(marshallingError, l.ctx, item, err)
		l.w.Error(err)
		return err
	}
	if err := l.w.Send(lib.ResponseStream, vomItem); err != nil {
		err = verror.New(marshallingError, l.ctx, item)
		l.w.Error(err)
		return err
	}
	return nil
}
func (l *localCall) Recv(interface{}) error                          { return nil }
func (l *localCall) GrantedBlessings() security.Blessings            { return security.Blessings{} }
func (l *localCall) Server() rpc.Server                              { return nil }
func (l *localCall) Timestamp() (t time.Time)                        { return }
func (l *localCall) Method() string                                  { return l.vrpc.Method }
func (l *localCall) MethodTags() []*vdl.Value                        { return l.tags }
func (l *localCall) Suffix() string                                  { return l.vrpc.Name }
func (l *localCall) LocalDischarges() map[string]security.Discharge  { return nil }
func (l *localCall) RemoteDischarges() map[string]security.Discharge { return nil }
func (l *localCall) LocalPrincipal() security.Principal              { return nil }
func (l *localCall) LocalBlessings() security.Blessings              { return security.Blessings{} }
func (l *localCall) RemoteBlessings() security.Blessings             { return security.Blessings{} }
func (l *localCall) LocalEndpoint() naming.Endpoint                  { return nil }
func (l *localCall) RemoteEndpoint() naming.Endpoint                 { return nil }
func (l *localCall) Security() security.Call                         { return l }

func (c *Controller) handleInternalCall(ctx *context.T, invoker rpc.Invoker, msg *RpcRequest, decoder *vom.Decoder, w lib.ClientWriter, span vtrace.Span) {
	argptrs, tags, err := invoker.Prepare(msg.Method, int(msg.NumInArgs))
	if err != nil {
		w.Error(verror.Convert(verror.ErrInternal, ctx, err))
		return
	}
	for _, argptr := range argptrs {
		if err := decoder.Decode(argptr); err != nil {
			w.Error(verror.Convert(verror.ErrInternal, ctx, err))
			return
		}
	}
	results, err := invoker.Invoke(ctx, &localCall{ctx, msg, tags, w}, msg.Method, argptrs)
	if err != nil {
		w.Error(verror.Convert(verror.ErrInternal, ctx, err))
		return
	}
	if msg.IsStreaming {
		if err := w.Send(lib.ResponseStreamClose, nil); err != nil {
			w.Error(verror.New(marshallingError, ctx, "ResponseStreamClose"))
		}
	}

	// Convert results from []interface{} to []*vdl.Value.
	vresults := make([]*vdl.Value, len(results))
	for i, res := range results {
		vv, err := vdl.ValueFromReflect(reflect.ValueOf(res))
		if err != nil {
			w.Error(verror.Convert(verror.ErrInternal, ctx, err))
			return
		}
		vresults[i] = vv
	}
	c.sendRPCResponse(ctx, w, span, vresults)
}

// HandleCaveatValidationResponse handles the response to caveat validation
// requests.
func (c *Controller) HandleCaveatValidationResponse(id int32, data string) {
	c.Lock()
	server, ok := c.flowMap[id].(*server.Server)
	c.Unlock()
	if !ok {
		vlog.Errorf("unexpected result from JavaScript. No server found matching id %d.", id)
		return // ignore unknown server
	}
	server.HandleCaveatValidationResponse(id, data)
}

// HandleVeyronRequest starts a vanadium rpc and returns before the rpc has been completed.
func (c *Controller) HandleVeyronRequest(ctx *context.T, id int32, data string, w lib.ClientWriter) {
	binbytes, err := hex.DecodeString(data)
	if err != nil {
		w.Error(verror.Convert(verror.ErrInternal, ctx, fmt.Errorf("Error decoding hex string %q: %v", data, err)))
		return
	}
	decoder := vom.NewDecoder(bytes.NewReader(binbytes))
	var msg RpcRequest
	if err := decoder.Decode(&msg); err != nil {
		w.Error(verror.Convert(verror.ErrInternal, ctx, err))
		return
	}
	vlog.VI(2).Infof("Rpc: %s.%s(..., streaming=%v)", msg.Name, msg.Method, msg.IsStreaming)
	spanName := fmt.Sprintf("<wspr>%q.%s", msg.Name, msg.Method)
	ctx, span := vtrace.WithContinuedTrace(ctx, spanName, msg.TraceRequest)
	ctx = i18n.WithLangID(ctx, i18n.LangID(msg.Context.Language))

	var cctx *context.T
	var cancel context.CancelFunc

	// TODO(mattr): To be consistent with go, we should not ignore 0 timeouts.
	// However as a rollout strategy we must, otherwise there is a circular
	// dependency between the WSPR change and the JS change that will follow.
	if msg.Deadline.IsZero() {
		cctx, cancel = context.WithCancel(ctx)
	} else {
		cctx, cancel = context.WithDeadline(ctx, msg.Deadline.Time)
	}

	// If this message is for an internal service, do a short-circuit dispatch here.
	if invoker, ok := c.reservedServices[msg.Name]; ok {
		go c.handleInternalCall(ctx, invoker, &msg, decoder, w, span)
		return
	}

	inArgs := make([]interface{}, msg.NumInArgs)
	for i := range inArgs {
		var v *vdl.Value
		if err := decoder.Decode(&v); err != nil {
			w.Error(err)
			return
		}
		inArgs[i] = v
	}

	request := &outstandingRequest{
		cancel: cancel,
	}
	if msg.IsStreaming {
		// If this rpc is streaming, we would expect that the client would try to send
		// on this stream.  Since the initial handshake is done asynchronously, we have
		// to put the outstanding stream in the map before we make the async call so that
		// the future send know which queue to write to, even if the client call isn't
		// actually ready yet.
		request.stream = newStream(c.blessingsCache)
	}
	c.Lock()
	c.outstandingRequests[id] = request
	go c.sendVeyronRequest(cctx, id, &msg, inArgs, w, request.stream, span)
	c.Unlock()
}

// HandleVeyronCancellation cancels the request corresponding to the
// given id if it is still outstanding.
func (c *Controller) HandleVeyronCancellation(id int32) {
	c.Lock()
	defer c.Unlock()
	if request, ok := c.outstandingRequests[id]; ok && request.cancel != nil {
		request.cancel()
	}
}

// CloseStream closes the stream for a given id.
func (c *Controller) CloseStream(id int32) {
	c.Lock()
	defer c.Unlock()
	if request, ok := c.outstandingRequests[id]; ok && request.stream != nil {
		request.stream.end()
		return
	}
	vlog.Errorf("close called on non-existent call: %v", id)
}

func (c *Controller) maybeCreateServer(serverId uint32, opts ...rpc.ServerOpt) (*server.Server, error) {
	c.Lock()
	defer c.Unlock()
	if server, ok := c.servers[serverId]; ok {
		return server, nil
	}
	server, err := server.NewServer(serverId, c.listenSpec, c, opts...)
	if err != nil {
		return nil, err
	}
	c.servers[serverId] = server
	return server, nil
}

// HandleLookupResponse handles the result of a Dispatcher.Lookup call that was
// run by the Javascript server.
func (c *Controller) HandleLookupResponse(id int32, data string) {
	c.Lock()
	server, ok := c.flowMap[id].(*server.Server)
	c.Unlock()
	if !ok {
		vlog.Errorf("unexpected result from JavaScript. No channel "+
			"for MessageId: %d exists. Ignoring the results.", id)
		//Ignore unknown responses that don't belong to any channel
		return
	}
	server.HandleLookupResponse(id, data)
}

// HandleAuthResponse handles the result of a Authorizer.Authorize call that was
// run by the Javascript server.
func (c *Controller) HandleAuthResponse(id int32, data string) {
	c.Lock()
	server, ok := c.flowMap[id].(*server.Server)
	c.Unlock()
	if !ok {
		vlog.Errorf("unexpected result from JavaScript. No channel "+
			"for MessageId: %d exists. Ignoring the results.", id)
		//Ignore unknown responses that don't belong to any channel
		return
	}
	server.HandleAuthResponse(id, data)
}

// Serve instructs WSPR to start listening for calls on behalf
// of a javascript server.
func (c *Controller) Serve(_ *context.T, _ rpc.ServerCall, name string, serverId uint32, rpcServerOpts []RpcServerOption) error {

	opts, err := c.serverOpts(rpcServerOpts)
	if err != nil {
		return verror.Convert(verror.ErrInternal, nil, err)
	}
	server, err := c.maybeCreateServer(serverId, opts...)
	if err != nil {
		return verror.Convert(verror.ErrInternal, nil, err)
	}
	vlog.VI(2).Infof("serving under name: %q", name)
	if err := server.Serve(name); err != nil {
		return verror.Convert(verror.ErrInternal, nil, err)
	}
	return nil
}

// Stop instructs WSPR to stop listening for calls for the
// given javascript server.
func (c *Controller) Stop(_ *context.T, _ rpc.ServerCall, serverId uint32) error {
	c.Lock()
	server, ok := c.servers[serverId]
	if !ok {
		c.Unlock()
		return nil
	}
	delete(c.servers, serverId)
	c.Unlock()

	server.Stop()
	return nil
}

// AddName adds a published name to an existing server.
func (c *Controller) AddName(_ *context.T, _ rpc.ServerCall, serverId uint32, name string) error {
	// Create a server for the pipe, if it does not exist already
	server, err := c.maybeCreateServer(serverId)
	if err != nil {
		return verror.Convert(verror.ErrInternal, nil, err)
	}
	// Add name
	if err := server.AddName(name); err != nil {
		return verror.Convert(verror.ErrInternal, nil, err)
	}
	return nil
}

// RemoveName removes a published name from an existing server.
func (c *Controller) RemoveName(_ *context.T, _ rpc.ServerCall, serverId uint32, name string) error {
	// Create a server for the pipe, if it does not exist already
	server, err := c.maybeCreateServer(serverId)
	if err != nil {
		return verror.Convert(verror.ErrInternal, nil, err)
	}
	// Remove name
	server.RemoveName(name)
	// Remove name from signature cache as well
	c.signatureManager.FlushCacheEntry(name)
	return nil
}

// HandleServerResponse handles the completion of outstanding calls to JavaScript services
// by filling the corresponding channel with the result from JavaScript.
func (c *Controller) HandleServerResponse(id int32, data string) {
	c.Lock()
	server, ok := c.flowMap[id].(*server.Server)
	c.Unlock()
	if !ok {
		vlog.Errorf("unexpected result from JavaScript. No channel "+
			"for MessageId: %d exists. Ignoring the results.", id)
		//Ignore unknown responses that don't belong to any channel
		return
	}
	server.HandleServerResponse(id, data)
}

// parseVeyronRequest parses a json rpc request into a RpcRequest object.
func (c *Controller) parseVeyronRequest(data string) (*RpcRequest, error) {
	var msg RpcRequest
	if err := lib.VomDecode(data, &msg); err != nil {
		return nil, err
	}
	vlog.VI(2).Infof("RpcRequest: %s.%s(..., streaming=%v)", msg.Name, msg.Method, msg.IsStreaming)
	return &msg, nil
}

// getSignature uses the signature manager to get and cache the signature of a remote server.
func (c *Controller) getSignature(ctx *context.T, name string) ([]signature.Interface, error) {
	return c.signatureManager.Signature(ctx, name)
}

// Signature uses the signature manager to get and cache the signature of a remote server.
func (c *Controller) Signature(ctx *context.T, _ rpc.ServerCall, name string) ([]signature.Interface, error) {
	return c.getSignature(ctx, name)
}

// UnlinkBlessings removes the given blessings from the blessings store.
func (c *Controller) UnlinkBlessings(_ *context.T, _ rpc.ServerCall, handle principal.BlessingsHandle) error {
	return c.blessingsCache.RemoveReference(handle)
}

// Bless binds extensions of blessings held by this principal to
// another principal (represented by its public key).
func (c *Controller) Bless(_ *context.T, _ rpc.ServerCall, publicKey string, blessingHandle principal.BlessingsHandle, extension string, caveats []security.Caveat) (string, principal.BlessingsHandle, error) {
	var inputBlessing security.Blessings
	if inputBlessing = c.GetBlessings(blessingHandle); inputBlessing.IsZero() {
		return "", principal.ZeroHandle, verror.New(invalidBlessingsHandle, nil, blessingHandle)
	}

	key, err := principal.DecodePublicKey(publicKey)
	if err != nil {
		return "", principal.ZeroHandle, err
	}

	if len(caveats) == 0 {
		caveats = append(caveats, security.UnconstrainedUse())
	}

	p := v23.GetPrincipal(c.ctx)
	blessings, err := p.Bless(key, inputBlessing, extension, caveats[0], caveats[1:]...)
	if err != nil {
		return "", principal.ZeroHandle, err
	}
	handle := c.blessingsCache.GetOrAddHandle(blessings)
	return publicKey, handle, nil
}

// BlessSelf creates a blessing with the provided name for this principal.
func (c *Controller) BlessSelf(_ *context.T, _ rpc.ServerCall, extension string, caveats []security.Caveat) (string, principal.BlessingsHandle, error) {
	p := v23.GetPrincipal(c.ctx)
	blessings, err := p.BlessSelf(extension)
	if err != nil {
		return "", principal.ZeroHandle, verror.Convert(verror.ErrInternal, nil, err)
	}

	handle := c.blessingsCache.GetOrAddHandle(blessings)

	encKey, err := principal.EncodePublicKey(p.PublicKey())
	return encKey, handle, err
}

// PutToBlessingStore puts a blessing with the provided name to the blessing store
// with the specified blessing pattern.
func (c *Controller) PutToBlessingStore(_ *context.T, _ rpc.ServerCall, handle principal.BlessingsHandle, pattern security.BlessingPattern) (*principal.JsBlessings, error) {
	var inputBlessings security.Blessings
	if inputBlessings = c.GetBlessings(handle); inputBlessings.IsZero() {
		return nil, verror.New(invalidBlessingsHandle, nil, handle)
	}

	p := v23.GetPrincipal(c.ctx)
	outBlessings, err := p.BlessingStore().Set(inputBlessings, security.BlessingPattern(pattern))
	if err != nil {
		return nil, err
	}

	if outBlessings.IsZero() {
		return nil, nil
	}

	jsBlessings := principal.ConvertBlessingsToHandle(outBlessings, c.blessingsCache.GetOrAddHandle(outBlessings))
	return jsBlessings, nil
}

// AddToRoots adds the provided blessing as a root.
func (c *Controller) AddToRoots(_ *context.T, _ rpc.ServerCall, handle principal.BlessingsHandle) error {
	var inputBlessings security.Blessings
	if inputBlessings = c.GetBlessings(handle); inputBlessings.IsZero() {
		return verror.New(invalidBlessingsHandle, nil, handle)
	}

	p := v23.GetPrincipal(c.ctx)
	return p.AddToRoots(inputBlessings)
}

func (c *Controller) GetDefaultBlessings(*context.T, rpc.ServerCall) (*principal.JsBlessings, error) {
	p := v23.GetPrincipal(c.ctx)
	outBlessings := p.BlessingStore().Default()

	if outBlessings.IsZero() {
		return nil, nil
	}

	jsBlessing := principal.ConvertBlessingsToHandle(outBlessings, c.blessingsCache.GetOrAddHandle(outBlessings))
	return jsBlessing, nil
}

// HandleGranterResponse handles the result of a Granter request.
func (c *Controller) HandleGranterResponse(id int32, data string) {
	c.Lock()
	granterStr, ok := c.flowMap[id].(*granterStream)
	c.Unlock()
	if !ok {
		vlog.Errorf("unexpected result from JavaScript. Flow was not a granter "+
			"stream for MessageId: %d exists. Ignoring the results.", id)
		//Ignore unknown responses that don't belong to any channel
		return
	}
	granterStr.Send(data)
}

func (c *Controller) BlessingsDebugString(_ *context.T, _ rpc.ServerCall, handle principal.BlessingsHandle) (string, error) {
	var inputBlessings security.Blessings
	if inputBlessings = c.GetBlessings(handle); inputBlessings.IsZero() {
		return "", verror.New(invalidBlessingsHandle, nil, handle)
	}

	return inputBlessings.String(), nil
}

func (c *Controller) RemoteBlessings(ctx *context.T, _ rpc.ServerCall, name, method string) ([]string, error) {
	vlog.VI(2).Infof("requesting remote blessings for %q", name)

	cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	clientCall, err := v23.GetClient(cctx).StartCall(cctx, name, method, nil)
	if err != nil {
		return nil, verror.Convert(verror.ErrInternal, cctx, err)
	}

	blessings, _ := clientCall.RemoteBlessings()
	return blessings, nil
}

func (c *Controller) SendLogMessage(level lib.LogLevel, msg string) error {
	c.Lock()
	defer c.Unlock()
	id := c.lastGeneratedId
	c.lastGeneratedId += 2
	return c.writerCreator(id).Send(lib.ResponseLog, lib.LogMessage{
		Level:   level,
		Message: msg,
	})
}
