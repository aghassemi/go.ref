package browspr

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"v.io/core/veyron2"
	"v.io/core/veyron2/context"
	"v.io/core/veyron2/ipc"
	"v.io/core/veyron2/naming"
	"v.io/core/veyron2/options"
	"v.io/core/veyron2/vom"

	"v.io/core/veyron/lib/testutil"
	_ "v.io/core/veyron/profiles"
	"v.io/core/veyron/runtimes/google/ipc/stream/proxy"
	mounttable "v.io/core/veyron/services/mounttable/lib"
	"v.io/wspr/veyron/services/wsprd/app"
	"v.io/wspr/veyron/services/wsprd/lib"
)

func startProxy() (*proxy.Proxy, error) {
	rid, err := naming.NewRoutingID()
	if err != nil {
		return nil, err
	}
	return proxy.New(rid, nil, "tcp", "127.0.0.1:0", "")
}

func startMounttable(ctx *context.T) (ipc.Server, naming.Endpoint, error) {
	mt, err := mounttable.NewMountTableDispatcher("")
	if err != nil {
		return nil, nil, err
	}

	s, err := veyron2.NewServer(ctx, options.ServesMountTable(true))
	if err != nil {
		return nil, nil, err
	}

	endpoints, err := s.Listen(veyron2.GetListenSpec(ctx))
	if err != nil {
		return nil, nil, err
	}

	if err := s.ServeDispatcher("", mt); err != nil {
		return nil, nil, err
	}

	return s, endpoints[0], nil
}

type mockServer struct{}

func (s mockServer) BasicCall(_ ipc.ServerCall, txt string) (string, error) {
	return "[" + txt + "]", nil
}

func startMockServer(ctx *context.T, desiredName string) (ipc.Server, naming.Endpoint, error) {
	// Create a new server instance.
	s, err := veyron2.NewServer(ctx)
	if err != nil {
		return nil, nil, err
	}

	endpoints, err := s.Listen(veyron2.GetListenSpec(ctx))
	if err != nil {
		return nil, nil, err
	}

	if err := s.Serve(desiredName, mockServer{}, nil); err != nil {
		return nil, nil, err
	}

	return s, endpoints[0], nil
}

