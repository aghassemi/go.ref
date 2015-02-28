package ipc

import (
	"net"
	"reflect"
	"sort"
	"testing"
	"time"

	"v.io/v23/config"
	"v.io/v23/context"
	"v.io/v23/ipc"
	"v.io/v23/naming"
	"v.io/v23/security"
	"v.io/v23/verror"
	"v.io/x/lib/vlog"

	"v.io/x/ref/lib/netstate"
	tsecurity "v.io/x/ref/lib/testutil/security"
	imanager "v.io/x/ref/runtimes/google/ipc/stream/manager"
	"v.io/x/ref/runtimes/google/ipc/stream/vc"
	inaming "v.io/x/ref/runtimes/google/naming"
	tnaming "v.io/x/ref/runtimes/google/testing/mocks/naming"
)

type noMethodsType struct{ Field string }

type fieldType struct {
	unexported string
}
type noExportedFieldsType struct{}

func (noExportedFieldsType) F(_ ipc.ServerCall, f fieldType) error { return nil }

type badObjectDispatcher struct{}

func (badObjectDispatcher) Lookup(suffix string) (interface{}, security.Authorizer, error) {
	return noMethodsType{}, nil, nil
}

// TestBadObject ensures that Serve handles bad reciver objects gracefully (in
// particular, it doesn't panic).
func TestBadObject(t *testing.T) {
	sm := imanager.InternalNew(naming.FixedRoutingID(0x555555555))
	defer sm.Shutdown()
	ns := tnaming.NewSimpleNamespace()
	ctx := testContext()
	server, err := testInternalNewServer(ctx, sm, ns)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Stop()

	if _, err := server.Listen(listenSpec); err != nil {
		t.Fatalf("Listen failed: %v", err)
	}
	if err := server.Serve("", nil, nil); err == nil {
		t.Fatal("should have failed")
	}
	if err := server.Serve("", new(noMethodsType), nil); err == nil {
		t.Fatal("should have failed")
	}
	if err := server.Serve("", new(noExportedFieldsType), nil); err == nil {
		t.Fatal("should have failed")
	}
	if err := server.ServeDispatcher("servername", badObjectDispatcher{}); err != nil {
		t.Fatalf("ServeDispatcher failed: %v", err)
	}
	client, err := InternalNewClient(sm, ns)
	if err != nil {
		t.Fatalf("InternalNewClient failed: %v", err)
	}
	ctx, _ = context.WithDeadline(testContext(), time.Now().Add(10*time.Second))
	call, err := client.StartCall(ctx, "servername", "SomeMethod", nil)
	if err != nil {
		t.Fatalf("StartCall failed: %v", err)
	}
	var result string
	if err := call.Finish(&result); err == nil {
		// TODO(caprita): Check the error type rather than
		// merely ensuring the test doesn't panic.
		t.Fatalf("should have failed")
	}
}

