package websocket

import (
	"net"
	"time"

	"v.io/core/veyron/lib/tcputil"
)

// TODO(jhahn): Figure out a way for this mapping to be shared.
var mapWebSocketToTCP = map[string]string{"ws": "tcp", "ws4": "tcp4", "ws6": "tcp6", "wsh": "tcp", "wsh4": "tcp4", "wsh6": "tcp6", "tcp": "tcp", "tcp4": "tcp4", "tcp6": "tcp6"}

// HybridDial returns net.Conn that can be used with a HybridListener but
// always uses tcp. A client must specifically elect to use websockets by
// calling websocket.Dialer. The returned net.Conn will report 'tcp' as its
// Network.
func HybridDial(network, address string, timeout time.Duration) (net.Conn, error) {
	tcp := mapWebSocketToTCP[network]
	conn, err := net.DialTimeout(tcp, address, timeout)
	if err != nil {
		return nil, err
	}
	if err := tcputil.EnableTCPKeepAlive(conn); err != nil {
		return nil, err
	}
	return conn, nil
}

// HybridListener returns a net.Listener that supports both tcp and
// websockets over the same, single, port. A listen address of
// --veyron.tcp.protocol=wsh --veyron.tcp.address=127.0.0.1:8101 means
// that port 8101 can accept connections that use either tcp or websocket.
// The listener looks at the first 4 bytes of the incoming data stream
// to decide if it's a websocket protocol or not. These must be 'GET ' for
// websockets, all other protocols must guarantee to not send 'GET ' as the
// first four bytes of the payload.
func HybridListener(protocol, address string) (net.Listener, error) {
	return listener(protocol, address, true)
}
