// The app package contains the struct that keeps per javascript app state and handles translating
// javascript requests to veyron requests and vice versa.
package app

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"sync"
	"time"

	vsecurity "v.io/core/veyron/security"
	"v.io/core/veyron2"
	"v.io/core/veyron2/context"
	"v.io/core/veyron2/ipc"
	"v.io/core/veyron2/naming"
	"v.io/core/veyron2/options"
	"v.io/core/veyron2/security"
	"v.io/core/veyron2/vdl"
	"v.io/core/veyron2/vdl/vdlroot/src/signature"
	"v.io/core/veyron2/verror"
	"v.io/core/veyron2/vlog"
	"v.io/core/veyron2/vom"
	"v.io/core/veyron2/vtrace"
	"v.io/wspr/veyron/services/wsprd/ipc/server"
	"v.io/wspr/veyron/services/wsprd/lib"
	"v.io/wspr/veyron/services/wsprd/namespace"
	"v.io/wspr/veyron/services/wsprd/principal"
)

// pkgPath is the prefix os errors in this package.
const pkgPath = "v.io/core/veyron/services/wsprd/app"

// Errors
var (
	marshallingError       = verror.Register(pkgPath+".marshallingError", verror.NoRetry, "{1} {2} marshalling error {_}")
	noResults              = verror.Register(pkgPath+".noResults", verror.NoRetry, "{1} {2} no results from call {_}")
	badCaveatType          = verror.Register(pkgPath+".badCaveatType", verror.NoRetry, "{1} {2} bad caveat type {_}")
	unknownBlessings       = verror.Register(pkgPath+".unknownBlessings", verror.NoRetry, "{1} {2} unknown public id {_}")
	invalidBlessingsHandle = verror.Register(pkgPath+".invalidBlessingsHandle", verror.NoRetry, "{1} {2} invalid blessings handle {_}")
)

// TODO(bjornick,nlacasse): Remove the retryTimeout flag once we able
// to pass it in from javascript. For now all RPCs have the same
// retryTimeout, set by command line flag.
var retryTimeout *int

func init() {
	// TODO(bjornick,nlacasse): Remove the retryTimeout flag once we able
	// to pass it in from javascript. For now all RPCs have the same
	// retryTimeout, set by command line flag.
	retryTimeout = flag.Int("retry-timeout", 2, "Duration in seconds to retry starting an RPC call. 0 means never retry.")
}

type serveRequest struct {
	Name     string
	ServerId uint32
}

type addRemoveNameRequest struct {
	Name     string
	ServerId uint32
}

type outstandingRequest struct {
	stream *outstandingStream
	cancel context.CancelFunc
}

// Controller represents all the state of a Veyron Web App.  This is the struct
// that is in charge performing all the veyron options.
type Controller struct {
	// Protects everything.
	// TODO(bjornick): We need to split this up.
	sync.Mutex

	// The context of this controller.
	ctx *context.T

	// The cleanup function for this controller.
	cancel context.CancelFunc

	// The ipc.ListenSpec to use with server.Listen
	listenSpec *ipc.ListenSpec

	// Used to generate unique ids for requests initiated by the proxy.
	// These ids will be even so they don't collide with the ids generated
	// by the client.
	lastGeneratedId int32

	// Used to keep track of data (streams and cancellation functions) for
	// outstanding requests.
	outstandingRequests map[int32]*outstandingRequest

	// Maps flowids to the server that owns them.
	flowMap map[int32]*server.Server

	// A manager that Handles fetching and caching signature of remote services
	signatureManager lib.SignatureManager

	// We maintain multiple Veyron server per pipe for serving JavaScript
	// services.
	servers map[uint32]*server.Server

	// Creates a client writer for a given flow.  This is a member so that tests can override
	// the default implementation.
	writerCreator func(id int32) lib.ClientWriter

	veyronProxyEP string

	// Store for all the Blessings that javascript has a handle to.
	blessingsStore *principal.JSBlessingsHandles
}

