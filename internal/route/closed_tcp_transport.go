package route

import (
	"crypto/tls"
	"net"
	"sync"
	"sync/atomic"
)

// closedTCPNodeTransport owns physical Node-Carrier retirement. Carrier Close
// aborts all multiplexed lanes; it is not a directional EOF or Service success.
// Closing the actual socket interrupts I/O without attempting another TLS
// notification after a peer reset. Lane owners still join all their workers.
type closedTCPNodeTransport struct {
	*tls.Conn
	closed  atomic.Bool
	once    sync.Once
	failure error
}

func (carrier *closedTCPNodeTransport) Close() error {
	carrier.once.Do(func() {
		carrier.closed.Store(true)
		carrier.failure = carrier.Conn.NetConn().Close()
	})
	return carrier.failure
}

func (carrier *closedTCPNodeTransport) Read(value []byte) (int, error) {
	if carrier.closed.Load() {
		return 0, net.ErrClosed
	}
	return carrier.Conn.Read(value)
}

func (carrier *closedTCPNodeTransport) Write(value []byte) (int, error) {
	if carrier.closed.Load() {
		return 0, net.ErrClosed
	}
	return carrier.Conn.Write(value)
}
