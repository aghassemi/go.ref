// Use a different package for the tests to ensure that only the exported API is used.

package vc_test

import (
	"bytes"
	crand "crypto/rand"
	"io"
	"math/rand"
	"net"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"

	"veyron/runtimes/google/ipc/stream/id"
	"veyron/runtimes/google/ipc/stream/vc"
	"veyron/runtimes/google/lib/bqueue"
	"veyron/runtimes/google/lib/bqueue/drrqueue"
	"veyron/runtimes/google/lib/iobuf"

	"veyron2"
	"veyron2/ipc/stream"
	"veyron2/naming"
	"veyron2/security"
)

// Convenience alias to avoid conflicts between the package name "vc" and variables called "vc".
const DefaultBytesBufferedPerFlow = vc.DefaultBytesBufferedPerFlow

const (
	// Shorthands
	SecurityNone = veyron2.VCSecurityNone
	SecurityTLS  = veyron2.VCSecurityConfidential
)

var (
	clientID = security.FakePrivateID("client")
	serverID = security.FakePrivateID("server")
)

// testFlowEcho writes a random string of 'size' bytes on the flow and then
// ensures that the same string is read back.
func testFlowEcho(t *testing.T, flow stream.Flow, size int) {
	defer flow.Close()
	wrote, err := randomString(size)
	if err != nil {
		t.Errorf("Couldn't generate random string: %v", err)
	}
	go func() {
		buf := wrote
		for len(buf) > 0 {
			limit := 1 + rand.Intn(len(buf)) // Random number in [1, n]
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

func testHandshake(t *testing.T, security veyron2.VCSecurityLevel, localID, remoteID security.PublicID) {
	h, vc := New(security)
	flow, err := vc.Connect()
	if err != nil {
		t.Fatal(err)
	}
	lID := flow.LocalID()
	if !reflect.DeepEqual(lID.Names(), localID.Names()) {
		t.Errorf("Client says LocalID is %q want %q", lID, localID)
	}
	rID := flow.RemoteID()
	if !reflect.DeepEqual(rID.Names(), remoteID.Names()) {
		t.Errorf("Client says RemoteID is %q want %q", rID, remoteID)
	}
	if g, w := lID.PublicKey(), localID.PublicKey(); !reflect.DeepEqual(g, w) {
		t.Errorf("Client identity public key mismatch. Got %v want %v", g, w)
	}
	if g, w := rID.PublicKey(), remoteID.PublicKey(); !reflect.DeepEqual(g, w) {
		t.Errorf("Server identity public key mismatch. Got %v want %v", g, w)
	}
	h.Close()
}
func TestHandshake(t *testing.T) {
	testHandshake(t, SecurityNone, security.FakePublicID("anonymous"), security.FakePublicID("anonymous"))
}
func TestHandshakeTLS(t *testing.T) {
	testHandshake(t, SecurityTLS, clientID.PublicID(), serverID.PublicID())
}

func testConnect_Small(t *testing.T, security veyron2.VCSecurityLevel) {
	h, vc := New(security)
	defer h.Close()
	flow, err := vc.Connect()
	if err != nil {
		t.Fatal(err)
	}
	testFlowEcho(t, flow, 10)
}
func TestConnect_Small(t *testing.T)    { testConnect_Small(t, SecurityNone) }
func TestConnect_SmallTLS(t *testing.T) { testConnect_Small(t, SecurityTLS) }

func testConnect(t *testing.T, security veyron2.VCSecurityLevel) {
	h, vc := New(security)
	defer h.Close()
	flow, err := vc.Connect()
	if err != nil {
		t.Fatal(err)
	}
	testFlowEcho(t, flow, 10*DefaultBytesBufferedPerFlow)
}
func TestConnect(t *testing.T)    { testConnect(t, SecurityNone) }
func TestConnectTLS(t *testing.T) { testConnect(t, SecurityTLS) }

// helper function for testing concurrent operations on multiple flows over the
// same VC.  Such tests are most useful when running the race detector.
// (go test -race ...)
func testConcurrentFlows(t *testing.T, security veyron2.VCSecurityLevel, flows, gomaxprocs int) {
	mp := runtime.GOMAXPROCS(gomaxprocs)
	defer runtime.GOMAXPROCS(mp)
	h, vc := New(security)
	defer h.Close()

	done := make(chan bool)
	for i := 0; i < flows; i++ {
		go func(n int) {
			flow, err := vc.Connect()
			if err != nil {
				t.Error(err)
			} else {
				testFlowEcho(t, flow, (n+1)*DefaultBytesBufferedPerFlow)
			}
			done <- true
		}(i)
	}

	for i := 0; i < flows; i++ {
		<-done
	}
}

func TestConcurrentFlows_1(t *testing.T)    { testConcurrentFlows(t, SecurityNone, 10, 1) }
func TestConcurrentFlows_1TLS(t *testing.T) { testConcurrentFlows(t, SecurityTLS, 10, 1) }

func TestConcurrentFlows_10(t *testing.T)    { testConcurrentFlows(t, SecurityNone, 10, 10) }
func TestConcurrentFlows_10TLS(t *testing.T) { testConcurrentFlows(t, SecurityTLS, 10, 10) }

func testListen(t *testing.T, security veyron2.VCSecurityLevel) {
	data := "the dark knight"
	h, vc := New(security)
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

func testNewFlowAfterClose(t *testing.T, security veyron2.VCSecurityLevel) {
	h, _ := New(security)
	defer h.Close()
	h.VC.Close("reason")
	if err := h.VC.AcceptFlow(id.Flow(10)); err == nil {
		t.Fatalf("New flows should not be accepted once the VC is closed")
	}
}
func TestNewFlowAfterClose(t *testing.T)    { testNewFlowAfterClose(t, SecurityNone) }
func TestNewFlowAfterCloseTLS(t *testing.T) { testNewFlowAfterClose(t, SecurityTLS) }

func testConnectAfterClose(t *testing.T, security veyron2.VCSecurityLevel) {
	h, vc := New(security)
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
func New(security veyron2.VCSecurityLevel) (*helper, stream.VC) {
	clientH := &helper{bq: drrqueue.New(vc.MaxPayloadSizeBytes)}
	serverH := &helper{bq: drrqueue.New(vc.MaxPayloadSizeBytes)}
	clientH.otherEnd = serverH
	serverH.otherEnd = clientH

	clientEP := endpoint(naming.FixedRoutingID(0xcccccccccccccccc))
	serverEP := endpoint(naming.FixedRoutingID(0x5555555555555555))

	vci := id.VC(1234)

	clientParams := vc.Params{
		VCI:      vci,
		Dialed:   true,
		LocalEP:  clientEP,
		RemoteEP: serverEP,
		Pool:     iobuf.NewPool(0),
		Helper:   clientH,
	}
	serverParams := vc.Params{
		VCI:      vci,
		LocalEP:  serverEP,
		RemoteEP: clientEP,
		Pool:     iobuf.NewPool(0),
		Helper:   serverH,
	}

	clientH.VC = vc.InternalNew(clientParams)
	serverH.VC = vc.InternalNew(serverParams)
	clientH.AddReceiveBuffers(vci, vc.SharedFlowID, vc.DefaultBytesBufferedPerFlow)

	go clientH.pipeLoop(serverH.VC)
	go serverH.pipeLoop(clientH.VC)

	c := serverH.VC.HandshakeAcceptedVC(security, veyron2.LocalID(serverID))
	if err := clientH.VC.HandshakeDialedVC(security, veyron2.LocalID(clientID)); err != nil {
		panic(err)
	}
	hr := <-c
	if hr.Error != nil {
		panic(hr.Error)
	}
	go acceptLoop(hr.Listener)
	return clientH, clientH.VC
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

func randomString(size int) ([]byte, error) {
	b := make([]byte, size)
	if _, err := crand.Read(b); err != nil {
		return nil, err
	}
	return b, nil
}

type endpoint naming.RoutingID

func (e endpoint) Network() string             { return "test" }
func (e endpoint) String() string              { return naming.RoutingID(e).String() }
func (e endpoint) RoutingID() naming.RoutingID { return naming.RoutingID(e) }
func (e endpoint) Addr() net.Addr              { return nil }
