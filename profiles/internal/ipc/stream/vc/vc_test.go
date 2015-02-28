// Use a different package for the tests to ensure that only the exported API is used.

package vc_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"

	"v.io/x/ref/lib/testutil"
	tsecurity "v.io/x/ref/lib/testutil/security"
	"v.io/x/ref/profiles/internal/ipc/stream/id"
	"v.io/x/ref/profiles/internal/ipc/stream/vc"
	"v.io/x/ref/profiles/internal/lib/bqueue"
	"v.io/x/ref/profiles/internal/lib/bqueue/drrqueue"
	"v.io/x/ref/profiles/internal/lib/iobuf"

	"v.io/v23/context"
	"v.io/v23/ipc/version"
	"v.io/v23/naming"
	"v.io/v23/options"
	"v.io/v23/security"
	"v.io/x/ref/profiles/internal/ipc/stream"
)

var (
	clientEP = endpoint(naming.FixedRoutingID(0xcccccccccccccccc))
	serverEP = endpoint(naming.FixedRoutingID(0x5555555555555555))
)

//go:generate v23 test generate

const (
	// Convenience alias to avoid conflicts between the package name "vc" and variables called "vc".
	DefaultBytesBufferedPerFlow = vc.DefaultBytesBufferedPerFlow
	// Shorthands
	SecurityNone = options.VCSecurityNone
	SecurityTLS  = options.VCSecurityConfidential

	LatestVersion = version.IPCVersion7
)

// testFlowEcho writes a random string of 'size' bytes on the flow and then
// ensures that the same string is read back.
func testFlowEcho(t *testing.T, flow stream.Flow, size int) {
	defer flow.Close()
	wrote := testutil.RandomBytes(size)
	go func() {
		buf := wrote
		for len(buf) > 0 {
			limit := 1 + testutil.Rand.Intn(len(buf)) // Random number in [1, n]
			n, err := flow.Write(buf[:limit])
			if n != limit || err != nil {
				t.Errorf("Write returned (%d, %v) want (%d, nil)", n, err, limit)
			}
			buf = buf[limit:]
		}
	}()

	total := 0
	read := make([]byte, size)
	buf := read
	for total < size {
		n, err := flow.Read(buf)
		if err != nil {
			t.Error(err)
			return
		}
		total += n
		buf = buf[n:]
	}
	if bytes.Compare(read, wrote) != 0 {
		t.Errorf("Data read != data written")
	}
}

