package namespace_test

import (
	"runtime"
	"runtime/debug"
	"sync"
	"testing"
	"time"

	"v.io/core/veyron2"
	"v.io/core/veyron2/context"
	"v.io/core/veyron2/ipc"
	"v.io/core/veyron2/naming"
	"v.io/core/veyron2/options"
	"v.io/core/veyron2/security"
	"v.io/core/veyron2/verror"
	"v.io/core/veyron2/vlog"

	"v.io/core/veyron/lib/testutil"
	tsecurity "v.io/core/veyron/lib/testutil/security"
	_ "v.io/core/veyron/profiles"
	"v.io/core/veyron/runtimes/google/naming/namespace"
	vsecurity "v.io/core/veyron/security"
	service "v.io/core/veyron/services/mounttable/lib"
)

func init() {
	testutil.Init()
}

func createContexts(t *testing.T) (sc, c *context.T, cleanup func()) {
	ctx, shutdown := testutil.InitForTest()
	var err error
	if sc, err = veyron2.SetPrincipal(ctx, tsecurity.NewPrincipal("test-blessing")); err != nil {
		t.Fatal(err)
	}
	if c, err = veyron2.SetPrincipal(ctx, tsecurity.NewPrincipal("test-blessing")); err != nil {
		t.Fatal(err)
	}
	return sc, c, shutdown
}

func boom(t *testing.T, f string, v ...interface{}) {
	t.Logf(f, v...)
	t.Fatal(string(debug.Stack()))
}

// N squared but who cares, this is a little test.
// Ignores dups.
func contains(container, contained []string) bool {
L:
	for _, d := range contained {
		for _, r := range container {
			if r == d {
				continue L
			}
		}
		return false
	}
	return true
}

func compare(t *testing.T, caller, name string, got, want []string) {
	// Compare ignoring dups.
	if !contains(got, want) || !contains(want, got) {
		boom(t, "%s: %q: got %v, want %v", caller, name, got, want)
	}
}

func doGlob(t *testing.T, ctx *context.T, ns naming.Namespace, pattern string, limit int) []string {
	var replies []string
	rc, err := ns.Glob(ctx, pattern)
	if err != nil {
		boom(t, "Glob(%s): %s", pattern, err)
	}
	for s := range rc {
		switch v := s.(type) {
		case *naming.MountEntry:
			replies = append(replies, v.Name)
			if limit > 0 && len(replies) > limit {
				boom(t, "Glob returns too many results, perhaps not limiting recursion")
			}
		}
	}
	return replies
}

type testServer struct {
	suffix string
}

func (testServer) KnockKnock(ctx ipc.ServerContext) string {
	return "Who's there?"
}

// testServer has the following namespace:
// "" -> {level1} -> {level2}
func (t *testServer) GlobChildren__(ipc.ServerContext) (<-chan string, error) {
	ch := make(chan string, 1)
	switch t.suffix {
	case "":
		ch <- "level1"
	case "level1":
		ch <- "level2"
	default:
		return nil, nil
	}
	close(ch)
	return ch, nil
}

type allowEveryoneAuthorizer struct{}

func (allowEveryoneAuthorizer) Authorize(security.Context) error { return nil }

type dispatcher struct{}

func (d *dispatcher) Lookup(suffix string) (interface{}, security.Authorizer, error) {
	return &testServer{suffix}, allowEveryoneAuthorizer{}, nil
}

func knockKnock(t *testing.T, ctx *context.T, name string) {
	client := veyron2.GetClient(ctx)
	call, err := client.StartCall(ctx, name, "KnockKnock", nil)
	if err != nil {
		boom(t, "StartCall failed: %s", err)
	}
	var result string
	if err := call.Finish(&result); err != nil {
		boom(t, "Finish returned an error: %s", err)
	}
	if result != "Who's there?" {
		boom(t, "Wrong result: %v", result)
	}
}

func doResolveTest(t *testing.T, fname string, f func(*context.T, string, ...naming.ResolveOpt) (*naming.MountEntry, error), ctx *context.T, name string, want []string, opts ...naming.ResolveOpt) {
	me, err := f(ctx, name, opts...)
	if err != nil {
		boom(t, "Failed to %s %s: %s", fname, name, err)
	}
	compare(t, fname, name, me.Names(), want)
}

