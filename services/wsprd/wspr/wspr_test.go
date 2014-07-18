package wspr

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"
	"veyron/services/wsprd/ipc/client"
	"veyron/services/wsprd/lib"
	"veyron/services/wsprd/signature"
	"veyron2"
	"veyron2/ipc"
	"veyron2/naming"
	"veyron2/rt"
	"veyron2/vdl/vdlutil"
	"veyron2/verror"
	"veyron2/vlog"
	"veyron2/vom"
	vom_wiretype "veyron2/vom/wiretype"
	"veyron2/wiretype"

	"veyron/runtimes/google/ipc/stream/proxy"
	mounttable "veyron/services/mounttable/lib"
)

var r veyron2.Runtime

func init() {
	r = rt.Init()
}

type simpleAdder struct{}

func (s simpleAdder) Add(_ ipc.ServerCall, a, b int32) (int32, error) {
	return a + b, nil
}

func (s simpleAdder) Divide(_ ipc.ServerCall, a, b int32) (int32, error) {
	if b == 0 {
		return 0, verror.BadArgf("can't divide by zero")
	}
	return a / b, nil
}

func (s simpleAdder) StreamingAdd(call ipc.ServerCall) (int32, error) {
	total := int32(0)
	var value int32
	for err := call.Recv(&value); err == nil; err = call.Recv(&value) {
		total += value
		call.Send(total)
	}
	return total, nil
}

func (s simpleAdder) Signature(call ipc.ServerCall) (ipc.ServiceSignature, error) {
	result := ipc.ServiceSignature{Methods: make(map[string]ipc.MethodSignature)}
	result.Methods["Add"] = ipc.MethodSignature{
		InArgs: []ipc.MethodArgument{
			{Name: "A", Type: 36},
			{Name: "B", Type: 36},
		},
		OutArgs: []ipc.MethodArgument{
			{Name: "Value", Type: 36},
			{Name: "Err", Type: 65},
		},
	}

	result.Methods["Divide"] = ipc.MethodSignature{
		InArgs: []ipc.MethodArgument{
			{Name: "A", Type: 36},
			{Name: "B", Type: 36},
		},
		OutArgs: []ipc.MethodArgument{
			{Name: "Value", Type: 36},
			{Name: "Err", Type: 65},
		},
	}

	result.Methods["StreamingAdd"] = ipc.MethodSignature{
		InArgs: []ipc.MethodArgument{},
		OutArgs: []ipc.MethodArgument{
			{Name: "Value", Type: 36},
			{Name: "Err", Type: 65},
		},
		InStream:  36,
		OutStream: 36,
	}
	result.TypeDefs = []vdlutil.Any{
		wiretype.NamedPrimitiveType{Type: 0x1, Name: "error", Tags: []string(nil)}}

	return result, nil
}

func startAnyServer(servesMT bool, dispatcher ipc.Dispatcher) (ipc.Server, naming.Endpoint, error) {
	// Create a new server instance.
	s, err := r.NewServer(veyron2.ServesMountTableOpt(servesMT))
	if err != nil {
		return nil, nil, err
	}

	endpoint, err := s.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, nil, err
	}

	if err := s.Serve("", dispatcher); err != nil {
		return nil, nil, err
	}
	return s, endpoint, nil
}

func startAdderServer() (ipc.Server, naming.Endpoint, error) {
	return startAnyServer(false, ipc.SoloDispatcher(simpleAdder{}, nil))
}

func startProxy() (*proxy.Proxy, error) {
	rid, err := naming.NewRoutingID()
	if err != nil {
		return nil, err
	}
	return proxy.New(rid, nil, "tcp", "127.0.0.1:0", "")
}

func startMountTableServer() (ipc.Server, naming.Endpoint, error) {
	mt, err := mounttable.NewMountTable("")
	if err != nil {
		return nil, nil, err
	}
	return startAnyServer(true, mt)
}

