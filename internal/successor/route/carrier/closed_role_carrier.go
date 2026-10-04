//go:build linux

package carrier

import (
	"crypto/tls"
	"errors"
	"github.com/quic-go/quic-go"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// ClosedRoleCarrierRequest names one State-selected direct role transport.
// There is no URL, DNS name, root pool, client certificate or profile fallback.
type ClosedRoleCarrierRequest struct {
	CarrierProfile CarrierProfile
	Endpoint       string
	ExpectedServer [32]byte
	Deadline       time.Time
}

// ClosedRoleTLSExporter returns the exporter of the already authenticated
// direct role TLS channel.  Admission keeps the derived bytes locally; the
// connection, its peer address, and any TLS state never become Route inputs.
func ClosedRoleTLSExporter(connection net.Conn) (ClosedTLSExporter, error) {
	if connection == nil {
		return nil, errors.New("closed role TLS exporter is unavailable")
	}
	var state tls.ConnectionState
	switch secured := connection.(type) {
	case *tls.Conn:
		state = secured.ConnectionState()
	case *closedRoleQUICCarrier:
		state = secured.connection.ConnectionState().TLS
	default:
		return nil, errors.New("closed role TLS exporter is unavailable")
	}
	if state.Version != tls.VersionTLS13 || state.NegotiatedProtocol != ClosedRouteProfile {
		return nil, errors.New("closed role TLS exporter is unavailable")
	}
	return func(label string, context []byte, length int) ([]byte, error) {
		return state.ExportKeyingMaterial(label, context, length)
	}, nil
}

type closedRoleQUICCarrier struct {
	stream     *quic.Stream
	connection *quic.Conn
	once       sync.Once
	closeErr   error
	closed     atomic.Bool
}

func (carrier *closedRoleQUICCarrier) Read(value []byte) (int, error) {
	return carrier.stream.Read(value)
}
func (carrier *closedRoleQUICCarrier) Write(value []byte) (int, error) {
	// A late child must not attempt a new frame after this owner retired the
	// physical stream. Preserve errors from writes already in flight; only a
	// new call after local Close receives the standard closed sentinel.
	if carrier.closed.Load() {
		return 0, net.ErrClosed
	}
	return carrier.stream.Write(value)
}
func (carrier *closedRoleQUICCarrier) LocalAddr() net.Addr  { return carrier.connection.LocalAddr() }
func (carrier *closedRoleQUICCarrier) RemoteAddr() net.Addr { return carrier.connection.RemoteAddr() }
func (carrier *closedRoleQUICCarrier) SetDeadline(deadline time.Time) error {
	return carrier.stream.SetDeadline(deadline)
}
func (carrier *closedRoleQUICCarrier) SetReadDeadline(deadline time.Time) error {
	return carrier.stream.SetReadDeadline(deadline)
}
func (carrier *closedRoleQUICCarrier) SetWriteDeadline(deadline time.Time) error {
	return carrier.stream.SetWriteDeadline(deadline)
}
func (carrier *closedRoleQUICCarrier) Close() error {
	carrier.once.Do(func() {
		carrier.closed.Store(true)
		carrier.closeErr = errors.Join(carrier.stream.Close(), carrier.connection.CloseWithError(0, "role-close"))
	})
	return carrier.closeErr
}

func closedRoleQUICConfig() *quic.Config { return closedNodeQUICConfig() }
func closedRoleQUICServerConfig() *quic.Config {
	config := closedRoleQUICConfig()
	config.MaxIncomingStreams = 1
	return config
}

var _ net.Conn = (*closedRoleQUICCarrier)(nil)
