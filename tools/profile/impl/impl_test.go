package impl_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"veyron/services/mgmt/profile"
	"veyron/tools/profile/impl"

	"veyron2"
	"veyron2/ipc"
	"veyron2/naming"
	"veyron2/rt"
	"veyron2/security"
	"veyron2/vlog"
)

var (
	// spec is an example profile specification used throughout the test.
	spec = profile.Specification{
		Format:      profile.Format{Name: "elf", Attributes: map[string]string{"os": "linux"}},
		Libraries:   map[profile.Library]struct{}{profile.Library{Name: "foo", MajorVersion: "1", MinorVersion: "0"}: struct{}{}},
		Label:       "example",
		Description: "Example profile to test the profile manager implementation.",
	}
)

type server struct {
	suffix string
}

func (s *server) Label(ipc.ServerContext) (string, error) {
	vlog.VI(2).Infof("%v.Label() was called", s.suffix)
	if s.suffix != "exists" {
		return "", fmt.Errorf("profile doesn't exist: %v", s.suffix)
	}
	return spec.Label, nil
}

func (s *server) Description(ipc.ServerContext) (string, error) {
	vlog.VI(2).Infof("%v.Description() was called", s.suffix)
	if s.suffix != "exists" {
		return "", fmt.Errorf("profile doesn't exist: %v", s.suffix)
	}
	return spec.Description, nil
}

func (s *server) Specification(ipc.ServerContext) (profile.Specification, error) {
	vlog.VI(2).Infof("%v.Specification() was called", s.suffix)
	if s.suffix != "exists" {
		return profile.Specification{}, fmt.Errorf("profile doesn't exist: %v", s.suffix)
	}
	return spec, nil
}

func (s *server) Put(_ ipc.ServerContext, _ profile.Specification) error {
	vlog.VI(2).Infof("%v.Put() was called", s.suffix)
	return nil
}

func (s *server) Remove(ipc.ServerContext) error {
	vlog.VI(2).Infof("%v.Remove() was called", s.suffix)
	if s.suffix != "exists" {
		return fmt.Errorf("profile doesn't exist: %v", s.suffix)
	}
	return nil
}

type dispatcher struct {
}

func NewDispatcher() *dispatcher {
	return &dispatcher{}
}

func (d *dispatcher) Lookup(suffix string) (ipc.Invoker, security.Authorizer, error) {
	invoker := ipc.ReflectInvoker(profile.NewServerProfile(&server{suffix: suffix}))
	return invoker, nil, nil
}

func startServer(t *testing.T, r veyron2.Runtime) (ipc.Server, naming.Endpoint, error) {
	dispatcher := NewDispatcher()
	server, err := r.NewServer()
	if err != nil {
		t.Errorf("NewServer failed: %v", err)
		return nil, nil, err
	}
	if err := server.Register("", dispatcher); err != nil {
		t.Errorf("Register failed: %v", err)
		return nil, nil, err
	}
	endpoint, err := server.Listen("tcp", "localhost:0")
	if err != nil {
		t.Errorf("Listen failed: %v", err)
		return nil, nil, err
	}
	return server, endpoint, nil
}

func stopServer(t *testing.T, server ipc.Server) {
	if err := server.Stop(); err != nil {
		t.Errorf("server.Stop failed: %v", err)
	}
}

func TestProfileClient(t *testing.T) {
	runtime := rt.Init()
	server, endpoint, err := startServer(t, runtime)
	if err != nil {
		return
	}
	defer stopServer(t, server)
	// Setup the command-line.
	cmd := impl.Root()
	var stdout, stderr bytes.Buffer
	cmd.Init(nil, &stdout, &stderr)
	exists := naming.JoinAddressName(endpoint.String(), "//exists")

	// Test the 'label' command.
	if err := cmd.Execute([]string{"label", exists}); err != nil {
		t.Fatalf("%v", err)
	}
	if expected, got := spec.Label, strings.TrimSpace(stdout.String()); got != expected {
		t.Errorf("Got %q, expected %q", got, expected)
	}
	stdout.Reset()

	// Test the 'description' command.
	if err := cmd.Execute([]string{"description", exists}); err != nil {
		t.Fatalf("%v", err)
	}
	if expected, got := spec.Description, strings.TrimSpace(stdout.String()); got != expected {
		t.Errorf("Got %q, expected %q", got, expected)
	}
	stdout.Reset()

	// Test the 'spec' command.
	if err := cmd.Execute([]string{"spec", exists}); err != nil {
		t.Fatalf("%v", err)
	}
	if expected, got := fmt.Sprintf("%#v", spec), strings.TrimSpace(stdout.String()); got != expected {
		t.Errorf("Got %q, expected %q", got, expected)
	}
	stdout.Reset()

	// Test the 'put' command.
	if err := cmd.Execute([]string{"put", exists}); err != nil {
		t.Fatalf("%v", err)
	}
	if expected, got := "Specification updated successfully.", strings.TrimSpace(stdout.String()); got != expected {
		t.Errorf("Got %q, expected %q", got, expected)
	}
	stdout.Reset()

	// Test the 'remove' command.
	if err := cmd.Execute([]string{"remove", exists}); err != nil {
		t.Fatalf("%v", err)
	}
	if expected, got := "Profile removed successfully.", strings.TrimSpace(stdout.String()); got != expected {
		t.Errorf("Got %q, expected %q", got, expected)
	}
	stdout.Reset()
}