// NewController creates a new Controller.  writerCreator will be used to create a new flow for rpcs to
// javascript server. veyronProxyEP is an endpoint for the veyron proxy to serve through.  It can't be empty.
func NewController(ctx *context.T, writerCreator func(id int32) lib.ClientWriter, listenSpec *ipc.ListenSpec, namespaceRoots []string, p security.Principal) (*Controller, error) {
	ctx, cancel := context.WithCancel(ctx)

	ctx, _ = vtrace.SetNewTrace(ctx)

	if namespaceRoots != nil {
		veyron2.GetNamespace(ctx).SetRoots(namespaceRoots...)
	}

	ctx, err := veyron2.SetPrincipal(ctx, p)
	if err != nil {
		return nil, err
	}

	controller := &Controller{
		ctx:            ctx,
		cancel:         cancel,
		writerCreator:  writerCreator,
		listenSpec:     listenSpec,
		blessingsStore: principal.NewJSBlessingsHandles(),
	}

	controller.setup()
	return controller, nil
}

// finishCall waits for the call to finish and write out the response to w.
func (c *Controller) finishCall(ctx *context.T, w lib.ClientWriter, clientCall ipc.Call, msg *VeyronRPCRequest, span vtrace.Span) {
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

	// TODO(bprosnitz) Remove this when we remove error from out args everywhere.
	numOutArgsWithError := msg.NumOutArgs + 1

	results := make([]interface{}, numOutArgsWithError)
	// This array will have pointers to the values in result.
	resultptrs := make([]interface{}, numOutArgsWithError)
	for ax := range results {
		resultptrs[ax] = &results[ax]
	}
	if err := clientCall.Finish(resultptrs...); err != nil {
		// return the call system error as is
		w.Error(err)
		return
	}
	c.sendRPCResponse(ctx, w, span, results)
}

