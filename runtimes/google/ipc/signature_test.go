package ipc_test

import (
	"fmt"
	"reflect"
	"testing"

	"v.io/core/veyron2"
	"v.io/core/veyron2/context"
	"v.io/core/veyron2/ipc"
	"v.io/core/veyron2/ipc/reserved"
	"v.io/core/veyron2/naming"
	"v.io/core/veyron2/rt"
	"v.io/core/veyron2/vdl"
	"v.io/core/veyron2/vdl/vdlroot/src/signature"

	"v.io/core/veyron/lib/testutil"
	tsecurity "v.io/core/veyron/lib/testutil/security"
	"v.io/core/veyron/profiles"
)

func init() { testutil.Init() }

func startSigServer(ctx *context.T, sig sigImpl) (string, func(), error) {
	server, err := veyron2.NewServer(ctx)
	if err != nil {
		return "", nil, fmt.Errorf("failed to start sig server: %v", err)
	}
	eps, err := server.Listen(profiles.LocalListenSpec)
	if err != nil {
		return "", nil, fmt.Errorf("failed to listen: %v", err)
	}
	if err := server.Serve("", sig, nil); err != nil {
		return "", nil, err
	}
	return eps[0].String(), func() { server.Stop() }, nil
}

type sigImpl struct{}

func (sigImpl) NonStreaming0(ipc.ServerContext)                          { panic("X") }
func (sigImpl) NonStreaming1(_ ipc.ServerContext, _ string) error        { panic("X") }
func (sigImpl) Streaming0(_ *streamStringBool)                           { panic("X") }
func (sigImpl) Streaming1(_ *streamStringBool, _ int64) (float64, error) { panic("X") }

type streamStringBool struct{ ipc.ServerCall }

func (*streamStringBool) Init(ipc.ServerCall) { panic("X") }
func (*streamStringBool) RecvStream() interface {
	Advance() bool
	Value() string
	Err() error
} {
	panic("X")
}
func (*streamStringBool) SendStream() interface {
	Send(_ bool) error
} {
	panic("X")
}

func TestMethodSignature(t *testing.T) {
	runtime, err := rt.New()
	if err != nil {
		t.Fatalf("Couldn't initialize runtime: %s", err)
	}
	defer runtime.Cleanup()
	ctx := runtime.NewContext()
	if ctx, err = veyron2.SetPrincipal(ctx, tsecurity.NewPrincipal("test-blessing")); err != nil {
		t.Fatal(err)
	}
	ep, stop, err := startSigServer(ctx, sigImpl{})
	if err != nil {
		t.Fatalf("startSigServer: %v", err)
	}
	defer stop()
	name := naming.JoinAddressName(ep, "")

	tests := []struct {
		Method string
		Want   signature.Method
	}{
		{"NonStreaming0", signature.Method{
			Name: "NonStreaming0",
		}},
		{"NonStreaming1", signature.Method{
			Name:    "NonStreaming1",
			InArgs:  []signature.Arg{{Type: vdl.StringType}},
			OutArgs: []signature.Arg{{Type: vdl.ErrorType}},
		}},
		{"Streaming0", signature.Method{
			Name:      "Streaming0",
			InStream:  &signature.Arg{Type: vdl.StringType},
			OutStream: &signature.Arg{Type: vdl.BoolType},
		}},
		{"Streaming1", signature.Method{
			Name:      "Streaming1",
			InArgs:    []signature.Arg{{Type: vdl.Int64Type}},
			OutArgs:   []signature.Arg{{Type: vdl.Float64Type}, {Type: vdl.ErrorType}},
			InStream:  &signature.Arg{Type: vdl.StringType},
			OutStream: &signature.Arg{Type: vdl.BoolType},
		}},
	}
	for _, test := range tests {
		sig, err := reserved.MethodSignature(ctx, name, test.Method)
		if err != nil {
			t.Errorf("call failed: %v", err)
		}
		if got, want := sig, test.Want; !reflect.DeepEqual(got, want) {
			t.Errorf("%s got %#v, want %#v", test.Method, got, want)
		}
	}
}

func TestSignature(t *testing.T) {
	runtime, err := rt.New()
	if err != nil {
		t.Fatalf("Couldn't initialize runtime: %s", err)
	}
	defer runtime.Cleanup()
	ctx := runtime.NewContext()
	if ctx, err = veyron2.SetPrincipal(ctx, tsecurity.NewPrincipal("test-blessing")); err != nil {
		t.Fatal(err)
	}
	ep, stop, err := startSigServer(ctx, sigImpl{})
	if err != nil {
		t.Fatalf("startSigServer: %v", err)
	}
	defer stop()
	name := naming.JoinAddressName(ep, "")
	sig, err := reserved.Signature(ctx, name)
	if err != nil {
		t.Errorf("call failed: %v", err)
	}
	if got, want := len(sig), 2; got != want {
		t.Fatalf("got sig %#v len %d, want %d", sig, got, want)
	}
	// Check expected methods.
	methods := signature.Interface{
		Doc: "The empty interface contains methods not attached to any interface.",
		Methods: []signature.Method{
			{
				Name: "NonStreaming0",
			},
			{
				Name:    "NonStreaming1",
				InArgs:  []signature.Arg{{Type: vdl.StringType}},
				OutArgs: []signature.Arg{{Type: vdl.ErrorType}},
			},
			{
				Name:      "Streaming0",
				InStream:  &signature.Arg{Type: vdl.StringType},
				OutStream: &signature.Arg{Type: vdl.BoolType},
			},
			{
				Name:      "Streaming1",
				InArgs:    []signature.Arg{{Type: vdl.Int64Type}},
				OutArgs:   []signature.Arg{{Type: vdl.Float64Type}, {Type: vdl.ErrorType}},
				InStream:  &signature.Arg{Type: vdl.StringType},
				OutStream: &signature.Arg{Type: vdl.BoolType},
			},
		},
	}
	if got, want := sig[0], methods; !reflect.DeepEqual(got, want) {
		t.Errorf("got sig[0] %#v, want %#v", got, want)
	}
	// Check reserved methods.
	if got, want := sig[1].Name, "__Reserved"; got != want {
		t.Errorf("got sig[1].Name %q, want %q", got, want)
	}
	if got, want := signature.MethodNames(sig[1:2]), []string{"__Glob", "__MethodSignature", "__Signature"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got sig[1] methods %v, want %v", got, want)
	}
}
