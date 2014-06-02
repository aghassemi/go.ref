package impl

import (
	"reflect"
	"testing"

	iapplication "veyron/services/mgmt/application"
	istore "veyron/services/store/testutil"

	"veyron2/naming"
	"veyron2/rt"
	"veyron2/services/mgmt/application"
)

// TestInterface tests that the implementation correctly implements
// the Application interface.
func TestInterface(t *testing.T) {
	runtime := rt.Init()
	ctx := runtime.NewContext()
	defer runtime.Shutdown()

	// Setup and start the application manager server.
	server, err := runtime.NewServer()
	if err != nil {
		t.Fatalf("NewServer() failed: %v", err)
	}

	// Setup and start a store server.
	name, cleanup := istore.NewStore(t, server, runtime.Identity().PublicID())
	defer cleanup()

	dispatcher, err := NewDispatcher(name, nil)
	if err != nil {
		t.Fatalf("NewDispatcher() failed: %v", err)
	}
	suffix := ""
	if err := server.Register(suffix, dispatcher); err != nil {
		t.Fatalf("Register(%v, %v) failed: %v", suffix, dispatcher, err)
	}
	protocol, hostname := "tcp", "localhost:0"
	endpoint, err := server.Listen(protocol, hostname)
	if err != nil {
		t.Fatalf("Listen(%v, %v) failed: %v", protocol, hostname, err)
	}

	// Create client stubs for talking to the server.
	stub, err := iapplication.BindRepository(naming.JoinAddressName(endpoint.String(), "search"))
	if err != nil {
		t.Fatalf("BindRepository() failed: %v", err)
	}
	stubV1, err := iapplication.BindRepository(naming.JoinAddressName(endpoint.String(), "search/v1"))
	if err != nil {
		t.Fatalf("BindRepository() failed: %v", err)
	}
	stubV2, err := iapplication.BindRepository(naming.JoinAddressName(endpoint.String(), "search/v2"))
	if err != nil {
		t.Fatalf("BindRepository() failed: %v", err)
	}

	// Create example envelopes.
	envelopeV1 := application.Envelope{
		Args:   []string{"--help"},
		Env:    []string{"DEBUG=1"},
		Binary: "/veyron/name/of/binary",
	}
	envelopeV2 := application.Envelope{
		Args:   []string{"--verbose"},
		Env:    []string{"DEBUG=0"},
		Binary: "/veyron/name/of/binary",
	}

	// Test Put(), adding a number of application envelopes.
	if err := stubV1.Put(ctx, []string{"base", "media"}, envelopeV1); err != nil {
		t.Fatalf("Put() failed: %v", err)
	}
	if err := stubV2.Put(ctx, []string{"base"}, envelopeV2); err != nil {
		t.Fatalf("Put() failed: %v", err)
	}
	if err := stub.Put(ctx, []string{"base", "media"}, envelopeV1); err == nil || err.Error() != errInvalidSuffix.Error() {
		t.Fatalf("Unexpected error: expected %v, got %v", errInvalidSuffix, err)
	}

	// Test Match(), trying to retrieve both existing and non-existing
	// application envelopes.
	var output application.Envelope
	if output, err = stubV2.Match(ctx, []string{"base", "media"}); err != nil {
		t.Fatalf("Match() failed: %v", err)
	}
	if !reflect.DeepEqual(envelopeV2, output) {
		t.Fatalf("Incorrect output: expected %v, got %v", envelopeV2, output)
	}
	if output, err = stubV1.Match(ctx, []string{"media"}); err != nil {
		t.Fatalf("Match() failed: %v", err)
	}
	if !reflect.DeepEqual(envelopeV1, output) {
		t.Fatalf("Unexpected output: expected %v, got %v", envelopeV1, output)
	}
	if _, err := stubV2.Match(ctx, []string{"media"}); err == nil || err.Error() != errNotFound.Error() {
		t.Fatalf("Unexpected error: expected %v, got %v", errNotFound, err)
	}
	if _, err := stubV2.Match(ctx, []string{}); err == nil || err.Error() != errNotFound.Error() {
		t.Fatalf("Unexpected error: expected %v, got %v", errNotFound, err)
	}
	if _, err := stub.Match(ctx, []string{"media"}); err == nil || err.Error() != errInvalidSuffix.Error() {
		t.Fatalf("Unexpected error: expected %v, got %v", errInvalidSuffix, err)
	}

	// Test Remove(), trying to remove both existing and non-existing
	// application envelopes.
	if err := stubV1.Remove(ctx, "base"); err != nil {
		t.Fatalf("Remove() failed: %v", err)
	}
	if output, err = stubV1.Match(ctx, []string{"media"}); err != nil {
		t.Fatalf("Match() failed: %v", err)
	}
	if err := stubV1.Remove(ctx, "base"); err == nil || err.Error() != errNotFound.Error() {
		t.Fatalf("Unexpected error: expected %v, got %v", errNotFound, err)
	}
	if err := stub.Remove(ctx, "base"); err != nil {
		t.Fatalf("Remove() failed: %v", err)
	}
	if err := stubV2.Remove(ctx, "media"); err == nil || err.Error() != errNotFound.Error() {
		t.Fatalf("Unexpected error: expected %v, got %v", errNotFound, err)
	}
	if err := stubV1.Remove(ctx, "media"); err != nil {
		t.Fatalf("Remove() failed: %v", err)
	}

	// Finally, use Match() to test that Remove really removed the
	// application envelopes.
	if _, err := stubV1.Match(ctx, []string{"base"}); err == nil || err.Error() != errNotFound.Error() {
		t.Fatalf("Unexpected error: expected %v, got %v", errNotFound, err)
	}
	if _, err := stubV1.Match(ctx, []string{"media"}); err == nil || err.Error() != errNotFound.Error() {
		t.Fatalf("Unexpected error: expected %v, got %v", errNotFound, err)
	}
	if _, err := stubV2.Match(ctx, []string{"base"}); err == nil || err.Error() != errNotFound.Error() {
		t.Fatalf("Unexpected error: expected %v, got %v", errNotFound, err)
	}

	// Shutdown the application manager server.
	if err := server.Stop(); err != nil {
		t.Fatalf("Stop() failed: %v", err)
	}
}
