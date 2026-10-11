package quic

import (
	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/quic-go/quic-go"
)

// nodeCarrier is the authenticated QUIC byte lane used by the current
// closed Node dial.
type nodeCarrier struct {
	stream      *quic.Stream
	connection  *quic.Conn
	closeOnce   sync.Once
	closeErr    error
	closed      atomic.Bool
	closeReason string
}

func (carrier *nodeCarrier) Read(buffer []byte) (int, error) {
	if carrier.closed.Load() {
		return 0, closedIOError(carrier.connection)
	}
	n, err := carrier.stream.Read(buffer)
	return n, classifyIOError(err)
}

func (carrier *nodeCarrier) Write(buffer []byte) (int, error) {
	if carrier.closed.Load() {
		return 0, closedIOError(carrier.connection)
	}
	n, err := carrier.stream.Write(buffer)
	return n, classifyIOError(err)
}

func (carrier *nodeCarrier) SetDeadline(deadline time.Time) error {
	return classifyIOError(carrier.stream.SetDeadline(quicDeadline(deadline)))
}

func (carrier *nodeCarrier) Close() error {
	carrier.closeOnce.Do(func() {
		carrier.closed.Store(true)
		// Physical retirement interrupts outstanding Read/Write. Stream.Close
		// is a directional FIN and forbids concurrent Write; it is not needed
		// before aborting this original connection. Borrowers still join I/O.
		reason := carrier.closeReason
		if reason == "" {
			reason = "carrier-close"
		}
		carrier.closeErr = classifyIOError(carrier.connection.CloseWithError(0, reason))
	})
	return carrier.closeErr
}

var _ transport.Carrier = (*nodeCarrier)(nil)
var _ net.Conn = (*nodeCarrier)(nil)

func (carrier *nodeCarrier) LocalAddr() net.Addr  { return carrier.connection.LocalAddr() }
func (carrier *nodeCarrier) RemoteAddr() net.Addr { return carrier.connection.RemoteAddr() }
func (carrier *nodeCarrier) SetReadDeadline(deadline time.Time) error {
	return classifyIOError(carrier.stream.SetReadDeadline(quicDeadline(deadline)))
}

func (carrier *nodeCarrier) SetWriteDeadline(deadline time.Time) error {
	return classifyIOError(carrier.stream.SetWriteDeadline(quicDeadline(deadline)))
}