type testWriter struct {
	sync.Mutex
	stream []response
	err    error
	logger vlog.Logger
	// If this channel is set then a message will be sent
	// to this channel after recieving a call to FinishMessage()
	notifier chan bool
}

func (w *testWriter) Send(responseType lib.ResponseType, msg interface{}) error {
	w.Lock()
	defer w.Unlock()
	// We serialize and deserialize the reponse so that we can do deep equal with
	// messages that contain non-exported structs.
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response{Type: responseType, Message: msg}); err != nil {
		return err
	}

	var r response

	if err := json.NewDecoder(&buf).Decode(&r); err != nil {
		return err
	}

	w.stream = append(w.stream, r)
	if w.notifier != nil {
		w.notifier <- true
	}
	return nil

}

func (w *testWriter) Error(err error) {
	w.err = err
}

func (w *testWriter) streamLength() int {
	w.Lock()
	defer w.Unlock()
	return len(w.stream)
}

// Waits until there is at least n messages in w.stream. Returns an error if we timeout
// waiting for the given number of messages.
func (w *testWriter) waitForMessage(n int) error {
	if w.streamLength() >= n {
		return nil
	}
	w.Lock()
	w.notifier = make(chan bool, 1)
	w.Unlock()
	for w.streamLength() < n {
		select {
		case <-w.notifier:
			continue
		case <-time.After(time.Second):
			return fmt.Errorf("timed out")
		}
	}
	w.Lock()
	w.notifier = nil
	w.Unlock()
	return nil
}

func checkResponses(w *testWriter, expectedStream []response, err error, t *testing.T) {
	if !reflect.DeepEqual(expectedStream, w.stream) {
		t.Errorf("streams don't match: expected %v, got %v", expectedStream, w.stream)
	}

	if !reflect.DeepEqual(err, w.err) {
		t.Errorf("unexpected error, got: %v, expected: %v", err, w.err)
	}
}

var adderServiceSignature signature.JSONServiceSignature = signature.JSONServiceSignature{
	"add": signature.JSONMethodSignature{
		InArgs:      []string{"A", "B"},
		NumOutArgs:  2,
		IsStreaming: false,
	},
	"divide": signature.JSONMethodSignature{
		InArgs:      []string{"A", "B"},
		NumOutArgs:  2,
		IsStreaming: false,
	},
	"streamingAdd": signature.JSONMethodSignature{
		InArgs:      []string{},
		NumOutArgs:  2,
		IsStreaming: true,
	},
}

func TestGetGoServerSignature(t *testing.T) {
	s, endpoint, err := startAdderServer()
	if err != nil {
		t.Errorf("unable to start server: %v", err)
		t.Fail()
		return
	}
	defer s.Stop()
	wspr := NewWSPR(0, "mockVeyronProxyEP")
	wspr.setup()
	wsp := websocketPipe{ctx: wspr}
	wsp.setup()
	jsSig, err := wsp.getSignature("/"+endpoint.String(), "")
	if err != nil {
		t.Errorf("Failed to get signature: %v", err)
	}

	if !reflect.DeepEqual(jsSig, adderServiceSignature) {
		t.Errorf("Unexpected signature, got :%v, expected: %v", jsSig, adderServiceSignature)
	}
}

type goServerTestCase struct {
	method             string
	inArgs             []interface{}
	numOutArgs         int32
	streamingInputs    []string
	streamingInputType vom.Type
	expectedStream     []response
	expectedError      error
}

