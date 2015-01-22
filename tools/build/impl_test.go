package main

import (
	"bytes"
	"strings"
	"testing"

	"v.io/core/veyron2"
	"v.io/core/veyron2/context"
	"v.io/core/veyron2/ipc"
	"v.io/core/veyron2/naming"
	"v.io/core/veyron2/options"
	"v.io/core/veyron2/rt"
	"v.io/core/veyron2/services/mgmt/binary"
	"v.io/core/veyron2/services/mgmt/build"
	verror "v.io/core/veyron2/verror2"
	"v.io/core/veyron2/vlog"

	tsecurity "v.io/core/veyron/lib/testutil/security"
	"v.io/core/veyron/profiles"
)

type mock struct{}

func (mock) Build(ctx build.BuilderBuildContext, arch build.Architecture, opsys build.OperatingSystem) ([]byte, error) {
	vlog.VI(2).Infof("Build(%v, %v) was called", arch, opsys)
	iterator := ctx.RecvStream()
	for iterator.Advance() {
	}
	if err := iterator.Err(); err != nil {
		vlog.Errorf("Advance() failed: %v", err)
		return nil, verror.Make(verror.Internal, ctx.Context())
	}
	return nil, nil
}

func (mock) Describe(_ ipc.ServerContext, name string) (binary.Description, error) {
	vlog.VI(2).Infof("Describe(%v) was called", name)
	return binary.Description{}, nil
}

type dispatcher struct{}

func startServer(ctx *context.T, t *testing.T) (ipc.Server, naming.Endpoint) {
	server, err := veyron2.NewServer(ctx)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	endpoints, err := server.Listen(profiles.LocalListenSpec)
	if err != nil {
		t.Fatalf("Listen(%s) failed: %v", profiles.LocalListenSpec, err)
	}
	unpublished := ""
	if err := server.Serve(unpublished, build.BuilderServer(&mock{}), nil); err != nil {
		t.Fatalf("Serve(%v) failed: %v", unpublished, err)
	}
	return server, endpoints[0]
}

func stopServer(t *testing.T, server ipc.Server) {
	if err := server.Stop(); err != nil {
		t.Errorf("Stop() failed: %v", err)
	}
}

func TestBuildClient(t *testing.T) {
	var err error
	// TODO(ataly, mattr, suharshs): This is a HACK to ensure that the server and the
	// client have the same freshly created principal. One way to avoid the RuntimePrincipal
	// option is to have a global client context.T (in main.go) instead of a veyron2.Runtime.
	runtime, err = rt.New(options.RuntimePrincipal{tsecurity.NewPrincipal("test-blessing")})
	if err != nil {
		t.Fatalf("Unexpected error initializing runtime: %s", err)
	}
	ctx := runtime.NewContext()
	server, endpoint := startServer(ctx, t)
	defer stopServer(t, server)

	cmd := root()
	var stdout, stderr bytes.Buffer
	cmd.Init(nil, &stdout, &stderr)

	// Test the 'Build' command.
	if err := cmd.Execute([]string{"build", naming.JoinAddressName(endpoint.String(), ""), "v.io/core/veyron/tools/build"}); err != nil {
		t.Fatalf("%v", err)
	}
	if expected, got := "", strings.TrimSpace(stdout.String()); got != expected {
		t.Errorf("Unexpected output from build: got %q, expected %q", got, expected)
	}
}