func testResolveToMountTable(t *testing.T, ctx *context.T, ns naming.Namespace, name string, want ...string) {
	doResolveTest(t, "ResolveToMountTable", ns.ResolveToMountTable, ctx, name, want)
}

func testResolveToMountTableWithPattern(t *testing.T, ctx *context.T, ns naming.Namespace, name string, pattern naming.ResolveOpt, want ...string) {
	doResolveTest(t, "ResolveToMountTable", ns.ResolveToMountTable, ctx, name, want, pattern)
}

func testResolve(t *testing.T, ctx *context.T, ns naming.Namespace, name string, want ...string) {
	doResolveTest(t, "Resolve", ns.Resolve, ctx, name, want)
}

func testResolveWithPattern(t *testing.T, ctx *context.T, ns naming.Namespace, name string, pattern naming.ResolveOpt, want ...string) {
	doResolveTest(t, "Resolve", ns.Resolve, ctx, name, want, pattern)
}

type serverEntry struct {
	mountPoint string
	server     ipc.Server
	endpoint   naming.Endpoint
	name       string
}

func runServer(t *testing.T, ctx *context.T, disp ipc.Dispatcher, mountPoint string) *serverEntry {
	return run(t, ctx, disp, mountPoint, false)
}

func runMT(t *testing.T, ctx *context.T, mountPoint string) *serverEntry {
	mtd, err := service.NewMountTableDispatcher("")
	if err != nil {
		boom(t, "NewMountTableDispatcher returned error: %v", err)
	}
	return run(t, ctx, mtd, mountPoint, true)
}

func run(t *testing.T, ctx *context.T, disp ipc.Dispatcher, mountPoint string, mt bool) *serverEntry {
	s, err := veyron2.NewServer(ctx, options.ServesMountTable(mt))
	if err != nil {
		boom(t, "r.NewServer: %s", err)
	}
	// Add a mount table server.
	// Start serving on a loopback address.
	eps, err := s.Listen(veyron2.GetListenSpec(ctx))
	if err != nil {
		boom(t, "Failed to Listen: %s", err)
	}
	if err := s.ServeDispatcher(mountPoint, disp); err != nil {
		boom(t, "Failed to serve mount table at %s: %s", mountPoint, err)
	}
	return &serverEntry{mountPoint: mountPoint, server: s, endpoint: eps[0], name: eps[0].Name()}
}

const (
	mt1MP = "mt1"
	mt2MP = "mt2"
	mt3MP = "mt3"
	mt4MP = "mt4"
	mt5MP = "mt5"
	j1MP  = "joke1"
	j2MP  = "joke2"
	j3MP  = "joke3"

	ttl = 100 * time.Second
)

// runMountTables creates a root mountable with some mount tables mounted
// in it: mt{1,2,3,4,5}
func runMountTables(t *testing.T, ctx *context.T) (*serverEntry, map[string]*serverEntry) {
	root := runMT(t, ctx, "")
	veyron2.GetNamespace(ctx).SetRoots(root.name)
	t.Logf("mountTable %q -> %s", root.mountPoint, root.endpoint)

	mps := make(map[string]*serverEntry)
	for _, mp := range []string{mt1MP, mt2MP, mt3MP, mt4MP, mt5MP} {
		m := runMT(t, ctx, mp)
		t.Logf("mountTable %q -> %s", mp, m.endpoint)
		mps[mp] = m
	}
	return root, mps
}

// createNamespace creates a hierarchy of mounttables and servers
// as follows:
// /mt1, /mt2, /mt3, /mt4, /mt5, /joke1, /joke2, /joke3.
// That is, mt1 is a mount table mounted in the root mount table,
// joke1 is a server mounted in the root mount table.
func createNamespace(t *testing.T, ctx *context.T) (*serverEntry, map[string]*serverEntry, map[string]*serverEntry, func()) {
	root, mts := runMountTables(t, ctx)
	jokes := make(map[string]*serverEntry)
	// Let's run some non-mount table services.
	for _, j := range []string{j1MP, j2MP, j3MP} {
		disp := &dispatcher{}
		jokes[j] = runServer(t, ctx, disp, j)
	}
	return root, mts, jokes, func() {
		for _, s := range jokes {
			s.server.Stop()
		}
		for _, s := range mts {
			s.server.Stop()
		}
		root.server.Stop()
	}
}

