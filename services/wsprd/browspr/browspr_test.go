package browspr

import (
	"encoding/json"
	"testing"

	"veyron.io/veyron/veyron/profiles"
	"veyron.io/veyron/veyron/runtimes/google/ipc/stream/proxy"
	mounttable "veyron.io/veyron/veyron/services/mounttable/lib"
	"veyron.io/veyron/veyron2/ipc"
	"veyron.io/veyron/veyron2/naming"
	"veyron.io/veyron/veyron2/options"
	"veyron.io/veyron/veyron2/rt"
	"veyron.io/veyron/veyron2/vdl/vdlutil"
	"veyron.io/veyron/veyron2/wiretype"
	"veyron.io/wspr/veyron/services/wsprd/app"
	"veyron.io/wspr/veyron/services/wsprd/lib"
)

var (
	r = rt.Init()
)

func startProxy() (*proxy.Proxy, error) {
	rid, err := naming.NewRoutingID()
	if err != nil {
		return nil, err
	}
	return proxy.New(rid, nil, "tcp", "127.0.0.1:0", "")
}

func startMounttable() (ipc.Server, naming.Endpoint, error) {
	mt, err := mounttable.NewMountTable("")
	if err != nil {
		return nil, nil, err
	}

	s, err := r.NewServer(options.ServesMountTable(true))
	if err != nil {
		return nil, nil, err
	}

	endpoint, err := s.Listen(profiles.LocalListenSpec)
	if err != nil {
		return nil, nil, err
	}

	if err := s.ServeDispatcher("", mt); err != nil {
		return nil, nil, err
	}

	return s, endpoint, nil
}

type mockServer struct{}

func (s mockServer) BasicCall(_ ipc.ServerCall, txt string) (string, error) {
	return "[" + txt + "]", nil
}

func (s mockServer) Signature(call ipc.ServerCall) (ipc.ServiceSignature, error) {
	result := ipc.ServiceSignature{Methods: make(map[string]ipc.MethodSignature)}
	result.Methods["BasicCall"] = ipc.MethodSignature{
		InArgs: []ipc.MethodArgument{
			{Name: "Txt", Type: 3},
		},
		OutArgs: []ipc.MethodArgument{
			{Name: "Value", Type: 3},
			{Name: "Err", Type: 65},
		},
	}
	result.TypeDefs = []vdlutil.Any{
		wiretype.NamedPrimitiveType{Type: 0x1, Name: "error", Tags: []string(nil)}}

	return result, nil
}

func startMockServer(desiredName string) (ipc.Server, naming.Endpoint, error) {
	// Create a new server instance.
	s, err := r.NewServer()
	if err != nil {
		return nil, nil, err
	}

	endpoint, err := s.Listen(profiles.LocalListenSpec)
	if err != nil {
		return nil, nil, err
	}

	if err := s.ServeDispatcher(desiredName, ipc.LeafDispatcher(mockServer{}, nil)); err != nil {
		return nil, nil, err
	}

	return s, endpoint, nil
}

type veyronTempRPC struct {
	Name        string
	Method      string
	InArgs      []json.RawMessage
	NumOutArgs  int32
	IsStreaming bool
	Timeout     int64
}