func TestHandshake(t *testing.T) {
	// When SecurityNone is used, the blessings should not be sent over the wire.
	var (
		client = tsecurity.NewPrincipal("client")
		server = tsecurity.NewPrincipal("server")
	)
	h, vc, err := New(SecurityNone, LatestVersion, client, server, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	flow, err := vc.Connect()
	if err != nil {
		t.Fatal(err)
	}
	if !flow.RemoteBlessings().IsZero() {
		t.Errorf("Server sent blessing %v over insecure transport", flow.RemoteBlessings())
	}
	if !flow.LocalBlessings().IsZero() {
		t.Errorf("Client sent blessing %v over insecure transport", flow.LocalBlessings())
	}
}

func testFlowAuthN(flow stream.Flow, serverBlessings security.Blessings, serverDischarges map[string]security.Discharge, clientPublicKey security.PublicKey) error {
	if got, want := flow.RemoteBlessings(), serverBlessings; !reflect.DeepEqual(got, want) {
		return fmt.Errorf("Server shared blessings %v, want %v", got, want)
	}
	if got, want := flow.RemoteDischarges(), serverDischarges; !reflect.DeepEqual(got, want) {
		return fmt.Errorf("Server shared discharges %#v, want %#v", got, want)
	}
	if got, want := flow.LocalBlessings().PublicKey(), clientPublicKey; !reflect.DeepEqual(got, want) {
		return fmt.Errorf("Client shared %v, want %v", got, want)
	}
	return nil
}

// auth implements security.Authorizer.
type auth struct {
	localPrincipal   security.Principal
	remoteBlessings  security.Blessings
	remoteDischarges map[string]security.Discharge
	suffix, method   string
	err              error
}

// Authorize tests that the context passed to the authorizer is the expected one.
func (a *auth) Authorize(ctx security.Call) error {
	if a.err != nil {
		return a.err
	}
	if got, want := ctx.LocalPrincipal(), a.localPrincipal; !reflect.DeepEqual(got, want) {
		return fmt.Errorf("ctx.LocalPrincipal: got %v, want %v", got, want)
	}
	if got, want := ctx.RemoteBlessings(), a.remoteBlessings; !reflect.DeepEqual(got, want) {
		return fmt.Errorf("ctx.RemoteBlessings: got %v, want %v", got, want)
	}
	if got, want := ctx.RemoteDischarges(), a.remoteDischarges; !reflect.DeepEqual(got, want) {
		return fmt.Errorf("ctx.RemoteDischarges: got %v, want %v", got, want)
	}
	if got, want := ctx.LocalEndpoint(), clientEP; !reflect.DeepEqual(got, want) {
		return fmt.Errorf("ctx.LocalEndpoint: got %v, want %v", got, want)
	}
	if got, want := ctx.RemoteEndpoint(), serverEP; !reflect.DeepEqual(got, want) {
		return fmt.Errorf("ctx.RemoteEndpoint: got %v, want %v", got, want)
	}
	if got, want := ctx.Suffix(), a.suffix; got != want {
		return fmt.Errorf("ctx.RemoteEndpoint: got %v, want %v", got, want)
	}
	if got, want := ctx.Method(), a.method; got != want {
		return fmt.Errorf("ctx.RemoteEndpoint: got %v, want %v", got, want)
	}
	return nil
}

// mockDischargeClient implements vc.DischargeClient.
type mockDischargeClient []security.Discharge

func (m mockDischargeClient) PrepareDischarges(_ *context.T, forcaveats []security.Caveat, impetus security.DischargeImpetus) []security.Discharge {
	return m
}
func (mockDischargeClient) Invalidate(...security.Discharge) {}
func (mockDischargeClient) IPCStreamListenerOpt()            {}
func (mockDischargeClient) IPCStreamVCOpt()                  {}

// Test that mockDischargeClient implements vc.DischargeClient.
var _ vc.DischargeClient = (mockDischargeClient)(nil)

func TestHandshakeTLS(t *testing.T) {
	matchesError := func(got error, want string) error {
		if (got == nil) && len(want) == 0 {
			return nil
		}
		if got == nil && !strings.Contains(got.Error(), want) {
			return fmt.Errorf("got error %q, wanted to match %q", got, want)
		}
		return nil
	}
	var (
		root       = tsecurity.NewIDProvider("root")
		discharger = tsecurity.NewPrincipal("discharger")
		client     = tsecurity.NewPrincipal()
		server     = tsecurity.NewPrincipal()
	)
	tpcav, err := security.NewPublicKeyCaveat(discharger.PublicKey(), "irrelevant", security.ThirdPartyRequirements{}, security.UnconstrainedUse())
	if err != nil {
		t.Fatal(err)
	}
	dis, err := discharger.MintDischarge(tpcav, security.UnconstrainedUse())
	if err != nil {
		t.Fatal(err)
	}
	// Root blesses the client
	if err := root.Bless(client, "client"); err != nil {
		t.Fatal(err)
	}
	// Root blesses the server with a third-party caveat
	if err := root.Bless(server, "server", tpcav); err != nil {
		t.Fatal(err)
	}

	testdata := []struct {
		dischargeClient      vc.DischargeClient
		auth                 *vc.ServerAuthorizer
		dialErr              string
		flowRemoteBlessings  security.Blessings
		flowRemoteDischarges map[string]security.Discharge
	}{
		{
			flowRemoteBlessings: server.BlessingStore().Default(),
		},
		{
			dischargeClient:      mockDischargeClient([]security.Discharge{dis}),
			flowRemoteBlessings:  server.BlessingStore().Default(),
			flowRemoteDischarges: map[string]security.Discharge{dis.ID(): dis},
		},
		{
			dischargeClient: mockDischargeClient([]security.Discharge{dis}),
			auth: &vc.ServerAuthorizer{
				Suffix: "suffix",
				Method: "method",
				Policy: &auth{
					localPrincipal:   client,
					remoteBlessings:  server.BlessingStore().Default(),
					remoteDischarges: map[string]security.Discharge{dis.ID(): dis},
					suffix:           "suffix",
					method:           "method",
				},
			},
			flowRemoteBlessings:  server.BlessingStore().Default(),
			flowRemoteDischarges: map[string]security.Discharge{dis.ID(): dis},
		},
		{
			dischargeClient: mockDischargeClient([]security.Discharge{dis}),
			auth: &vc.ServerAuthorizer{
				Suffix: "suffix",
				Method: "method",
				Policy: &auth{
					err: errors.New("authorization error"),
				},
			},
			dialErr: "authorization error",
		},
	}
	for i, d := range testdata {
		h, vc, err := New(SecurityTLS, LatestVersion, client, server, d.dischargeClient, d.auth)
		if merr := matchesError(err, d.dialErr); merr != nil {
			t.Errorf("Test #%d: HandshakeDialedVC with server authorizer %#v:: %v", i, d.auth.Policy, merr)
		}
		if err != nil {
			continue
		}
		flow, err := vc.Connect()
		if err != nil {
			h.Close()
			t.Errorf("Unable to create flow: %v", err)
			continue
		}
		if err := testFlowAuthN(flow, d.flowRemoteBlessings, d.flowRemoteDischarges, client.PublicKey()); err != nil {
			h.Close()
			t.Error(err)
			continue
		}
		h.Close()
	}
}

func testConnect_Small(t *testing.T, security options.VCSecurityLevel) {
	h, vc, err := New(security, LatestVersion, tsecurity.NewPrincipal("client"), tsecurity.NewPrincipal("server"), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	flow, err := vc.Connect()
	if err != nil {
		t.Fatal(err)
	}
	testFlowEcho(t, flow, 10)
}
func TestConnect_Small(t *testing.T)    { testConnect_Small(t, SecurityNone) }
func TestConnect_SmallTLS(t *testing.T) { testConnect_Small(t, SecurityTLS) }

func testConnect(t *testing.T, security options.VCSecurityLevel) {
	h, vc, err := New(security, LatestVersion, tsecurity.NewPrincipal("client"), tsecurity.NewPrincipal("server"), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	flow, err := vc.Connect()
	if err != nil {
		t.Fatal(err)
	}
	testFlowEcho(t, flow, 10*DefaultBytesBufferedPerFlow)
}
func TestConnect(t *testing.T)    { testConnect(t, SecurityNone) }
func TestConnectTLS(t *testing.T) { testConnect(t, SecurityTLS) }

func testConnect_Version7(t *testing.T, security options.VCSecurityLevel) {
	h, vc, err := New(security, version.IPCVersion7, tsecurity.NewPrincipal("client"), tsecurity.NewPrincipal("server"), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	flow, err := vc.Connect()
	if err != nil {
		t.Fatal(err)
	}
	testFlowEcho(t, flow, 10)
}
func TestConnect_Version7(t *testing.T)    { testConnect_Version7(t, SecurityNone) }
func TestConnect_Version7TLS(t *testing.T) { testConnect_Version7(t, SecurityTLS) }

// helper function for testing concurrent operations on multiple flows over the
// same VC.  Such tests are most useful when running the race detector.
// (go test -race ...)
func testConcurrentFlows(t *testing.T, security options.VCSecurityLevel, flows, gomaxprocs int) {
	mp := runtime.GOMAXPROCS(gomaxprocs)
	defer runtime.GOMAXPROCS(mp)
	h, vc, err := New(security, LatestVersion, tsecurity.NewPrincipal("client"), tsecurity.NewPrincipal("server"), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()

	var wg sync.WaitGroup
	wg.Add(flows)
	for i := 0; i < flows; i++ {
		go func(n int) {
			defer wg.Done()
			flow, err := vc.Connect()
			if err != nil {
				t.Error(err)
			} else {
				testFlowEcho(t, flow, (n+1)*DefaultBytesBufferedPerFlow)
			}
		}(i)
	}
	wg.Wait()
}

func TestConcurrentFlows_1(t *testing.T)    { testConcurrentFlows(t, SecurityNone, 10, 1) }
func TestConcurrentFlows_1TLS(t *testing.T) { testConcurrentFlows(t, SecurityTLS, 10, 1) }

func TestConcurrentFlows_10(t *testing.T)    { testConcurrentFlows(t, SecurityNone, 10, 10) }
func TestConcurrentFlows_10TLS(t *testing.T) { testConcurrentFlows(t, SecurityTLS, 10, 10) }

func testListen(t *testing.T, security options.VCSecurityLevel) {
	data := "the dark knight"
	h, vc, err := New(security, LatestVersion, tsecurity.NewPrincipal("client"), tsecurity.NewPrincipal("server"), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	if err := h.VC.AcceptFlow(id.Flow(21)); err == nil {
		t.Errorf("Expected AcceptFlow on a new flow to fail as Listen was not called")
	}

	ln, err := vc.Listen()
	if err != nil {
		t.Fatalf("vc.Listen failed: %v", err)
		return
	}
	_, err = vc.Listen()
	if err == nil {
		t.Fatalf("Second call to vc.Listen should have failed")
		return
	}

	if err := h.VC.AcceptFlow(id.Flow(23)); err != nil {
		t.Fatal(err)
	}
	cipherdata, err := h.otherEnd.VC.Encrypt(id.Flow(23), iobuf.NewSlice([]byte(data)))
	if err != nil {
		t.Fatal(err)
	}
	if err := h.VC.DispatchPayload(id.Flow(23), cipherdata); err != nil {
		t.Fatal(err)
	}
	flow, err := ln.Accept()
	if err != nil {
		t.Fatal(err)
	}
	if err := ln.Close(); err != nil {
		t.Error(err)
	}
	flow.Close()
	var buf [4096]byte
	if n, err := flow.Read(buf[:]); n != len(data) || err != nil || string(buf[:n]) != data {
		t.Errorf("Got (%d, %v) = %q, want (%d, nil) = %q", n, err, string(buf[:n]), len(data), data)
	}
	if n, err := flow.Read(buf[:]); n != 0 || err != io.EOF {
		t.Errorf("Got (%d, %v) want (0, %v)", n, err, io.EOF)
	}
}
func TestListen(t *testing.T)    { testListen(t, SecurityNone) }
func TestListenTLS(t *testing.T) { testListen(t, SecurityTLS) }

func testNewFlowAfterClose(t *testing.T, security options.VCSecurityLevel) {
	h, _, err := New(security, LatestVersion, tsecurity.NewPrincipal("client"), tsecurity.NewPrincipal("server"), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	h.VC.Close("reason")
	if err := h.VC.AcceptFlow(id.Flow(10)); err == nil {
		t.Fatalf("New flows should not be accepted once the VC is closed")
	}
}
func TestNewFlowAfterClose(t *testing.T)    { testNewFlowAfterClose(t, SecurityNone) }
func TestNewFlowAfterCloseTLS(t *testing.T) { testNewFlowAfterClose(t, SecurityTLS) }

func testConnectAfterClose(t *testing.T, security options.VCSecurityLevel) {
	h, vc, err := New(security, LatestVersion, tsecurity.NewPrincipal("client"), tsecurity.NewPrincipal("server"), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	h.VC.Close("myerr")
	if f, err := vc.Connect(); f != nil || err == nil || !strings.Contains(err.Error(), "myerr") {
		t.Fatalf("Got (%v, %v), want (nil, %q)", f, err, "myerr")
	}
}
func TestConnectAfterClose(t *testing.T)    { testConnectAfterClose(t, SecurityNone) }
func TestConnectAfterCloseTLS(t *testing.T) { testConnectAfterClose(t, SecurityTLS) }

// helper implements vc.Helper and also sets up a single VC.
type helper struct {
	VC *vc.VC
	bq bqueue.T

	mu       sync.Mutex
	otherEnd *helper // GUARDED_BY(mu)
}

// New creates both ends of a VC but returns only the "client" end (i.e., the
// one that initiated the VC). The "server" end (the one that "accepted" the VC)
// listens for flows and simply echoes data read.
func New(security options.VCSecurityLevel, v version.IPCVersion, client, server security.Principal, dischargeClient vc.DischargeClient, auth *vc.ServerAuthorizer) (*helper, stream.VC, error) {
	clientH := &helper{bq: drrqueue.New(vc.MaxPayloadSizeBytes)}
	serverH := &helper{bq: drrqueue.New(vc.MaxPayloadSizeBytes)}
	clientH.otherEnd = serverH
	serverH.otherEnd = clientH

	vci := id.VC(1234)

	clientParams := vc.Params{
		VCI:      vci,
		Dialed:   true,
		LocalEP:  clientEP,
		RemoteEP: serverEP,
		Pool:     iobuf.NewPool(0),
		Helper:   clientH,
		Version:  v,
	}
	serverParams := vc.Params{
		VCI:      vci,
		LocalEP:  serverEP,
		RemoteEP: clientEP,
		Pool:     iobuf.NewPool(0),
		Helper:   serverH,
		Version:  v,
	}

	clientH.VC = vc.InternalNew(clientParams)
	serverH.VC = vc.InternalNew(serverParams)
	clientH.AddReceiveBuffers(vci, vc.SharedFlowID, vc.DefaultBytesBufferedPerFlow)

	go clientH.pipeLoop(serverH.VC)
	go serverH.pipeLoop(clientH.VC)

	lopts := []stream.ListenerOpt{vc.LocalPrincipal{server}, security}
	vcopts := []stream.VCOpt{vc.LocalPrincipal{client}, security}

	if dischargeClient != nil {
		lopts = append(lopts, dischargeClient)
	}
	if auth != nil {
		vcopts = append(vcopts, auth)
	}

	c := serverH.VC.HandshakeAcceptedVC(lopts...)
	if err := clientH.VC.HandshakeDialedVC(vcopts...); err != nil {
		go func() { <-c }()
		return nil, nil, err
	}
	hr := <-c
	if hr.Error != nil {
		return nil, nil, hr.Error
	}
	go acceptLoop(hr.Listener)
	return clientH, clientH.VC, nil
}

// pipeLoop forwards slices written to h.bq to dst.
func (h *helper) pipeLoop(dst *vc.VC) {
	for {
		w, bufs, err := h.bq.Get(nil)
		if err != nil {
			return
		}
		fid := id.Flow(w.ID())
		for _, b := range bufs {
			cipher, err := h.VC.Encrypt(fid, b)
			if err != nil {
				panic(err)
			}
			if err := dst.DispatchPayload(fid, cipher); err != nil {
				panic(err)
				return
			}
		}
		if w.IsDrained() {
			h.VC.ShutdownFlow(fid)
			dst.ShutdownFlow(fid)
		}
	}
}

func acceptLoop(ln stream.Listener) {
	for {
		f, err := ln.Accept()
		if err != nil {
			return
		}
		go echoLoop(f)
	}
}

func echoLoop(flow stream.Flow) {
	var buf [vc.DefaultBytesBufferedPerFlow * 20]byte
	for {
		n, err := flow.Read(buf[:])
		if err == io.EOF {
			return
		}
		if err == nil {
			_, err = flow.Write(buf[:n])
		}
		if err != nil {
			panic(err)
		}
	}
}

func (h *helper) NotifyOfNewFlow(vci id.VC, fid id.Flow, bytes uint) {
	h.mu.Lock()
	if h.otherEnd != nil {
		if err := h.otherEnd.VC.AcceptFlow(fid); err != nil {
			panic(err)
		}
		h.otherEnd.VC.ReleaseCounters(fid, uint32(bytes))
	}
	h.mu.Unlock()
}

func (h *helper) AddReceiveBuffers(vci id.VC, fid id.Flow, bytes uint) {
	h.mu.Lock()
	if h.otherEnd != nil {
		h.otherEnd.VC.ReleaseCounters(fid, uint32(bytes))
	}
	h.mu.Unlock()
}

func (h *helper) NewWriter(vci id.VC, fid id.Flow) (bqueue.Writer, error) {
	return h.bq.NewWriter(bqueue.ID(fid), 0, DefaultBytesBufferedPerFlow)
}

func (h *helper) Close() {
	h.VC.Close("helper closed")
	h.bq.Close()
	h.mu.Lock()
	otherEnd := h.otherEnd
	h.otherEnd = nil
	h.mu.Unlock()
	if otherEnd != nil {
		otherEnd.mu.Lock()
		otherEnd.otherEnd = nil
		otherEnd.mu.Unlock()
		otherEnd.Close()
	}
}

type endpoint naming.RoutingID

func (e endpoint) Network() string             { return "test" }
func (e endpoint) VersionedString(int) string  { return e.String() }
func (e endpoint) String() string              { return naming.RoutingID(e).String() }
func (e endpoint) Name() string                { return naming.JoinAddressName(e.String(), "") }
func (e endpoint) RoutingID() naming.RoutingID { return naming.RoutingID(e) }
func (e endpoint) Addr() net.Addr              { return nil }
func (e endpoint) ServesMountTable() bool      { return false }
func (e endpoint) BlessingNames() []string     { return nil }
