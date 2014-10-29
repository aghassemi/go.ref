package main

import (
	"bytes"
	"crypto/ecdsa"
	"fmt"
	"runtime/ppapi"
	"syscall"

	"veyron.io/veyron/veyron2/ipc"
	"veyron.io/veyron/veyron2/options"
	"veyron.io/veyron/veyron2/rt"
	"veyron.io/veyron/veyron2/security"
	"veyron.io/veyron/veyron2/vdl"
	"veyron.io/wspr/veyron/services/wsprd/channel/channel_nacl"
	"veyron.io/wspr/veyron/services/wsprd/wspr"

	vsecurity "veyron.io/veyron/veyron/security"
	// TODO(cnicolaou): shouldn't be depending on the runtime here.
	_ "veyron.io/veyron/veyron/profiles"
	_ "veyron.io/veyron/veyron/runtimes/google/security"
)

func main() {
	ppapi.Init(newWsprInstance)
}

// WSPR instance represents an instance on a PPAPI client and receives callbacks from PPAPI to handle events.
type wsprInstance struct {
	ppapi.Instance
	channel *channel_nacl.Channel
}

var _ ppapi.InstanceHandlers = (*wsprInstance)(nil)

func (inst wsprInstance) DidCreate(args map[string]string) bool {
	fmt.Printf("Got to DidCreate")
	return true
}

func (wsprInstance) DidDestroy() {
	fmt.Printf("Got to DidDestroy()")
}

func (wsprInstance) DidChangeView(view ppapi.View) {
	fmt.Printf("Got to DidChangeView(%v)", view)
}

func (wsprInstance) DidChangeFocus(has_focus bool) {
	fmt.Printf("Got to DidChangeFocus(%v)", has_focus)
}

func (wsprInstance) HandleDocumentLoad(url_loader ppapi.Resource) bool {
	fmt.Printf("Got to HandleDocumentLoad(%v)", url_loader)
	return true
}

func (wsprInstance) HandleInputEvent(event ppapi.InputEvent) bool {
	fmt.Printf("Got to HandleInputEvent(%v)", event)
	return true
}

func (wsprInstance) Graphics3DContextLost() {
	fmt.Printf("Got to Graphics3DContextLost()")
}

// startWSPR handles starting WSPR.
func (wsprInstance) startWSPR(message ppapi.Var) {
	// HACK!!
	// TODO(ataly, ashankar, bprosnitz): The private key should be
	// generated/retrieved by directly talking to some secure storage
	// in Chrome, e.g. LocalStorage (and not from the config as below).
	pemKey, err := message.LookupStringValuedKey("pemPrivateKey")
	if err != nil {
		panic(err.Error())
	}

	// TODO(ataly, ashankr, bprosnitz): Figure out whether we need
	// passphrase protection here (most likely we do but how do we
	// request the passphrase from the user?)
	key, err := vsecurity.LoadPEMKey(bytes.NewBufferString(pemKey), nil)
	if err != nil {
		panic(err.Error())
	}
	ecdsaKey, ok := key.(*ecdsa.PrivateKey)
	if !ok {
		panic(fmt.Errorf("got key of type %T, want *ecdsa.PrivateKey", key))
	}

	principal, err := vsecurity.NewPrincipalFromSigner(security.NewInMemoryECDSASigner(ecdsaKey))
	if err != nil {
		panic(err.Error())
	}

	defaultBlessingName, err := message.LookupStringValuedKey("defaultBlessingName")
	if err != nil {
		panic(err.Error())
	}

	if err := vsecurity.InitDefaultBlessings(principal, defaultBlessingName); err != nil {
		panic(err.Error())
	}

	rt.Init(options.RuntimePrincipal{principal})

	veyronProxy, err := message.LookupStringValuedKey("proxyName")
	if err != nil {
		panic(err.Error())
	}
	if veyronProxy == "" {
		panic("Empty proxy")
	}

	mounttable, err := message.LookupStringValuedKey("namespaceRoot")
	if err != nil {
		panic(err.Error())
	}
	syscall.Setenv("MOUNTTABLE_ROOT", mounttable)
	syscall.Setenv("NAMESPACE_ROOT", mounttable)

	identd, err := message.LookupStringValuedKey("identityd")
	if err != nil {
		panic(err.Error())
	}

	wsprHttpPort, err := message.LookupIntValuedKey("wsprHttpPort")
	if err != nil {
		panic(err.Error())
	}

	// TODO(cnicolaou,bprosnitz) Should we use the roaming profile?
	// It uses flags. We should change that.
	listenSpec := ipc.ListenSpec{
		Proxy:    veyronProxy,
		Protocol: "tcp",
		Address:  ":0",
	}

	fmt.Printf("Starting WSPR with config: proxy=%q mounttable=%q identityd=%q port=%d", veyronProxy, mounttable, identd, wsprHttpPort)
	proxy := wspr.NewWSPR(wsprHttpPort, listenSpec, identd)
	go func() {
		proxy.Serve()
	}()
}