func TestBrowspr(t *testing.T) {
	ctx, shutdown := testutil.InitForTest()
	defer shutdown()

	proxy, err := startProxy()
	if err != nil {
		t.Fatalf("Failed to start proxy: %v", err)
	}
	defer proxy.Shutdown()

	mtServer, mtEndpoint, err := startMounttable(ctx)
	if err != nil {
		t.Fatalf("Failed to start mounttable server: %v", err)
	}
	defer mtServer.Stop()
	root := mtEndpoint.Name()
	if err := veyron2.GetNamespace(ctx).SetRoots(root); err != nil {
		t.Fatalf("Failed to set namespace roots: %v", err)
	}

	mockServerName := "mock/server"
	mockServer, mockServerEndpoint, err := startMockServer(ctx, mockServerName)
	if err != nil {
		t.Fatalf("Failed to start mock server: %v", err)
	}
	defer mockServer.Stop()

	then := time.Now()
found:
	for {
		status := mockServer.Status()
		for _, v := range status.Mounts {
			if v.Name == mockServerName && v.Server == mockServerEndpoint.String() && !v.LastMount.IsZero() {
				if v.LastMountErr != nil {
					t.Fatalf("Failed to mount %s: %v", v.Name, v.LastMountErr)
				}
				break found
			}
		}
		if time.Now().Sub(then) > time.Minute {
			t.Fatalf("Failed to find mounted server and endpoint: %v: %v", mockServerName, mtEndpoint)
		}
		time.Sleep(100 * time.Millisecond)
	}
	mountEntry, err := veyron2.GetNamespace(ctx).Resolve(ctx, mockServerName)
	if err != nil {
		t.Fatalf("Error fetching published names from mounttable: %v", err)
	}

	servers := []string{}
	for _, s := range mountEntry.Servers {
		if strings.Index(s.Server, "@tcp") != -1 {
			servers = append(servers, s.Server)
		}
	}
	if len(servers) != 1 || servers[0] != mockServerEndpoint.String() {
		t.Fatalf("Incorrect names retrieved from mounttable: %v", mountEntry)
	}

	spec := veyron2.GetListenSpec(ctx)
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

	veyron2.GetNamespace(ctx).SetRoots(root)
	browspr := NewBrowspr(ctx, postMessageHandler, &spec, "/mock:1234/identd", []string{root})

	// browspr sets its namespace root to use the "ws" protocol, but we want to force "tcp" here.
	browspr.namespaceRoots = []string{root}

	browspr.accountManager.SetMockBlesser(newMockBlesserService(veyron2.GetPrincipal(ctx)))

	msgInstanceId := int32(11)
	msgOrigin := "http://test-origin.com"

	// Associate the origin with the root accounts' blessings, otherwise a
	// dummy account will be used and will be rejected by the authorizer.
	accountName := "test-account"
	bp := veyron2.GetPrincipal(browspr.ctx)
	if err := browspr.principalManager.AddAccount(accountName, bp.BlessingStore().Default()); err != nil {
		t.Fatalf("Failed to add account: %v")
	}
	if err := browspr.accountManager.AssociateAccount(msgOrigin, accountName, nil); err != nil {
		t.Fatalf("Failed to associate account: %v")
	}

	rpc := app.VeyronRPCRequest{
		Name:        mockServerName,
		Method:      "BasicCall",
		NumInArgs:   1,
		NumOutArgs:  1,
		IsStreaming: false,
		Timeout:     (1 << 31) - 1,
	}

	var buf bytes.Buffer
	encoder, err := vom.NewBinaryEncoder(&buf)
	if err != nil {
		t.Fatalf("Failed to vom encode rpc message: %v", err)
	}
	if err := encoder.Encode(rpc); err != nil {
		t.Fatalf("Failed to vom encode rpc message: %v", err)
	}
	if err := encoder.Encode("InputValue"); err != nil {
		t.Fatalf("Failed to vom encode rpc message: %v", err)
	}
	vomRPC := hex.EncodeToString(buf.Bytes())

	msg, err := json.Marshal(app.Message{
		Id:   1,
		Data: vomRPC,
		Type: app.VeyronRequestMessage,
	})
	if err != nil {
		t.Fatalf("Failed to marshall app message to json: %v", err)
	}

	err = browspr.HandleMessage(msgInstanceId, msgOrigin, string(msg))
	if err != nil {
		t.Fatalf("Error while handling message: %v", err)
	}

	<-receivedResponse

	if receivedInstanceId != msgInstanceId {
		t.Errorf("Received unexpected instance id: %d. Expected: %d", receivedInstanceId, msgInstanceId)
	}
	if receivedType != "browsprMsg" {
		t.Errorf("Received unexpected response type. Expected: %q, but got %q", "browsprMsg", receivedType)
	}

	var outMsg app.Message
	if err := lib.VomDecode(receivedMsg, &outMsg); err != nil {
		t.Fatalf("Failed to unmarshall outgoing message: %v", err)
	}
	if outMsg.Id != int32(1) {
		t.Errorf("Id was %v, expected %v", outMsg.Id, int32(1))
	}
	if outMsg.Type != app.VeyronRequestMessage {
		t.Errorf("Message type was %v, expected %v", outMsg.Type, app.MessageType(0))
	}

	var responseMsg lib.Response
	if err := lib.VomDecode(outMsg.Data, &responseMsg); err != nil {
		t.Fatalf("Failed to unmarshall outgoing response: %v", err)
	}
	if responseMsg.Type != lib.ResponseFinal {
		t.Errorf("Data was %q, expected %q", outMsg.Data, `["[InputValue]"]`)
	}
	var outArg string
	var ok bool
	if outArg, ok = responseMsg.Message.(string); !ok {
		t.Errorf("Got unexpected response message body of type %T, expected type string", responseMsg.Message)
	}
	var result app.VeyronRPCResponse
	if err := lib.VomDecode(outArg, &result); err != nil {
		t.Errorf("Failed to vom decode args from %v: %v", outArg, err)
	}
	if got, want := result.OutArgs[0], "[InputValue]"; !reflect.DeepEqual(got, want) {
		t.Errorf("Result got %v, want %v", got, want)
	}
}
