package main

import (
	"bytes"
	"strings"
	"testing"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/ipc"
	"v.io/v23/naming"
	"v.io/v23/services/mgmt/binary"
	"v.io/v23/services/mgmt/build"
	"v.io/v23/verror"
	"v.io/x/lib/vlog"

	"v.io/core/veyron/lib/testutil"
	_ "v.io/core/veyron/profiles"
)

type mock struct{}

func (mock) Build(ctx build.BuilderBuildContext, arch build.Architecture, opsys build.OperatingSystem) ([]byte, error) {
	vlog.VI(2).Infof("Build(%v, %v) was called", arch, opsys)
	iterator := ctx.RecvStream()
	for iterator.Advance() {
	}
	if err := iterator.Err(); err != nil {
		vlog.Errorf("Advance() failed: %v", err)
		return nil, verror.New(verror.ErrInternal, ctx.Context())
	}
	return nil, nil
}

func (mock) Describe(_ ipc.ServerContext, name string) (binary.Description, error) {
	vlog.VI(2).Infof("Describe(%v) was called", name)
	return binary.Description{}, nil
}

type dispatcher struct{}

func startServer(ctx *context.T, t *testing.T) (ipc.Server, naming.Endpoint) {
	server, err := v23.NewServer(ctx)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	l := v23.GetListenSpec(ctx)
	endpoints, err := server.Listen(l)
	if err != nil {
		t.Fatalf("Listen(%s) failed: %v", l, err)
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
	var shutdown v23.Shutdown
	gctx, shutdown = testutil.InitForTest()
	defer shutdown()

	server, endpoint := startServer(gctx, t)
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