// runNestedMountTables creates some nested mount tables in the hierarchy
// created by createNamespace as follows:
// /mt4/foo, /mt4/foo/bar and /mt4/baz where foo, bar and baz are mount tables.
func runNestedMountTables(t *testing.T, ctx *context.T, mts map[string]*serverEntry) {
	ns := veyron2.GetNamespace(ctx)
	// Set up some nested mounts and verify resolution.
	for _, m := range []string{"mt4/foo", "mt4/foo/bar"} {
		mts[m] = runMT(t, ctx, m)
	}

	// Use a global name for a mount, rather than a relative one.
	// We directly mount baz into the mt4/foo mount table.
	globalMP := naming.JoinAddressName(mts["mt4/foo"].name, "baz")
	mts["baz"] = runMT(t, ctx, "baz")
	if err := ns.Mount(ctx, globalMP, mts["baz"].name, ttl); err != nil {
		boom(t, "Failed to Mount %s: %s", globalMP, err)
	}
}

// TestNamespaceCommon tests common use of the Namespace library
// against a root mount table and some mount tables mounted on it.
func TestNamespaceCommon(t *testing.T) {
	_, c, cleanup := createContexts(t)
	defer cleanup()

	root, mts, jokes, stopper := createNamespace(t, c)
	defer stopper()
	ns := veyron2.GetNamespace(c)

	// All of the initial mounts are served by the root mounttable
	// and hence ResolveToMountTable should return the root mountable
	// as the address portion of the terminal name for those mounttables.
	testResolveToMountTable(t, c, ns, "", root.name)
	for _, m := range []string{mt2MP, mt3MP, mt5MP} {
		rootMT := naming.Join(root.name, m)
		// All of these mount tables are hosted by the root mount table
		testResolveToMountTable(t, c, ns, m, rootMT)

		// The server registered for each mount point is a mount table
		testResolve(t, c, ns, m, mts[m].name)

		// ResolveToMountTable will walk through to the sub MountTables
		mtbar := naming.Join(m, "bar")
		subMT := naming.Join(mts[m].name, "bar")
		testResolveToMountTable(t, c, ns, mtbar, subMT)
	}

	for _, j := range []string{j1MP, j2MP, j3MP} {
		testResolve(t, c, ns, j, jokes[j].name)
	}
}

// TestNamespaceDetails tests more detailed use of the Namespace library.
func TestNamespaceDetails(t *testing.T) {
	sc, c, cleanup := createContexts(t)
	defer cleanup()

	root, mts, _, stopper := createNamespace(t, sc)
	defer stopper()

	ns := veyron2.GetNamespace(c)
	ns.SetRoots(root.name)

	// /mt2 is not an endpoint. Thus, the example below will fail.
	mt3Server := mts[mt3MP].name
	mt2a := "/mt2/a"
	if err := ns.Mount(c, mt2a, mt3Server, ttl); verror.Is(err, naming.ErrNoSuchName.ID) {
		boom(t, "Successfully mounted %s - expected an err %v, not %v", mt2a, naming.ErrNoSuchName, err)
	}

	// Mount using the relative name.
	// This means walk through mt2 if it already exists and mount within
	// the lower level mount table, if the name doesn't exist we'll create
	// a new name for it.
	mt2a = "mt2/a"
	if err := ns.Mount(c, mt2a, mt3Server, ttl); err != nil {
		boom(t, "Failed to Mount %s: %s", mt2a, err)
	}

	mt2mt := naming.Join(mts[mt2MP].name, "a")
	// The mt2/a is served by the mt2 mount table
	testResolveToMountTable(t, c, ns, mt2a, mt2mt)
	// The server for mt2a is mt3server from the second mount above.
	testResolve(t, c, ns, mt2a, mt3Server)

	// Add two more mounts. The // should be stripped off of the
	// second.
	for _, mp := range []struct{ name, server string }{
		{"mt2", mts[mt4MP].name},
		{"mt2//", mts[mt5MP].name},
	} {
		if err := ns.Mount(c, mp.name, mp.server, ttl, naming.ServesMountTableOpt(true)); err != nil {
			boom(t, "Failed to Mount %s: %s", mp.name, err)
		}
	}

	names := []string{naming.JoinAddressName(mts[mt4MP].name, "a"),
		naming.JoinAddressName(mts[mt5MP].name, "a")}
	names = append(names, naming.JoinAddressName(mts[mt2MP].name, "a"))
	// We now have 3 mount tables prepared to serve mt2/a
	testResolveToMountTable(t, c, ns, "mt2/a", names...)
	names = []string{mts[mt4MP].name, mts[mt5MP].name}
	names = append(names, mts[mt2MP].name)
	testResolve(t, c, ns, "mt2", names...)
}

