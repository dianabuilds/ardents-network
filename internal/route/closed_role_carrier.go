package route

import (
	"context"
	"crypto/tls"
	"errors"
	"github.com/quic-go/quic-go"
	"net"
	"sync"
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

// ClosedRoleCarrierListener accepts one selected direct role carrier. Its
// returned connection has already completed the role TLS authentication.
type ClosedRoleCarrierListener interface {
	Accept(context.Context, time.Time) (net.Conn, error)
	Close() error
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

// ListenClosedRoleCarrier binds one literal State-selected direct role endpoint.
func ListenClosedRoleCarrier(profile CarrierProfile, endpoint string, certificate tls.Certificate) (ClosedRoleCarrierListener, error) {
	if !literalEndpoint(endpoint) || certificate.PrivateKey == nil {
		return nil, errors.New("closed role carrier listener is invalid")
	}
	switch profile {
	case ClosedCarrierTCP:
		listener, err := net.Listen("tcp", endpoint)
		if err != nil {
			return nil, err
		}
		return &closedRoleTCPListener{listener: listener, certificate: certificate}, nil
	case ClosedCarrierQUIC:
		listener, err := quic.ListenAddr(endpoint, closedRoleServerTLS(certificate), closedRoleQUICServerConfig())
		if err != nil {
			return nil, err
		}
		return &closedRoleQUICListener{listener: listener}, nil
	default:
		return nil, errors.New("closed role carrier profile is unsupported")
	}
}

type closedRoleTCPListener struct {
	listener    net.Listener
	certificate tls.Certificate
}

func (listener *closedRoleTCPListener) Accept(ctx context.Context, deadline time.Time) (net.Conn, error) {
	if ctx == nil || deadline.IsZero() || !time.Now().Before(deadline) {
		return nil, errors.New("closed role carrier acceptance is invalid")
	}
	raw, err := listener.listener.Accept()
	if err != nil {
		return nil, err
	}
	secured, err := AcceptClosedRoleTLS(ctx, raw, listener.certificate, deadline)
	if err != nil {
		_ = raw.Close()
		return nil, err
	}
	return secured, nil
}
func (listener *closedRoleTCPListener) Close() error { return listener.listener.Close() }

type closedRoleQUICListener struct{ listener *quic.Listener }

func (listener *closedRoleQUICListener) Accept(ctx context.Context, deadline time.Time) (net.Conn, error) {
	if ctx == nil || deadline.IsZero() || !time.Now().Before(deadline) {
		return nil, errors.New("closed role carrier acceptance is invalid")
	}
	connection, err := listener.listener.Accept(ctx)
	if err != nil {
		return nil, err
	}
	if err := validClosedRoleTLSState(connection.ConnectionState().TLS, [32]byte{}, true); err != nil {
		_ = connection.CloseWithError(1, "role-peer-invalid")
		return nil, err
	}
	attempt, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()
	stream, err := connection.AcceptStream(attempt)
	if err != nil {
		_ = connection.CloseWithError(1, "role-stream-invalid")
		return nil, err
	}
	carrier := &closedRoleQUICCarrier{stream: stream, connection: connection}
	if err := carrier.SetDeadline(deadline); err != nil {
		_ = carrier.Close()
		return nil, err
	}
	if err := carrier.SetDeadline(time.Time{}); err != nil {
		_ = carrier.Close()
		return nil, err
	}
	return carrier, nil
}
func (listener *closedRoleQUICListener) Close() error { return listener.listener.Close() }

type closedRoleQUICCarrier struct {
	stream     *quic.Stream
	connection *quic.Conn
	once       sync.Once
	closeErr   error
}

func (carrier *closedRoleQUICCarrier) Read(value []byte) (int, error) {
	return carrier.stream.Read(value)
}
func (carrier *closedRoleQUICCarrier) Write(value []byte) (int, error) {
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
var _ ClosedRoleCarrierListener = (*closedRoleTCPListener)(nil)
var _ ClosedRoleCarrierListener = (*closedRoleQUICListener)(nil)
