package impl_test

import (
	"testing"

	"veyron.io/veyron/veyron2/ipc"
	"veyron.io/veyron/veyron2/rt"

	"veyron.io/veyron/veyron/profiles"
)

func startServer(t *testing.T, disp ipc.Dispatcher) (ipc.Server, string, error) {
	server, err := rt.R().NewServer()
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
		return nil, "", err
	}
	endpoint, err := server.ListenX(profiles.LocalListenSpec)
	if err != nil {
		t.Fatalf("Listen failed: %v", err)
		return nil, "", err
	}
	if err := server.Serve("", disp); err != nil {
		t.Fatalf("Serve failed: %v", err)
		return nil, "", err
	}
	return server, endpoint.String(), nil
}

func stopServer(t *testing.T, server ipc.Server) {
	if err := server.Stop(); err != nil {
		t.Errorf("server.Stop failed: %v", err)
	}
}