func runGoServerTestCase(t *testing.T, test goServerTestCase) {
	s, endpoint, err := startAdderServer()
	if err != nil {
		t.Errorf("unable to start server: %v", err)
		t.Fail()
		return
	}
	defer s.Stop()

	wspr := NewWSPR(0, "mockVeyronProxyEP")
	wspr.setup()
	wsp := websocketPipe{ctx: wspr}
	wsp.setup()
	writer := testWriter{
		logger: wspr.logger,
	}

	var signal chan ipc.Stream
	if len(test.streamingInputs) > 0 {
		signal = make(chan ipc.Stream, 1)
		wsp.outstandingStreams[0] = outstandingStream{
			stream: client.StartQueueingStream(signal),
			inType: test.streamingInputType,
		}
		go func() {
			for _, value := range test.streamingInputs {
				wsp.sendOnStream(0, value, &writer)
			}
			wsp.closeStream(0)
		}()
	}

	request := veyronRPC{
		Name:        "/" + endpoint.String(),
		Method:      test.method,
		InArgs:      test.inArgs,
		NumOutArgs:  test.numOutArgs,
		IsStreaming: signal != nil,
	}
	wsp.sendVeyronRequest(0, &request, &writer, signal)

	checkResponses(&writer, test.expectedStream, test.expectedError, t)
}

func TestCallingGoServer(t *testing.T) {
	runGoServerTestCase(t, goServerTestCase{
		method:     "Add",
		inArgs:     []interface{}{2, 3},
		numOutArgs: 2,
		expectedStream: []response{
			response{
				Message: []interface{}{5.0},
				Type:    lib.ResponseFinal,
			},
		},
	})
}

func TestCallingGoServerWithError(t *testing.T) {
	runGoServerTestCase(t, goServerTestCase{
		method:        "Divide",
		inArgs:        []interface{}{1, 0},
		numOutArgs:    2,
		expectedError: verror.BadArgf("can't divide by zero"),
	})
}

func TestCallingGoWithStreaming(t *testing.T) {
	runGoServerTestCase(t, goServerTestCase{
		method:             "StreamingAdd",
		inArgs:             []interface{}{},
		streamingInputs:    []string{"1", "2", "3", "4"},
		streamingInputType: vom_wiretype.Type{ID: 36},
		numOutArgs:         2,
		expectedStream: []response{
			response{
				Message: 1.0,
				Type:    lib.ResponseStream,
			},
			response{
				Message: 3.0,
				Type:    lib.ResponseStream,
			},
			response{
				Message: 6.0,
				Type:    lib.ResponseStream,
			},
			response{
				Message: 10.0,
				Type:    lib.ResponseStream,
			},
			response{
				Message: nil,
				Type:    lib.ResponseStreamClose,
			},
			response{
				Message: []interface{}{10.0},
				Type:    lib.ResponseFinal,
			},
		},
	})
}

type runningTest struct {
	wspr             *WSPR
	wsp              *websocketPipe
	writer           *testWriter
	mounttableServer ipc.Server
	proxyServer      *proxy.Proxy
}

func serveServer() (*runningTest, error) {
	mounttableServer, endpoint, err := startMountTableServer()

	if err != nil {
		return nil, fmt.Errorf("unable to start mounttable: %v", err)
	}

	proxyServer, err := startProxy()

	if err != nil {
		return nil, fmt.Errorf("unable to start proxy: %v", err)
	}

	proxyEndpoint := proxyServer.Endpoint().String()

	wspr := NewWSPR(0, "/"+proxyEndpoint, veyron2.NamespaceRoots{"/" + endpoint.String()})
	wspr.setup()
	wsp := websocketPipe{ctx: wspr}
	writer := testWriter{
		logger: wspr.logger,
	}
	wsp.writerCreator = func(int64) lib.ClientWriter {
		return &writer
	}
	wsp.setup()
	wsp.serve(serveRequest{
		Name:    "adder",
		Service: adderServiceSignature,
	}, &writer)

	return &runningTest{
		wspr, &wsp, &writer, mounttableServer, proxyServer,
	}, nil
}

