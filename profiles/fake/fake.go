package fake

import (
	"sync"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/ipc"

	_ "v.io/x/ref/profiles/internal/ipc/protocols/tcp"
	_ "v.io/x/ref/profiles/internal/ipc/protocols/ws"
	_ "v.io/x/ref/profiles/internal/ipc/protocols/wsh"
	"v.io/x/ref/profiles/internal/lib/websocket"
)

var (
	runtimeInfo struct {
		mu       sync.Mutex
		runtime  v23.Runtime  // GUARDED_BY mu
		ctx      *context.T   // GUARDED_BY mu
		shutdown v23.Shutdown // GUARDED_BY mu
	}
)

func init() {
	v23.RegisterProfileInit(Init)
	ipc.RegisterUnknownProtocol("wsh", websocket.HybridDial, websocket.HybridListener)
}

func Init(ctx *context.T) (v23.Runtime, *context.T, v23.Shutdown, error) {
	runtimeInfo.mu.Lock()
	defer runtimeInfo.mu.Unlock()
	if runtimeInfo.runtime != nil {
		shutdown := func() {
			runtimeInfo.mu.Lock()
			runtimeInfo.shutdown()
			runtimeInfo.runtime = nil
			runtimeInfo.ctx = nil
			runtimeInfo.shutdown = nil
			runtimeInfo.mu.Unlock()
		}
		return runtimeInfo.runtime, runtimeInfo.ctx, shutdown, nil
	}
	return new(ctx)
}

// InjectRuntime allows packages to inject whichever runtime, ctx, and shutdown.
// This allows a package that needs different runtimes in tests to swap them as needed.
// The injected runtime will be valid until the shutdown returned from v23.Init is called.
func InjectRuntime(runtime v23.Runtime, ctx *context.T, shutdown v23.Shutdown) {
	runtimeInfo.mu.Lock()
	runtimeInfo.runtime = runtime
	runtimeInfo.ctx = ctx
	runtimeInfo.shutdown = shutdown
	runtimeInfo.mu.Unlock()
}