func (c *Controller) sendRPCResponse(ctx *context.T, w lib.ClientWriter, span vtrace.Span, results []interface{}) {
	// for now we assume last out argument is always error
	if err, ok := results[len(results)-1].(error); ok {
		// return the call Application error as is
		w.Error(err)
		return
	}

	outargs := make([]vdl.AnyRep, len(results)-1)
	for i := range outargs {
		outargs[i] = results[i]
	}

	span.Finish()
	traceRecord := vtrace.GetStore(ctx).TraceRecord(span.Trace())

	response := VeyronRPCResponse{
		OutArgs: outargs,
		TraceResponse: vtrace.Response{
			Method: vtrace.InMemory,
			Trace:  *traceRecord,
		},
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

func (c *Controller) startCall(ctx *context.T, w lib.ClientWriter, msg *VeyronRPCRequest, inArgs []interface{}) (ipc.Call, error) {
	methodName := lib.UppercaseFirstCharacter(msg.Method)
	retryTimeoutOpt := options.RetryTimeout(time.Duration(*retryTimeout) * time.Second)
	clientCall, err := veyron2.GetClient(ctx).StartCall(ctx, msg.Name, methodName, inArgs, retryTimeoutOpt)
	if err != nil {
		return nil, fmt.Errorf("error starting call (name: %v, method: %v, args: %v): %v", msg.Name, methodName, inArgs, err)
	}

	return clientCall, nil
}

// Implements the serverHelper interface

// CreateNewFlow creats a new server flow that will be used to write out
// streaming messages to Javascript.
func (c *Controller) CreateNewFlow(s *server.Server, stream ipc.Stream) *server.Flow {
	c.Lock()
	defer c.Unlock()
	id := c.lastGeneratedId
	c.lastGeneratedId += 2
	c.flowMap[id] = s
	os := newStream()
	os.init(stream)
	c.outstandingRequests[id] = &outstandingRequest{
		stream: os,
	}
	return &server.Flow{ID: id, Writer: c.writerCreator(id)}
}

// CleanupFlow removes the bookkeping for a previously created flow.
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

// AddBlessings adds the Blessings to the local blessings store and returns
// the handle to it.  This function exists because JS only has
// a handle to the blessings to avoid shipping the certificate forest
// to JS and back.
func (c *Controller) AddBlessings(blessings security.Blessings) int32 {
	return c.blessingsStore.Add(blessings)
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
	c.flowMap = make(map[int32]*server.Server)
	c.servers = make(map[uint32]*server.Server)
}

// SendOnStream writes data on id's stream.  The actual network write will be
// done asynchronously.  If there is an error, it will be sent to w.
func (c *Controller) SendOnStream(id int32, data string, w lib.ClientWriter) {
	c.Lock()
	request := c.outstandingRequests[id]
	if request == nil || request.stream == nil {
		vlog.Errorf("unknown stream: %d", id)
		return
	}
	stream := request.stream
	c.Unlock()
	stream.send(data, w)
}

// SendVeyronRequest makes a veyron request for the given flowId.  If signal is non-nil, it will receive
// the call object after it has been constructed.
func (c *Controller) sendVeyronRequest(ctx *context.T, id int32, msg *VeyronRPCRequest, inArgs []interface{}, w lib.ClientWriter, stream *outstandingStream, span vtrace.Span) {
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

	// We have to make the start call synchronous so we can make sure that we populate
	// the call map before we can Handle a recieve call.
	call, err := c.startCall(ctx, w, msg, inArgs)
	if err != nil {
		w.Error(verror.Convert(verror.Internal, ctx, err))
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
	vrpc *VeyronRPCRequest
	tags []interface{}
}

func (l *localCall) Send(interface{}) error                          { return nil }
func (l *localCall) Recv(interface{}) error                          { return nil }
func (l *localCall) Blessings() security.Blessings                   { return nil }
func (l *localCall) Server() ipc.Server                              { return nil }
func (l *localCall) Context() *context.T                             { return l.ctx }
func (l *localCall) Timestamp() (t time.Time)                        { return }
func (l *localCall) Method() string                                  { return l.vrpc.Method }
func (l *localCall) MethodTags() []interface{}                       { return l.tags }
func (l *localCall) Name() string                                    { return l.vrpc.Name }
func (l *localCall) Suffix() string                                  { return "" }
func (l *localCall) RemoteDischarges() map[string]security.Discharge { return nil }
func (l *localCall) LocalPrincipal() security.Principal              { return nil }
func (l *localCall) LocalBlessings() security.Blessings              { return nil }
func (l *localCall) RemoteBlessings() security.Blessings             { return nil }
func (l *localCall) LocalEndpoint() naming.Endpoint                  { return nil }
func (l *localCall) RemoteEndpoint() naming.Endpoint                 { return nil }

func (c *Controller) handleInternalCall(ctx *context.T, msg *VeyronRPCRequest, decoder *vom.Decoder, w lib.ClientWriter, span vtrace.Span) {
	invoker, err := ipc.ReflectInvoker(ControllerServer(c))
	if err != nil {
		w.Error(verror.Convert(verror.Internal, ctx, err))
		return
	}
	argptrs, tags, err := invoker.Prepare(msg.Method, int(msg.NumInArgs))
	if err != nil {
		w.Error(verror.Convert(verror.Internal, ctx, err))
		return
	}
	for _, argptr := range argptrs {
		if err := decoder.Decode(argptr); err != nil {
			w.Error(verror.Convert(verror.Internal, ctx, err))
			return
		}
	}
	results, err := invoker.Invoke(msg.Method, &localCall{ctx, msg, tags}, argptrs)
	if err != nil {
		w.Error(verror.Convert(verror.Internal, ctx, err))
		return
	}
	c.sendRPCResponse(ctx, w, span, results)
}

// HandleVeyronRequest starts a veyron rpc and returns before the rpc has been completed.
func (c *Controller) HandleVeyronRequest(ctx *context.T, id int32, data string, w lib.ClientWriter) {
	binbytes, err := hex.DecodeString(data)
	if err != nil {
		w.Error(verror.Convert(verror.Internal, ctx, err))
		return
	}
	decoder, err := vom.NewDecoder(bytes.NewReader(binbytes))
	if err != nil {
		w.Error(verror.Convert(verror.Internal, ctx, err))
		return
	}

	var msg VeyronRPCRequest
	if err := decoder.Decode(&msg); err != nil {
		w.Error(verror.Convert(verror.Internal, ctx, err))
		return
	}
	vlog.VI(2).Infof("VeyronRPC: %s.%s(..., streaming=%v)", msg.Name, msg.Method, msg.IsStreaming)
	spanName := fmt.Sprintf("<wspr>%q.%s", msg.Name, msg.Method)
	ctx, span := vtrace.SetContinuedTrace(ctx, spanName, msg.TraceRequest)

	var cctx *context.T
	var cancel context.CancelFunc

	// TODO(mattr): To be consistent with go, we should not ignore 0 timeouts.
	// However as a rollout strategy we must, otherwise there is a circular
	// dependency between the WSPR change and the JS change that will follow.
	if msg.Timeout == lib.JSIPCNoTimeout || msg.Timeout == 0 {
		cctx, cancel = context.WithCancel(ctx)
	} else {
		cctx, cancel = context.WithTimeout(ctx, lib.JSToGoDuration(msg.Timeout))
	}

	// If this message is for an internal service, do a short-circuit dispatch here.
	if msg.Name == "controller" {
		c.handleInternalCall(ctx, &msg, decoder, w, span)
		return
	}

	inArgs := make([]interface{}, msg.NumInArgs)
	for i := range inArgs {
		if err := decoder.Decode(&inArgs[i]); err != nil {
			w.Error(err)
			return
		}
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
		request.stream = newStream()
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

func (c *Controller) maybeCreateServer(serverId uint32) (*server.Server, error) {
	c.Lock()
	defer c.Unlock()
	if server, ok := c.servers[serverId]; ok {
		return server, nil
	}
	server, err := server.NewServer(serverId, c.listenSpec, c)
	if err != nil {
		return nil, err
	}
	c.servers[serverId] = server
	return server, nil
}

func (c *Controller) removeServer(serverId uint32) {
	c.Lock()
	server := c.servers[serverId]
	if server == nil {
		c.Unlock()
		return
	}
	delete(c.servers, serverId)
	c.Unlock()

	server.Stop()
}

// HandleServeRequest takes a request to serve a server, creates a server,
// registers the provided services and sends true if everything succeeded.
func (c *Controller) Serve(ctx ipc.ServerContext, name string, serverId uint32) error {
	server, err := c.maybeCreateServer(serverId)
	if err != nil {
		return verror.Convert(verror.Internal, nil, err)
	}
	vlog.VI(2).Infof("serving under name: %q", name)
	if err := server.Serve(name); err != nil {
		return verror.Convert(verror.Internal, nil, err)
	}
	return nil
}

// HandleLookupResponse handles the result of a Dispatcher.Lookup call that was
// run by the Javascript server.
func (c *Controller) HandleLookupResponse(id int32, data string) {
	c.Lock()
	server := c.flowMap[id]
	c.Unlock()
	if server == nil {
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
	server := c.flowMap[id]
	c.Unlock()
	if server == nil {
		vlog.Errorf("unexpected result from JavaScript. No channel "+
			"for MessageId: %d exists. Ignoring the results.", id)
		//Ignore unknown responses that don't belong to any channel
		return
	}
	server.HandleAuthResponse(id, data)
}

// HandleStopRequest takes a request to stop a server.
func (c *Controller) HandleStopRequest(data string, w lib.ClientWriter) {
	var serverId uint32
	if err := json.Unmarshal([]byte(data), &serverId); err != nil {
		w.Error(verror.Convert(verror.Internal, nil, err))
		return
	}

	c.removeServer(serverId)

	// Send true to indicate stop has finished
	if err := w.Send(lib.ResponseFinal, true); err != nil {
		w.Error(verror.Convert(verror.Internal, nil, err))
		return
	}
}

// HandleAddNameRequest takes a request to add a new name to a server
func (c *Controller) HandleAddNameRequest(data string, w lib.ClientWriter) {
	var request addRemoveNameRequest
	if err := json.Unmarshal([]byte(data), &request); err != nil {
		w.Error(verror.Convert(verror.Internal, nil, err))
		return
	}

	// Create a server for the pipe, if it does not exist already
	server, err := c.maybeCreateServer(request.ServerId)
	if err != nil {
		w.Error(verror.Convert(verror.Internal, nil, err))
		return
	}

	// Add name
	if err := server.AddName(request.Name); err != nil {
		w.Error(verror.Convert(verror.Internal, nil, err))
		return
	}

	// Send true to indicate request has finished without error
	if err := w.Send(lib.ResponseFinal, true); err != nil {
		w.Error(verror.Convert(verror.Internal, nil, err))
		return
	}
}

// HandleRemoveNameRequest takes a request to remove a name from a server
func (c *Controller) HandleRemoveNameRequest(data string, w lib.ClientWriter) {
	var request addRemoveNameRequest
	if err := json.Unmarshal([]byte(data), &request); err != nil {
		w.Error(verror.Convert(verror.Internal, nil, err))
		return
	}

	// Create a server for the pipe, if it does not exist already
	server, err := c.maybeCreateServer(request.ServerId)
	if err != nil {
		w.Error(verror.Convert(verror.Internal, nil, err))
		return
	}

	// Remove name
	server.RemoveName(request.Name)

	// Remove name from signature cache as well
	c.signatureManager.FlushCacheEntry(request.Name)

	// Send true to indicate request has finished without error
	if err := w.Send(lib.ResponseFinal, true); err != nil {
		w.Error(verror.Convert(verror.Internal, nil, err))
		return
	}
}

// HandleServerResponse handles the completion of outstanding calls to JavaScript services
// by filling the corresponding channel with the result from JavaScript.
func (c *Controller) HandleServerResponse(id int32, data string) {
	c.Lock()
	server := c.flowMap[id]
	c.Unlock()
	if server == nil {
		vlog.Errorf("unexpected result from JavaScript. No channel "+
			"for MessageId: %d exists. Ignoring the results.", id)
		//Ignore unknown responses that don't belong to any channel
		return
	}
	server.HandleServerResponse(id, data)
}

// parseVeyronRequest parses a json rpc request into a VeyronRPCRequest object.
func (c *Controller) parseVeyronRequest(data string) (*VeyronRPCRequest, error) {
	var msg VeyronRPCRequest
	if err := lib.VomDecode(data, &msg); err != nil {
		return nil, err
	}
	vlog.VI(2).Infof("VeyronRPCRequest: %s.%s(..., streaming=%v)", msg.Name, msg.Method, msg.IsStreaming)
	return &msg, nil
}

type signatureRequest struct {
	Name string
}

func (c *Controller) getSignature(ctx *context.T, name string) ([]signature.Interface, error) {
	retryTimeoutOpt := options.RetryTimeout(time.Duration(*retryTimeout) * time.Second)
	return c.signatureManager.Signature(ctx, name, retryTimeoutOpt)
}

// HandleSignatureRequest uses signature manager to get and cache signature of a remote server
func (c *Controller) HandleSignatureRequest(ctx *context.T, data string, w lib.ClientWriter) {
	// Decode the request
	var request signatureRequest
	if err := json.Unmarshal([]byte(data), &request); err != nil {
		w.Error(verror.Convert(verror.Internal, ctx, err))
		return
	}

	vlog.VI(2).Infof("requesting Signature for %q", request.Name)
	sig, err := c.getSignature(ctx, request.Name)
	if err != nil {
		w.Error(err)
		return
	}

	vomSig, err := lib.VomEncode(sig)
	if err != nil {
		w.Error(err)
		return
	}
	// Send the signature back
	if err := w.Send(lib.ResponseFinal, vomSig); err != nil {
		w.Error(verror.Convert(verror.Internal, ctx, err))
		return
	}
}

// HandleUnlinkJSBlessings removes the specified blessings from the JS blessings
// store.  'data' should be a JSON encoded number (representing the blessings handle).
func (c *Controller) HandleUnlinkJSBlessings(data string, w lib.ClientWriter) {
	var handle int32
	if err := json.Unmarshal([]byte(data), &handle); err != nil {
		w.Error(verror.Convert(verror.Internal, nil, err))
		return
	}
	c.blessingsStore.Remove(handle)
}

func (c *Controller) getBlessingsHandle(handle int32) (*principal.BlessingsHandle, error) {
	id := c.blessingsStore.Get(handle)
	if id == nil {
		return nil, verror.New(unknownBlessings, nil)
	}
	return principal.ConvertBlessingsToHandle(id, handle), nil
}

func (c *Controller) blessPublicKey(request BlessingRequest) (*principal.BlessingsHandle, error) {
	var blessee security.Blessings
	if blessee = c.blessingsStore.Get(request.Handle); blessee == nil {
		return nil, verror.New(invalidBlessingsHandle, nil)
	}

	expiryCav, err := security.ExpiryCaveat(time.Now().Add(time.Duration(request.DurationMs) * time.Millisecond))
	if err != nil {
		return nil, err
	}
	caveats := append(request.Caveats, expiryCav)

	// TODO(ataly, ashankar, bjornick): Currently the Bless operation is carried
	// out using the Default blessing in this principal's blessings store. We
	// should change this so that the JS blessing request can also specify the
	// blessing to be used for the Bless operation.
	p := veyron2.GetPrincipal(c.ctx)
	blessings, err := p.Bless(blessee.PublicKey(), p.BlessingStore().Default(), request.Extension, caveats[0], caveats[1:]...)
	if err != nil {
		return nil, err
	}

	return principal.ConvertBlessingsToHandle(blessings, c.blessingsStore.Add(blessings)), nil
}

// HandleBlessPublicKey handles a blessing request from JS.
func (c *Controller) HandleBlessPublicKey(data string, w lib.ClientWriter) {
	var request BlessingRequest
	if err := lib.VomDecode(data, &request); err != nil {
		w.Error(verror.Convert(verror.Internal, nil, err))
		return
	}

	handle, err := c.blessPublicKey(request)
	if err != nil {
		w.Error(verror.Convert(verror.Internal, nil, err))
		return
	}

	// Send the id back.
	if err := w.Send(lib.ResponseFinal, handle); err != nil {
		w.Error(verror.Convert(verror.Internal, nil, err))
		return
	}
}

func (c *Controller) HandleCreateBlessings(data string, w lib.ClientWriter) {
	var extension string
	if err := json.Unmarshal([]byte(data), &extension); err != nil {
		w.Error(verror.Convert(verror.Internal, nil, err))
		return
	}
	p, err := vsecurity.NewPrincipal()
	if err != nil {
		w.Error(verror.Convert(verror.Internal, nil, err))
		return
	}

	blessings, err := p.BlessSelf(extension)
	if err != nil {
		w.Error(verror.Convert(verror.Internal, nil, err))
		return
	}
	handle := principal.ConvertBlessingsToHandle(blessings, c.blessingsStore.Add(blessings))
	if err := w.Send(lib.ResponseFinal, handle); err != nil {
		w.Error(verror.Convert(verror.Internal, nil, err))
		return
	}
}

type remoteBlessingsRequest struct {
	Name   string
	Method string
}

func (c *Controller) getRemoteBlessings(ctx *context.T, name, method string) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	call, err := veyron2.GetClient(ctx).StartCall(ctx, name, method, nil)
	if err != nil {
		return nil, err
	}

	blessings, _ := call.RemoteBlessings()
	return blessings, nil
}

func (c *Controller) HandleRemoteBlessingsRequest(ctx *context.T, data string, w lib.ClientWriter) {
	var request remoteBlessingsRequest
	if err := json.Unmarshal([]byte(data), &request); err != nil {
		w.Error(verror.Convert(verror.Internal, ctx, err))
		return
	}

	vlog.VI(2).Infof("requesting remote blessings for %q", request.Name)
	blessings, err := c.getRemoteBlessings(ctx, request.Name, request.Method)
	if err != nil {
		w.Error(verror.Convert(verror.Internal, ctx, err))
		return
	}

	vomRemoteBlessings, err := lib.VomEncode(blessings)
	if err != nil {
		w.Error(err)
		return
	}

	if err := w.Send(lib.ResponseFinal, vomRemoteBlessings); err != nil {
		w.Error(verror.Convert(verror.Internal, ctx, err))
		return
	}
}

// HandleNamespaceRequest uses the namespace client to respond to namespace specific requests such as glob
func (c *Controller) HandleNamespaceRequest(ctx *context.T, data string, w lib.ClientWriter) {
	namespace.HandleRequest(ctx, data, w)
}