func TestJavascriptServeServer(t *testing.T) {
	rt, err := serveServer()
	defer rt.mounttableServer.Stop()
	defer rt.proxyServer.Shutdown()
	defer rt.wsp.cleanup()
	if err != nil {
		t.Fatalf("could not serve server %v", err)
	}

	if len(rt.writer.stream) != 1 {
		t.Errorf("expected only one response, got %d", len(rt.writer.stream))
		return
	}

	resp := rt.writer.stream[0]

	if resp.Type != lib.ResponseFinal {
		t.Errorf("unknown stream message Got: %v, expected: serve response", resp)
		return
	}

	if msg, ok := resp.Message.(string); ok {
		if _, err := r.NewEndpoint(msg); err == nil {
			return
		}
	}
	t.Errorf("invalid endpdoint returned from serve: %v", resp.Message)
}

func TestJavascriptStopServer(t *testing.T) {
	rt, err := serveServer()
	defer rt.mounttableServer.Stop()
	defer rt.proxyServer.Shutdown()
	defer rt.wsp.cleanup()

	if err != nil {
		t.Errorf("could not serve server %v", err)
		return
	}

	// ensure there is only one server and then stop the server
	if len(rt.wsp.servers) != 1 {
		t.Errorf("expected only one server but got: %d", len(rt.wsp.servers))
		return
	}
	for serverId := range rt.wsp.servers {
		rt.wsp.removeServer(serverId)
	}

	// ensure there is no more servers now
	if len(rt.wsp.servers) != 0 {
		t.Errorf("expected no server after stopping the only one but got: %d", len(rt.wsp.servers))
		return
	}

	return
}

type jsServerTestCase struct {
	method string
	inArgs []interface{}
	// The set of streaming inputs from the client to the server.
	clientStream []interface{}
	// The set of streaming outputs from the server to the client.
	serverStream  []interface{}
	finalResponse interface{}
	err           *verror.Standard
}

func sendServerStream(t *testing.T, wsp *websocketPipe, test *jsServerTestCase, w lib.ClientWriter) {
	for _, msg := range test.serverStream {
		wsp.sendParsedMessageOnStream(0, msg, w)
	}

	serverReply := map[string]interface{}{
		"Results": []interface{}{test.finalResponse},
		"Err":     test.err,
	}

	bytes, err := json.Marshal(serverReply)
	if err != nil {
		t.Fatalf("Failed to serialize the reply: %v", err)
	}
	wsp.handleServerResponse(0, string(bytes))
}