// TestNestedMounts tests some more deeply nested mounts
func TestNestedMounts(t *testing.T) {
	sc, c, cleanup := createContexts(t)
	defer cleanup()

	root, mts, _, stopper := createNamespace(t, sc)
	runNestedMountTables(t, sc, mts)
	defer stopper()

	ns := veyron2.GetNamespace(c)
	ns.SetRoots(root.name)

	// Set up some nested mounts and verify resolution.
	for _, m := range []string{"mt4/foo", "mt4/foo/bar"} {
		testResolve(t, c, ns, m, mts[m].name)
	}

	testResolveToMountTable(t, c, ns, "mt4/foo",
		naming.JoinAddressName(mts[mt4MP].name, "foo"))
	testResolveToMountTable(t, c, ns, "mt4/foo/bar",
		naming.JoinAddressName(mts["mt4/foo"].name, "bar"))
	testResolveToMountTable(t, c, ns, "mt4/foo/baz",
		naming.JoinAddressName(mts["mt4/foo"].name, "baz"))
}

// TestServers tests invoking RPCs on simple servers
func TestServers(t *testing.T) {
	sc, c, cleanup := createContexts(t)
	defer cleanup()

	root, mts, jokes, stopper := createNamespace(t, sc)
	defer stopper()
	ns := veyron2.GetNamespace(c)
	ns.SetRoots(root.name)

	// Let's run some non-mount table services
	for _, j := range []string{j1MP, j2MP, j3MP} {
		testResolve(t, c, ns, j, jokes[j].name)
		knockKnock(t, c, j)
		globalName := naming.JoinAddressName(mts["mt4"].name, j)
		disp := &dispatcher{}
		gj := "g_" + j
		jokes[gj] = runServer(t, c, disp, globalName)
		testResolve(t, c, ns, "mt4/"+j, jokes[gj].name)
		knockKnock(t, c, "mt4/"+j)
		testResolveToMountTable(t, c, ns, "mt4/"+j, globalName)
		testResolveToMountTable(t, c, ns, "mt4/"+j+"/garbage", globalName+"/garbage")
	}
}

// TestGlob tests some glob patterns.
func TestGlob(t *testing.T) {
	sc, c, cleanup := createContexts(t)
	defer cleanup()

	root, mts, _, stopper := createNamespace(t, sc)
	runNestedMountTables(t, sc, mts)
	defer stopper()
	ns := veyron2.GetNamespace(c)
	ns.SetRoots(root.name)

	tln := []string{"baz", "mt1", "mt2", "mt3", "mt4", "mt5", "joke1", "joke2", "joke3"}
	barbaz := []string{"mt4/foo/bar", "mt4/foo/baz"}
	level12 := []string{"joke1/level1", "joke1/level1/level2", "joke2/level1", "joke2/level1/level2", "joke3/level1", "joke3/level1/level2"}
	foo := append([]string{"mt4/foo"}, barbaz...)
	foo = append(foo, level12...)
	// Try various globs.
	globTests := []struct {
		pattern  string
		expected []string
	}{
		{"*", tln},
		{"x", []string{}},
		{"m*", []string{"mt1", "mt2", "mt3", "mt4", "mt5"}},
		{"mt[2,3]", []string{"mt2", "mt3"}},
		{"*z", []string{"baz"}},
		{"joke1/*", []string{"joke1/level1"}},
		{"j?ke1/level1/*", []string{"joke1/level1/level2"}},
		{"joke1/level1/*", []string{"joke1/level1/level2"}},
		{"joke1/level1/level2/...", []string{"joke1/level1/level2"}},
		{"...", append(append(tln, foo...), "")},
		{"*/...", append(tln, foo...)},
		{"*/foo/*", barbaz},
		{"*/*/*z", []string{"mt4/foo/baz"}},
		{"*/f??/*z", []string{"mt4/foo/baz"}},
		{"mt4/foo/baz", []string{"mt4/foo/baz"}},
	}
	for _, test := range globTests {
		out := doGlob(t, c, ns, test.pattern, 0)
		compare(t, "Glob", test.pattern, out, test.expected)
		// Do the same with a full rooted name.
		out = doGlob(t, c, ns, naming.JoinAddressName(root.name, test.pattern), 0)
		var expectedWithRoot []string
		for _, s := range test.expected {
			expectedWithRoot = append(expectedWithRoot, naming.JoinAddressName(root.name, s))
		}
		compare(t, "Glob", test.pattern, out, expectedWithRoot)
	}
}

