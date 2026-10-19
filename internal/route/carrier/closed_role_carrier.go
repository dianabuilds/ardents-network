package carrier

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
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
	if !LiteralEndpoint(endpoint) || certificate.PrivateKey == nil {
		return nil, errors.New("closed role carrier listener is invalid")
	}
	switch profile {
	case ClosedCarrierTCP:
		listener, err := net.Listen("tcp", endpoint)
		if err != nil {
			return nil, err
		}
		return &closedRoleTCPListener{listener: listener, certificate: certificate}, nil

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

var _ ClosedRoleCarrierListener = (*closedRoleTCPListener)(nil)
