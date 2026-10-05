package quic

import (
	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
	"github.com/quic-go/quic-go"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

type roleCarrier struct {
	stream     *quic.Stream
	connection *quic.Conn
	once       sync.Once
	closeErr   error
	closed     atomic.Bool
}

func (carrier *roleCarrier) Read(value []byte) (int, error) {
	if carrier.closed.Load() {
		return 0, net.ErrClosed
	}
	n, err := carrier.stream.Read(value)
	return n, classifyIOError(err)
}
func (carrier *roleCarrier) Write(value []byte) (int, error) {
	// A late child must not attempt a new frame after this owner retired the
	// physical stream. Preserve errors from writes already in flight; only a
	// new call after local Close receives the standard closed sentinel.
	if carrier.closed.Load() {
		return 0, net.ErrClosed
	}
	n, err := carrier.stream.Write(value)
	return n, classifyIOError(err)
}
func (carrier *roleCarrier) LocalAddr() net.Addr  { return carrier.connection.LocalAddr() }
func (carrier *roleCarrier) RemoteAddr() net.Addr { return carrier.connection.RemoteAddr() }
func (carrier *roleCarrier) SetDeadline(deadline time.Time) error {
	return classifyIOError(carrier.stream.SetDeadline(quicDeadline(deadline)))
}
func (carrier *roleCarrier) SetReadDeadline(deadline time.Time) error {
	return classifyIOError(carrier.stream.SetReadDeadline(quicDeadline(deadline)))
}
func (carrier *roleCarrier) SetWriteDeadline(deadline time.Time) error {
	return classifyIOError(carrier.stream.SetWriteDeadline(quicDeadline(deadline)))
}
func (carrier *roleCarrier) Close() error {
	carrier.once.Do(func() {
		carrier.closed.Store(true)
		// Close is physical interruption, not directional FIN. Calling
		// Stream.Close here could race an already selected physical Write.
		carrier.closeErr = classifyIOError(carrier.connection.CloseWithError(0, "role-close"))
	})
	return carrier.closeErr
}

func roleConfig() *quic.Config { return nodeConfig() }
func serverConfig() *quic.Config {
	config := roleConfig()
	config.MaxIncomingStreams = 1
	return config
}

var _ net.Conn = (*roleCarrier)(nil)

// RoleTLSExporter is available only on a direct role stream. Node streams use
// a separate type and cannot acquire this capability from their shared TLS.
func (carrier *roleCarrier) RoleTLSExporter() (transport.ClosedTLSExporter, error) {
	return transport.RoleExporter(carrier.connection.ConnectionState().TLS)
}
