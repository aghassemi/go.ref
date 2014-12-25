// Package wsh registers the websocket 'hybrid' protocol.
// We prefer to use tcp whenever we can to avoid the overhead of websockets.
package wsh

import (
	"v.io/veyron/veyron2/ipc/stream"

	"v.io/veyron/veyron/lib/websocket"
)

func init() {
	for _, p := range []string{"wsh", "wsh4", "wsh6"} {
		stream.RegisterProtocol(p, websocket.HybridDial, websocket.HybridListener)
	}
}