func runJsServerTestCase(t *testing.T, test jsServerTestCase) {
	rt, err := serveServer()
	defer rt.mounttableServer.Stop()
	defer rt.proxyServer.Shutdown()
	defer rt.wsp.cleanup()

	if err != nil {
		t.Errorf("could not serve server %v", err)
	}

	if len(rt.writer.stream) != 1 {
		t.Errorf("expected only on response, got %d", len(rt.writer.stream))
		return
	}

	resp := rt.writer.stream[0]

	if resp.Type != lib.ResponseFinal {
		t.Errorf("unknown stream message Got: %v, expected: serve response", resp)
		return
	}

	msg, ok := resp.Message.(string)
	if !ok {
		t.Errorf("invalid endpdoint returned from serve: %v", resp.Message)
	}

	if _, err := r.NewEndpoint(msg); err != nil {
		t.Errorf("invalid endpdoint returned from serve: %v", resp.Message)
	}

	rt.writer.stream = nil

	// Create a client using wspr's runtime so it points to the right mounttable.
	client, err := rt.wspr.rt.NewClient()

	if err != nil {
		t.Errorf("unable to create client: %v", err)
	}

	call, err := client.StartCall(rt.wspr.rt.NewContext(), "/"+msg+"/adder", test.method, test.inArgs)
	if err != nil {
		t.Errorf("failed to start call: %v", err)
	}

	expectedWebsocketMessage := []response{
		response{
			Type: lib.ResponseServerRequest,
			Message: map[string]interface{}{
				"ServerId": 0.0,
				"Method":   lib.LowercaseFirstCharacter(test.method),
				"Args":     test.inArgs,
				"Context": map[string]interface{}{
					"Name":   "adder",
					"Suffix": "adder",
				},
			},
		},
	}

	// Wait until the rpc has started.
	if err := rt.writer.waitForMessage(len(expectedWebsocketMessage)); err != nil {
		t.Errorf("didn't recieve expected message: %v", err)
	}
	for _, msg := range test.clientStream {
		expectedWebsocketMessage = append(expectedWebsocketMessage, response{Type: lib.ResponseStream, Message: msg})
		if err := call.Send(msg); err != nil {
			t.Errorf("unexpected error while sending %v: %v", msg, err)
		}
	}

	// Wait until all the streaming messages have been acknowledged.
	if err := rt.writer.waitForMessage(len(expectedWebsocketMessage)); err != nil {
		t.Errorf("didn't recieve expected message: %v", err)
	}

	expectedWebsocketMessage = append(expectedWebsocketMessage, response{Type: lib.ResponseStreamClose})

	expectedStream := test.serverStream
	go sendServerStream(t, rt.wsp, &test, rt.writer)
	for {
		var data interface{}
		if err := call.Recv(&data); err != nil {
			break
		}
		if len(expectedStream) == 0 {
			t.Errorf("unexpected stream value: %v", data)
			continue
		}
		if !reflect.DeepEqual(data, expectedStream[0]) {
			t.Errorf("unexpected stream value: got %v, expected %v", data, expectedStream[0])
		}
		expectedStream = expectedStream[1:]
	}
	var result interface{}
	var err2 error

	if err := call.Finish(&result, &err2); err != nil {
		t.Errorf("unexpected err :%v", err)
	}

	if !reflect.DeepEqual(result, test.finalResponse) {
		t.Errorf("unexected final response: got %v, expected %v", result, test.finalResponse)
	}

	// If err2 is nil and test.err is nil reflect.DeepEqual will return false because the
	// types are different.  Because of this, we only use reflect.DeepEqual if one of
	// the values is non-nil.  If both values are nil, then we consider them equal.
	if (err2 != nil || test.err != nil) && !reflect.DeepEqual(err2, test.err) {
		t.Errorf("unexected error: got %v, expected %v", err2, test.err)
	}

	// Wait until the close streaming messages have been acknowledged.
	if err := rt.writer.waitForMessage(len(expectedWebsocketMessage)); err != nil {
		t.Errorf("didn't recieve expected message: %v", err)
	}

	checkResponses(rt.writer, expectedWebsocketMessage, nil, t)
}

func TestSimpleJSServer(t *testing.T) {
	runJsServerTestCase(t, jsServerTestCase{
		method:        "Add",
		inArgs:        []interface{}{1.0, 2.0},
		finalResponse: 3.0,
	})
}

func TestJSServerWithError(t *testing.T) {
	runJsServerTestCase(t, jsServerTestCase{
		method:        "Add",
		inArgs:        []interface{}{1.0, 2.0},
		finalResponse: 3.0,
		err: &verror.Standard{
			ID:  verror.Internal,
			Msg: "JS Server failed",
		},
	})
}

func TestJSServerWihStreamingInputs(t *testing.T) {
	runJsServerTestCase(t, jsServerTestCase{
		method:        "StreamingAdd",
		inArgs:        []interface{}{},
		clientStream:  []interface{}{3.0, 4.0},
		finalResponse: 10.0,
	})
}

func TestJSServerWihStreamingOutputs(t *testing.T) {
	runJsServerTestCase(t, jsServerTestCase{
		method:        "StreamingAdd",
		inArgs:        []interface{}{},
		serverStream:  []interface{}{3.0, 4.0},
		finalResponse: 10.0,
	})
}

func TestJSServerWihStreamingInputsAndOutputs(t *testing.T) {
	runJsServerTestCase(t, jsServerTestCase{
		method:        "StreamingAdd",
		inArgs:        []interface{}{},
		clientStream:  []interface{}{1.0, 2.0},
		serverStream:  []interface{}{3.0, 4.0},
		finalResponse: 10.0,
	})
}

// TODO(bjornick): Make sure that send on stream is nonblocking
