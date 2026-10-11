package tls

import (
	"crypto/tls"
	"errors"
	"net"
	"sync"
	"sync/atomic"

	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
)

// nodeCarrier owns physical Node-Carrier retirement. Carrier Close
// aborts all multiplexed lanes; it is not a directional EOF or Service success.
// Closing the actual socket interrupts I/O without attempting another TLS
// notification after a peer reset. Lane owners still join all their workers.
type nodeCarrier struct {
	*tls.Conn
	closed  atomic.Bool
	once    sync.Once
	failure error
	socket  *nativeSocket
}

func (carrier *nodeCarrier) Close() error {
	carrier.once.Do(func() {
		carrier.closed.Store(true)
		carrier.failure = carrier.Conn.NetConn().Close()
	})
	return carrier.failure
}

func (carrier *nodeCarrier) Read(value []byte) (int, error) {
	if carrier.closed.Load() {
		return 0, net.ErrClosed
	}
	return carrier.Conn.Read(value)
}

func (carrier *nodeCarrier) Write(value []byte) (int, error) {
	if carrier.closed.Load() {
		if carrier.socket != nil {
			observed := carrier.socket.observe()
			if observed.closed && !observed.failed && observed.active == 0 {
				return 0, transport.MarkUnstartedWrite(net.ErrClosed)
			}
		}
		return 0, net.ErrClosed
	}
	if carrier.socket == nil {
		return carrier.Conn.Write(value)
	}
	before := carrier.socket.observe()
	n, err := carrier.Conn.Write(value)
	after := carrier.socket.observe()
	var refusal *socketWriteRefusal
	if n == 0 && errors.As(err, &refusal) && after.closed && !before.failed && !after.failed &&
		before.active == 0 && after.active == 0 && before.attempts == after.attempts {
		err = transport.MarkUnstartedWrite(err)
	}
	return n, err
}