func TestServerArgs(t *testing.T) {
	sm := imanager.InternalNew(naming.FixedRoutingID(0x555555555))
	defer sm.Shutdown()
	ns := tnaming.NewSimpleNamespace()
	server, err := InternalNewServer(testContext(), sm, ns, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Stop()
	_, err = server.Listen(ipc.ListenSpec{})
	if !verror.Is(err, verror.ErrBadArg.ID) {
		t.Fatalf("expected a BadArg error: got %v", err)
	}
	_, err = server.Listen(ipc.ListenSpec{Addrs: ipc.ListenAddrs{{"tcp", "*:0"}}})
	if !verror.Is(err, verror.ErrBadArg.ID) {
		t.Fatalf("expected a BadArg error: got %v", err)
	}
	_, err = server.Listen(ipc.ListenSpec{
		Addrs: ipc.ListenAddrs{
			{"tcp", "*:0"},
			{"tcp", "127.0.0.1:0"},
		}})
	if verror.Is(err, verror.ErrBadArg.ID) {
		t.Fatalf("expected a BadArg error: got %v", err)
	}
	status := server.Status()
	if got, want := len(status.Errors), 1; got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
	_, err = server.Listen(ipc.ListenSpec{Addrs: ipc.ListenAddrs{{"tcp", "*:0"}}})
	if !verror.Is(err, verror.ErrBadArg.ID) {
		t.Fatalf("expected a BadArg error: got %v", err)
	}
	status = server.Status()
	if got, want := len(status.Errors), 1; got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

type statusServer struct{ ch chan struct{} }

func (s *statusServer) Hang(ctx ipc.ServerCall) error {
	<-s.ch
	return nil
}

func TestServerStatus(t *testing.T) {
	ctx := testContext()
	sm := imanager.InternalNew(naming.FixedRoutingID(0x555555555))
	defer sm.Shutdown()
	ns := tnaming.NewSimpleNamespace()
	principal := vc.LocalPrincipal{tsecurity.NewPrincipal("testServerStatus")}
	server, err := testInternalNewServer(ctx, sm, ns, principal)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Stop()

	status := server.Status()
	if got, want := status.State, ipc.ServerInit; got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
	server.Listen(ipc.ListenSpec{Addrs: ipc.ListenAddrs{{"tcp", "127.0.0.1:0"}}})
	status = server.Status()
	if got, want := status.State, ipc.ServerActive; got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
	serverChan := make(chan struct{})
	err = server.Serve("test", &statusServer{serverChan}, nil)
	if err != nil {
		t.Fatalf(err.Error())
	}
	status = server.Status()
	if got, want := status.State, ipc.ServerActive; got != want {
		t.Fatalf("got %s, want %s", got, want)
	}

	progress := make(chan error)

	client, err := InternalNewClient(sm, ns, principal)
	makeCall := func(ctx *context.T) {
		call, err := client.StartCall(ctx, "test", "Hang", nil)
		progress <- err
		progress <- call.Finish()
	}
	go makeCall(ctx)

	// Wait for RPC to start
	if err := <-progress; err != nil {
		t.Fatalf(err.Error())
	}

	// Stop server asynchronously
	go func() {
		err = server.Stop()
		if err != nil {
			t.Fatalf(err.Error())
		}
	}()

	// Server should enter 'ServerStopping' state.
	then := time.Now()
	for {
		status = server.Status()
		if got, want := status.State, ipc.ServerStopping; got != want {
			if time.Now().Sub(then) > time.Minute {
				t.Fatalf("got %s, want %s", got, want)
			}
		} else {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	// Server won't stop until the statusServer's hung method completes.
	close(serverChan)
	// Wait for RPC to finish
	if err := <-progress; err != nil {
		t.Fatalf(err.Error())
	}

	// Now that the RPC is done, the server should be able to stop.
	then = time.Now()
	for {
		status = server.Status()
		if got, want := status.State, ipc.ServerStopped; got != want {
			if time.Now().Sub(then) > time.Minute {
				t.Fatalf("got %s, want %s", got, want)
			}
		} else {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func TestServerStates(t *testing.T) {
	sm := imanager.InternalNew(naming.FixedRoutingID(0x555555555))
	defer sm.Shutdown()
	ns := tnaming.NewSimpleNamespace()
	ctx := testContext()

	expectBadState := func(err error) {
		if !verror.Is(err, verror.ErrBadState.ID) {
			t.Fatalf("%s: unexpected error: %v", loc(1), err)
		}
	}

	expectNoError := func(err error) {
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", loc(1), err)
		}
	}

	server, err := testInternalNewServer(ctx, sm, ns)
	expectNoError(err)
	defer server.Stop()

	expectState := func(s ipc.ServerState) {
		if got, want := server.Status().State, s; got != want {
			t.Fatalf("%s: got %s, want %s", loc(1), got, want)
		}
	}

	expectState(ipc.ServerInit)

	// Need to call Listen first.
	err = server.Serve("", &testServer{}, nil)
	expectBadState(err)
	err = server.AddName("a")
	expectBadState(err)

	_, err = server.Listen(ipc.ListenSpec{Addrs: ipc.ListenAddrs{{"tcp", "127.0.0.1:0"}}})
	expectNoError(err)

	expectState(ipc.ServerActive)

	err = server.Serve("", &testServer{}, nil)
	expectNoError(err)

	err = server.Serve("", &testServer{}, nil)
	expectBadState(err)

	expectState(ipc.ServerActive)

	err = server.AddName("a")
	expectNoError(err)

	expectState(ipc.ServerActive)

	server.RemoveName("a")

	expectState(ipc.ServerActive)

	err = server.Stop()
	expectNoError(err)
	err = server.Stop()
	expectNoError(err)

	err = server.AddName("a")
	expectBadState(err)
}

func TestMountStatus(t *testing.T) {
	sm := imanager.InternalNew(naming.FixedRoutingID(0x555555555))
	defer sm.Shutdown()
	ns := tnaming.NewSimpleNamespace()
	server, err := testInternalNewServer(testContext(), sm, ns)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Stop()

	eps, err := server.Listen(ipc.ListenSpec{
		Addrs: ipc.ListenAddrs{
			{"tcp", "127.0.0.1:0"},
			{"tcp", "127.0.0.1:0"},
		}})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(eps), 2; got != want {
		t.Fatalf("got %d, want %d", got, want)
	}
	if err = server.Serve("foo", &testServer{}, nil); err != nil {
		t.Fatal(err)
	}
	status := server.Status()
	if got, want := len(status.Mounts), 2; got != want {
		t.Fatalf("got %d, want %d", got, want)
	}
	servers := status.Mounts.Servers()
	if got, want := len(servers), 2; got != want {
		t.Fatalf("got %d, want %d", got, want)
	}
	if got, want := servers, endpointToStrings(eps); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}

	// Add a second name and we should now see 4 mounts, 2 for each name.
	if err := server.AddName("bar"); err != nil {
		t.Fatal(err)
	}
	status = server.Status()
	if got, want := len(status.Mounts), 4; got != want {
		t.Fatalf("got %d, want %d", got, want)
	}
	servers = status.Mounts.Servers()
	if got, want := len(servers), 2; got != want {
		t.Fatalf("got %d, want %d", got, want)
	}
	if got, want := servers, endpointToStrings(eps); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	names := status.Mounts.Names()
	if got, want := len(names), 2; got != want {
		t.Fatalf("got %d, want %d", got, want)
	}
	serversPerName := map[string][]string{}
	for _, ms := range status.Mounts {
		serversPerName[ms.Name] = append(serversPerName[ms.Name], ms.Server)
	}
	if got, want := len(serversPerName), 2; got != want {
		t.Fatalf("got %d, want %d", got, want)
	}
	for _, name := range []string{"foo", "bar"} {
		if got, want := len(serversPerName[name]), 2; got != want {
			t.Fatalf("got %d, want %d", got, want)
		}
	}
}

func updateHost(ep naming.Endpoint, address string) naming.Endpoint {
	niep := *(ep).(*inaming.Endpoint)
	niep.Address = address
	return &niep
}

func getIPAddrs(eps []naming.Endpoint) []ipc.Address {
	hosts := map[string]struct{}{}
	for _, ep := range eps {
		iep := (ep).(*inaming.Endpoint)
		h, _, _ := net.SplitHostPort(iep.Address)
		if len(h) > 0 {
			hosts[h] = struct{}{}
		}
	}
	addrs := []ipc.Address{}
	for h, _ := range hosts {
		a := &netstate.AddrIfc{Addr: &net.IPAddr{IP: net.ParseIP(h)}}
		addrs = append(addrs, a)
	}
	return addrs
}

func endpointToStrings(eps []naming.Endpoint) []string {
	r := []string{}
	for _, ep := range eps {
		r = append(r, ep.String())
	}
	sort.Strings(r)
	return r
}

func cmpEndpoints(got, want []naming.Endpoint) bool {
	if len(got) != len(want) {
		return false
	}
	return reflect.DeepEqual(endpointToStrings(got), endpointToStrings(want))
}

func getUniqPorts(eps []naming.Endpoint) []string {
	ports := map[string]struct{}{}
	for _, ep := range eps {
		iep := ep.(*inaming.Endpoint)
		_, p, _ := net.SplitHostPort(iep.Address)
		ports[p] = struct{}{}
	}
	r := []string{}
	for p, _ := range ports {
		r = append(r, p)
	}
	return r
}

func TestRoaming(t *testing.T) {
	sm := imanager.InternalNew(naming.FixedRoutingID(0x555555555))
	defer sm.Shutdown()
	ns := tnaming.NewSimpleNamespace()
	server, err := testInternalNewServer(testContext(), sm, ns)
	defer server.Stop()

	if err != nil {
		t.Fatal(err)
	}

	publisher := config.NewPublisher()
	roaming := make(chan config.Setting)
	stop, err := publisher.CreateStream("TestRoaming", "TestRoaming", roaming)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { publisher.Shutdown(); <-stop }()

	ipv4And6 := func(network string, addrs []ipc.Address) ([]ipc.Address, error) {
		accessible := netstate.AddrList(addrs)
		ipv4 := accessible.Filter(netstate.IsUnicastIPv4)
		ipv6 := accessible.Filter(netstate.IsUnicastIPv6)
		return append(ipv4, ipv6...), nil
	}
	spec := ipc.ListenSpec{
		Addrs: ipc.ListenAddrs{
			{"tcp", "*:0"},
			{"tcp", ":0"},
			{"tcp", ":0"},
		},
		StreamName:      "TestRoaming",
		StreamPublisher: publisher,
		AddressChooser:  ipv4And6,
	}

	eps, err := server.Listen(spec)
	if err != nil {
		t.Fatal(err)
	}
	if len(eps) == 0 {
		t.Fatal(err)
	}

	if err = server.Serve("foo", &testServer{}, nil); err != nil {
		t.Fatal(err)
	}
	if err = server.AddName("bar"); err != nil {
		t.Fatal(err)
	}

	status := server.Status()
	if got, want := status.Endpoints, eps; !cmpEndpoints(got, want) {
		t.Fatalf("got %d, want %d", got, want)
	}

	if got, want := len(status.Mounts), len(eps)*2; got != want {
		t.Fatalf("got %d, want %d", got, want)
	}

	n1 := &netstate.AddrIfc{Addr: &net.IPAddr{IP: net.ParseIP("1.1.1.1")}}
	n2 := &netstate.AddrIfc{Addr: &net.IPAddr{IP: net.ParseIP("2.2.2.2")}}

	watcher := make(chan ipc.NetworkChange, 10)
	server.WatchNetwork(watcher)
	defer close(watcher)

	roaming <- ipc.NewAddAddrsSetting([]ipc.Address{n1, n2})

	waitForChange := func() *ipc.NetworkChange {
		vlog.Infof("Waiting on %p", watcher)
		select {
		case c := <-watcher:
			return &c
		case <-time.After(time.Minute):
			t.Fatalf("timedout: %s", loc(1))
		}
		return nil
	}

	// We expect 4 changes, one for each IP per usable listen spec addr.
	change := waitForChange()
	if got, want := len(change.Changed), 4; got != want {
		t.Fatalf("got %d, want %d", got, want)
	}

	nepsA := make([]naming.Endpoint, len(eps))
	copy(nepsA, eps)
	for _, p := range getUniqPorts(eps) {
		nep1 := updateHost(eps[0], net.JoinHostPort("1.1.1.1", p))
		nep2 := updateHost(eps[0], net.JoinHostPort("2.2.2.2", p))
		nepsA = append(nepsA, []naming.Endpoint{nep1, nep2}...)
	}

	status = server.Status()
	if got, want := status.Endpoints, nepsA; !cmpEndpoints(got, want) {
		t.Fatalf("got %v, want %v [%d, %d]", got, want, len(got), len(want))
	}

	if got, want := len(status.Mounts), len(nepsA)*2; got != want {
		t.Fatalf("got %d, want %d", got, want)
	}
	if got, want := len(status.Mounts.Servers()), len(nepsA); got != want {
		t.Fatalf("got %d, want %d", got, want)
	}

	roaming <- ipc.NewRmAddrsSetting([]ipc.Address{n1})

	// We expect 2 changes, one for each usable listen spec addr.
	change = waitForChange()
	if got, want := len(change.Changed), 2; got != want {
		t.Fatalf("got %d, want %d", got, want)
	}

	nepsR := make([]naming.Endpoint, len(eps))
	copy(nepsR, eps)
	for _, p := range getUniqPorts(eps) {
		nep2 := updateHost(eps[0], net.JoinHostPort("2.2.2.2", p))
		nepsR = append(nepsR, nep2)
	}

	status = server.Status()
	if got, want := status.Endpoints, nepsR; !cmpEndpoints(got, want) {
		t.Fatalf("got %v, want %v [%d, %d]", got, want, len(got), len(want))
	}

	// Remove all addresses to mimic losing all connectivity.
	roaming <- ipc.NewRmAddrsSetting(getIPAddrs(nepsR))

	// We expect changes for all of the current endpoints
	change = waitForChange()
	if got, want := len(change.Changed), len(nepsR); got != want {
		t.Fatalf("got %d, want %d", got, want)
	}

	status = server.Status()
	if got, want := len(status.Mounts), 0; got != want {
		t.Fatalf("got %d, want %d", got, want)
	}

	roaming <- ipc.NewAddAddrsSetting([]ipc.Address{n1})
	// We expect 2 changes, one for each usable listen spec addr.
	change = waitForChange()
	if got, want := len(change.Changed), 2; got != want {
		t.Fatalf("got %d, want %d", got, want)
	}

}

func TestWatcherDeadlock(t *testing.T) {
	sm := imanager.InternalNew(naming.FixedRoutingID(0x555555555))
	defer sm.Shutdown()
	ns := tnaming.NewSimpleNamespace()
	server, err := testInternalNewServer(testContext(), sm, ns)
	defer server.Stop()

	if err != nil {
		t.Fatal(err)
	}

	publisher := config.NewPublisher()
	roaming := make(chan config.Setting)
	stop, err := publisher.CreateStream("TestWatcherDeadlock", "TestWatcherDeadlock", roaming)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { publisher.Shutdown(); <-stop }()

	spec := ipc.ListenSpec{
		Addrs: ipc.ListenAddrs{
			{"tcp", ":0"},
		},
		StreamName:      "TestWatcherDeadlock",
		StreamPublisher: publisher,
	}
	eps, err := server.Listen(spec)
	if err != nil {
		t.Fatal(err)
	}
	if err = server.Serve("foo", &testServer{}, nil); err != nil {
		t.Fatal(err)
	}

	// Set a watcher that we never read from - the intent is to make sure
	// that the listener still listens to changes even though there is no
	// goroutine to read from the watcher channel.
	watcher := make(chan ipc.NetworkChange, 0)
	server.WatchNetwork(watcher)
	defer close(watcher)

	// Remove all addresses to mimic losing all connectivity.
	roaming <- ipc.NewRmAddrsSetting(getIPAddrs(eps))

	// Add in two new addresses
	n1 := &netstate.AddrIfc{Addr: &net.IPAddr{IP: net.ParseIP("1.1.1.1")}}
	n2 := &netstate.AddrIfc{Addr: &net.IPAddr{IP: net.ParseIP("2.2.2.2")}}
	roaming <- ipc.NewAddAddrsSetting([]ipc.Address{n1, n2})

	neps := make([]naming.Endpoint, 0, len(eps))
	for _, p := range getUniqPorts(eps) {
		nep1 := updateHost(eps[0], net.JoinHostPort("1.1.1.1", p))
		nep2 := updateHost(eps[0], net.JoinHostPort("2.2.2.2", p))
		neps = append(neps, []naming.Endpoint{nep1, nep2}...)
	}
	then := time.Now()
	for {
		status := server.Status()
		if got, want := status.Endpoints, neps; cmpEndpoints(got, want) {
			break
		}
		time.Sleep(100 * time.Millisecond)
		if time.Now().Sub(then) > time.Minute {
			t.Fatalf("timed out waiting for changes to take effect")
		}
	}

}