type GlobbableServer struct {
	callCount int
	mu        sync.Mutex
}

func (g *GlobbableServer) Glob__(ipc.ServerContext, string) (<-chan naming.VDLMountEntry, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.callCount++
	return nil, nil
}

func (g *GlobbableServer) GetAndResetCount() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	cnt := g.callCount
	g.callCount = 0

	return cnt
}

// TestGlobEarlyStop tests that Glob doesn't query terminal servers with finished patterns.
func TestGlobEarlyStop(t *testing.T) {
	sc, c, cleanup := createContexts(t)
	defer cleanup()

	root, mts, _, stopper := createNamespace(t, sc)
	runNestedMountTables(t, sc, mts)
	defer stopper()

	globServer := &GlobbableServer{}
	name := naming.JoinAddressName(mts["mt4/foo/bar"].name, "glob")
	runningGlobServer := runServer(t, c, testutil.LeafDispatcher(globServer, nil), name)
	defer runningGlobServer.server.Stop()

	ns := veyron2.GetNamespace(c)
	ns.SetRoots(root.name)

	tests := []struct {
		pattern       string
		expectedCalls int
		expected      []string
	}{
		{"mt4/foo/bar/glob", 0, []string{"mt4/foo/bar/glob"}},
		{"mt4/foo/bar/glob/...", 1, []string{"mt4/foo/bar/glob"}},
		{"mt4/foo/bar/glob/*", 1, nil},
		{"mt4/foo/bar/***", 0, []string{"mt4/foo/bar", "mt4/foo/bar/glob"}},
		{"mt4/foo/bar/...", 1, []string{"mt4/foo/bar", "mt4/foo/bar/glob"}},
		{"mt4/foo/bar/*", 0, []string{"mt4/foo/bar/glob"}},
		{"mt4/***/bar/***", 0, []string{"mt4/foo/bar", "mt4/foo/bar/glob"}},
		{"mt4/*/bar/***", 0, []string{"mt4/foo/bar", "mt4/foo/bar/glob"}},
	}
	// Test allowing the tests to descend into leaves.
	for _, test := range tests {
		out := doGlob(t, c, ns, test.pattern, 0)
		compare(t, "Glob", test.pattern, out, test.expected)
		if calls := globServer.GetAndResetCount(); calls != test.expectedCalls {
			boom(t, "Wrong number of Glob calls to terminal server got: %d want: %d.", calls, test.expectedCalls)
		}
	}
}

