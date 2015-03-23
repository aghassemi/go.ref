package websocket

import (
	"v.io/v23/rpc"

	"v.io/x/ref/profiles/internal/lib/websocket"
)

func init() {
	// ws, ws4, ws6 represent websocket protocol instances.
	rpc.RegisterProtocol("ws", websocket.Dial, websocket.Listener, "ws4", "ws6")
	rpc.RegisterProtocol("ws4", websocket.Dial, websocket.Listener)
	rpc.RegisterProtocol("ws6", websocket.Dial, websocket.Listener)
}