func TestBrowspr(t *testing.T) {
	proxy, err := startProxy()
	if err != nil {
		t.Fatalf("Failed to start proxy: %v", err)
	}
	defer proxy.Shutdown()

	mtServer, mtEndpoint, err := startMounttable()
	if err != nil {
		t.Fatalf("Failed to start mounttable server: %v", err)
	}
	defer mtServer.Stop()
	if err := r.Namespace().SetRoots("/" + mtEndpoint.String()); err != nil {
		t.Fatalf("Failed to set namespace roots: %v", err)
	}

	mockServerName := "mock/server"
	mockServer, mockServerEndpoint, err := startMockServer(mockServerName)
	if err != nil {
		t.Fatalf("Failed to start mock server: %v", err)
	}
	defer mockServer.Stop()

	names, err := mockServer.Published()
	if err != nil {
		t.Fatalf("Error fetching published names: %v", err)
	}
	if len(names) != 1 || names[0] != "/"+mtEndpoint.String()+"/"+mockServerName {
		t.Fatalf("Incorrectly mounted server. Names: %v", names)
	}
	mountEntry, err := r.Namespace().ResolveX(nil, mockServerName)
	if err != nil {
		t.Fatalf("Error fetching published names from mounttable: %v", err)
	}
	if len(mountEntry.Servers) != 1 || mountEntry.Servers[0].Server != "/"+mockServerEndpoint.String() {
		t.Fatalf("Incorrect names retrieved from mounttable: %v", mountEntry)
	}

	spec := profiles.LocalListenSpec
	spec.Proxy = proxy.Endpoint().String()

	receivedResponse := make(chan bool, 1)
	var receivedInstanceId int32
	var receivedType string
	var receivedMsg string

	var postMessageHandler = func(instanceId int32, ty, msg string) {
		receivedInstanceId = instanceId
		receivedType = ty
		receivedMsg = msg
		receivedResponse <- true
	}

	browspr := NewBrowspr(postMessageHandler, spec, "/mock/identd", []string{"/" + mtEndpoint.String()}, options.RuntimePrincipal{r.Principal()})

	principal := browspr.rt.Principal()
	browspr.accountManager.SetMockBlesser(newMockBlesserService(principal))

	msgInstanceId := int32(11)

	rpcMessage := veyronTempRPC{
		Name:   mockServerName,
		Method: "BasicCall",
		InArgs: []json.RawMessage{
			json.RawMessage([]byte("\"InputValue\"")),
		},
		NumOutArgs:  2,
		IsStreaming: false,
		Timeout:     (1 << 31) - 1,
	}

	jsonRpcMessage, err := json.Marshal(rpcMessage)
	if err != nil {
		t.Fatalf("Failed to marshall rpc message to json: %v", err)
	}

	msg, err := json.Marshal(app.Message{
		Id:   1,
		Data: string(jsonRpcMessage),
		Type: app.VeyronRequestMessage,
	})
	if err != nil {
		t.Fatalf("Failed to marshall app message to json: %v", err)
	}

	err = browspr.HandleMessage(msgInstanceId, string(msg))
	if err != nil {
		t.Fatalf("Error while handling message: %v", err)
	}

	<-receivedResponse

	if receivedInstanceId != msgInstanceId {
		t.Errorf("Received unexpected instance id: %d. Expected: %d", receivedInstanceId, msgInstanceId)
	}
	if receivedType != "msg" {
		t.Errorf("Received unexpected response type. Expected: %q, but got %q", "msg", receivedType)
	}

	var outMsg app.Message
	if err := json.Unmarshal([]byte(receivedMsg), &outMsg); err != nil {
		t.Fatalf("Failed to unmarshall outgoing message: %v", err)
	}
	if outMsg.Id != int64(1) {
		t.Errorf("Id was %v, expected %v", outMsg.Id, int64(1))
	}
	if outMsg.Type != app.VeyronRequestMessage {
		t.Errorf("Message type was %v, expected %v", outMsg.Type, app.MessageType(0))
	}

	var responseMsg app.Response
	if err := json.Unmarshal([]byte(outMsg.Data), &responseMsg); err != nil {
		t.Fatalf("Failed to unmarshall outgoing response: %v", err)
	}
	if responseMsg.Type != lib.ResponseFinal {
		t.Errorf("Data was %q, expected %q", outMsg.Data, `["[InputValue]"]`)
	}
	outArgs := responseMsg.Message.([]interface{})
	if len(outArgs) != 1 || outArgs[0].(string) != "[InputValue]" {
		t.Errorf("Got unexpected response message body: %v", responseMsg.Message)
	}
}