func (inst wsprInstance) handleRpcMessage(message ppapi.Var) {
	fmt.Printf("Handle RPC message")
	inst.channel.HandleMessage(message)
}

func TestFuncA(val *vdl.Value) (interface{}, error) {
	if val.RawString() != "ArgA" {
		panic(fmt.Sprintf("vdl val: %v expected: %v", val, "ArgA"))
	}
	return "ResponseA", nil
}

func TestFuncB(val *vdl.Value) (interface{}, error) {
	if val.RawString() != "ArgB" {
		panic(fmt.Sprintf("vdl val: %v expected: %v", val, "ArgB"))
	}
	return "ResponseB", nil
}

func TestFuncC(val *vdl.Value) (interface{}, error) {
	if val.RawString() != "ArgC" {
		panic(fmt.Sprintf("vdl val: %v expected: %v", val, "ArgC"))
	}
	return "", fmt.Errorf("An error")
}

func (inst wsprInstance) startRpcTest(message ppapi.Var) {
	inst.channel.RegisterRpcHandler("TypeA", TestFuncA)
	inst.channel.RegisterRpcHandler("TypeB", TestFuncB)
	inst.channel.RegisterRpcHandler("TypeC", TestFuncC)
	if response, err := inst.channel.PerformRpc("ToJSTypeA", "ToJSValA"); err != nil {
		panic(fmt.Sprintf("Got unexpected error %v %T", err, err))
	} else if response.RawString() != "ToJSRespA" {
		panic(fmt.Sprintf("Got unexpected response %v", response))
	}
	if _, err := inst.channel.PerformRpc("ToJSTypeB", "ToJSValB"); err == nil || err.Error() != "Got an error here" {
		panic(fmt.Sprintf("Didn't get right error: %v", err))
	}
	fmt.Printf("Done with RPCs")
}

// HandleMessage receives messages from Javascript and uses them to perform actions.
// A message is of the form {"type": "typeName", "body": { stuff here }},
// where the body is passed to the message handler.
func (inst wsprInstance) HandleMessage(message ppapi.Var) {
	fmt.Printf("Entered HandleMessage")
	type handlerType func(ppapi.Var)
	handlerMap := map[string]handlerType{
		"start":      inst.startWSPR,
		"rpcMessage": inst.handleRpcMessage,
		"testRpc":    inst.startRpcTest,
	}
	ty, err := message.LookupStringValuedKey("type")
	if err != nil {
		panic(err.Error())
	}
	h, ok := handlerMap[ty]
	if !ok {
		panic(fmt.Sprintf("No handler found for message type: %q", ty))
	}
	body, err := message.LookupKey("body")
	if err != nil {
		body = ppapi.VarFromString("INVALID")
	}
	h(body)
	body.Release()
}

func (wsprInstance) MouseLockLost() {
	fmt.Printf("Got to MouseLockLost()")
}

func newWsprInstance(inst ppapi.Instance) ppapi.InstanceHandlers {
	return wsprInstance{
		Instance: inst,
		channel:  channel_nacl.NewChannel(inst),
	}
}