func TestCycles(t *testing.T) {
	sc, c, cleanup := createContexts(t)
	defer cleanup()

	root, _, _, stopper := createNamespace(t, sc)
	defer stopper()
	ns := veyron2.GetNamespace(c)
	ns.SetRoots(root.name)

	c1 := runMT(t, c, "c1")
	c2 := runMT(t, c, "c2")
	c3 := runMT(t, c, "c3")
	defer c1.server.Stop()
	defer c2.server.Stop()
	defer c3.server.Stop()

	m := "c1/c2"
	if err := ns.Mount(c, m, c1.name, ttl, naming.ServesMountTableOpt(true)); err != nil {
		boom(t, "Failed to Mount %s: %s", "c1/c2", err)
	}

	m = "c1/c2/c3"
	if err := ns.Mount(c, m, c3.name, ttl, naming.ServesMountTableOpt(true)); err != nil {
		boom(t, "Failed to Mount %s: %s", m, err)
	}

	m = "c1/c3/c4"
	if err := ns.Mount(c, m, c1.name, ttl, naming.ServesMountTableOpt(true)); err != nil {
		boom(t, "Failed to Mount %s: %s", m, err)
	}

	// Since c1 was mounted with the Serve call, it will have both the tcp and ws endpoints.
	testResolve(t, c, ns, "c1", c1.name)
	testResolve(t, c, ns, "c1/c2", c1.name)
	testResolve(t, c, ns, "c1/c3", c3.name)
	testResolve(t, c, ns, "c1/c3/c4", c1.name)
	testResolve(t, c, ns, "c1/c3/c4/c3/c4", c1.name)
	cycle := "c3/c4"
	for i := 0; i < 40; i++ {
		cycle += "/c3/c4"
	}
	if _, err := ns.Resolve(c, "c1/"+cycle); !verror.Is(err, naming.ErrResolutionDepthExceeded.ID) {
		boom(t, "Failed to detect cycle")
	}

	// Perform the glob with a response length limit.
	doGlob(t, c, ns, "c1/...", 1000)
}

// TestGoroutineLeaks tests for leaking goroutines - we have many:-(
func TestGoroutineLeaks(t *testing.T) {
	t.Skip()
	sc, _, cleanup := createContexts(t)
	defer cleanup()

	_, _, _, stopper := createNamespace(t, sc)
	defer func() {
		vlog.Infof("%d goroutines:", runtime.NumGoroutine())
	}()
	defer stopper()
	defer func() {
		vlog.Infof("%d goroutines:", runtime.NumGoroutine())
	}()
	//panic("this will show up lots of goroutine+channel leaks!!!!")
}

func TestBadRoots(t *testing.T) {
	if _, err := namespace.New(); err != nil {
		t.Errorf("namespace.New should not have failed with no roots")
	}
	if _, err := namespace.New("not a rooted name"); err == nil {
		t.Errorf("namespace.New should have failed with an unrooted name")
	}
}

func bless(blesser, delegate security.Principal, extension string) {
	b, err := blesser.Bless(delegate.PublicKey(), blesser.BlessingStore().Default(), extension, security.UnconstrainedUse())
	if err != nil {
		panic(err)
	}
	delegate.BlessingStore().SetDefault(b)
}

func TestRootBlessing(t *testing.T) {
	c, cc, cleanup := createContexts(t)
	defer cleanup()

	proot, err := vsecurity.NewPrincipal()
	if err != nil {
		panic(err)
	}
	b, err := proot.BlessSelf("root")
	if err != nil {
		panic(err)
	}
	proot.BlessingStore().SetDefault(b)

	sprincipal := veyron2.GetPrincipal(c)
	cprincipal := veyron2.GetPrincipal(cc)
	bless(proot, sprincipal, "server")
	bless(proot, cprincipal, "client")
	cprincipal.AddToRoots(proot.BlessingStore().Default())
	sprincipal.AddToRoots(proot.BlessingStore().Default())

	root, mts, _, stopper := createNamespace(t, c)
	defer stopper()
	ns := veyron2.GetNamespace(c)

	name := naming.Join(root.name, mt2MP)
	// First check with a non-matching blessing pattern.
	_, err = ns.Resolve(c, name, naming.RootBlessingPatternOpt("root/foobar"))
	if !verror.Is(err, verror.NotTrusted.ID) {
		t.Errorf("Resolve expected NotTrusted error, got %v", err)
	}
	_, err = ns.ResolveToMountTable(c, name, naming.RootBlessingPatternOpt("root/foobar"))
	if !verror.Is(err, verror.NotTrusted.ID) {
		t.Errorf("ResolveToMountTable expected NotTrusted error, got %v", err)
	}

	// Now check a matching pattern.
	testResolveWithPattern(t, c, ns, name, naming.RootBlessingPatternOpt("root/server"), mts[mt2MP].name)
	testResolveToMountTableWithPattern(t, c, ns, name, naming.RootBlessingPatternOpt("root/server"), name)

	// After successful lookup it should be cached, so the pattern doesn't matter.
	testResolveWithPattern(t, c, ns, name, naming.RootBlessingPatternOpt("root/foobar"), mts[mt2MP].name)
}
