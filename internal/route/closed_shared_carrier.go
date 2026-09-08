package route

import (
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"errors"
	"net"
	"time"

	"github.com/quic-go/quic-go"
)

// ClosedSharedCarrierKind identifies the only two post-TLS states of a
// literal successor listener. It is selected before any ARDP bytes are read.
type ClosedSharedCarrierKind uint8

const (
	ClosedSharedDirect ClosedSharedCarrierKind = iota + 1
	ClosedSharedNode
)

// ClosedSharedCarrier is one classified v3 carrier. NodeKey is set only for
// the authenticated outer Node-Carrier state.
type ClosedSharedCarrier struct {
	Kind       ClosedSharedCarrierKind
	Connection net.Conn
	NodeKey    [32]byte
}

// ClosedSharedPeerVerifier accepts exactly a current State-authorized Node
// public key. It is called only for a single syntactically valid certificate.
type ClosedSharedPeerVerifier func([32]byte) bool

// ClosedSharedCarrierListener accepts direct role TLS and outer Node Carrier
// TLS from one literal endpoint. The caller owns the returned connection.
type ClosedSharedCarrierListener interface {
	Accept(context.Context, time.Time) (ClosedSharedCarrier, error)
	Close() error
}

// ListenClosedSharedCarrier creates the one v3 listener shared by direct role
// and Node Carrier TLS. The verifier must read current authenticated State at
// classification time; a rejected certificate is closed before ARDP work.
func ListenClosedSharedCarrier(profile CarrierProfile, endpoint string, certificate tls.Certificate, verify ClosedSharedPeerVerifier, handshakeLimit uint16) (ClosedSharedCarrierListener, error) {
	if !literalEndpoint(endpoint) || certificate.PrivateKey == nil || verify == nil || handshakeLimit == 0 || handshakeLimit > 16 {
		return nil, errors.New("closed shared carrier listener is invalid")
	}
	switch profile {
	case ClosedCarrierTCP:
		listener, err := net.Listen("tcp", endpoint)
		if err != nil {
			return nil, err
		}
		return &closedSharedTCPListener{listener: listener, certificate: certificate, verify: verify, handshakes: make(chan struct{}, handshakeLimit)}, nil
	case ClosedCarrierQUIC:
		listener, err := quic.ListenAddr(endpoint, closedSharedServerTLS(certificate), closedRoleQUICServerConfig())
		if err != nil {
			return nil, err
		}
		return &closedSharedQUICListener{listener: listener, verify: verify, handshakes: make(chan struct{}, handshakeLimit)}, nil
	default:
		return nil, errors.New("closed shared carrier profile is unsupported")
	}
}

type closedSharedTCPListener struct {
	listener    net.Listener
	certificate tls.Certificate
	verify      ClosedSharedPeerVerifier
	handshakes  chan struct{}
}

func (listener *closedSharedTCPListener) Accept(ctx context.Context, deadline time.Time) (ClosedSharedCarrier, error) {
	if ctx == nil || deadline.IsZero() || !time.Now().Before(deadline) {
		return ClosedSharedCarrier{}, errors.New("closed shared carrier acceptance is invalid")
	}
	var raw net.Conn
	for {
		var err error
		raw, err = listener.listener.Accept()
		if err != nil {
			return ClosedSharedCarrier{}, err
		}
		select {
		case listener.handshakes <- struct{}{}:
			defer func() { <-listener.handshakes }()
			goto admitted
		default:
			_ = raw.Close()
		}
	}
admitted:
	secured := tls.Server(raw, closedSharedServerTLS(listener.certificate))
	if err := secured.SetDeadline(deadline); err != nil {
		_ = raw.Close()
		return ClosedSharedCarrier{}, err
	}
	if err := secured.HandshakeContext(ctx); err != nil {
		_ = raw.Close()
		return ClosedSharedCarrier{}, err
	}
	classified, err := classifyClosedSharedTLS(secured.ConnectionState(), listener.verify)
	if err != nil {
		_ = raw.Close()
		return ClosedSharedCarrier{}, err
	}
	if err := secured.SetDeadline(time.Time{}); err != nil {
		_ = raw.Close()
		return ClosedSharedCarrier{}, err
	}
	classified.Connection = secured
	return classified, nil
}

func (listener *closedSharedTCPListener) Close() error { return listener.listener.Close() }

type closedSharedQUICListener struct {
	listener   *quic.Listener
	verify     ClosedSharedPeerVerifier
	handshakes chan struct{}
}

func (listener *closedSharedQUICListener) Accept(ctx context.Context, deadline time.Time) (ClosedSharedCarrier, error) {
	if ctx == nil || deadline.IsZero() || !time.Now().Before(deadline) {
		return ClosedSharedCarrier{}, errors.New("closed shared carrier acceptance is invalid")
	}
	var connection *quic.Conn
	for {
		var err error
		connection, err = listener.listener.Accept(ctx)
		if err != nil {
			return ClosedSharedCarrier{}, err
		}
		select {
		case listener.handshakes <- struct{}{}:
			defer func() { <-listener.handshakes }()
			goto admitted
		default:
			_ = connection.CloseWithError(1, "handshake-capacity-unavailable")
		}
	}
admitted:
	classified, err := classifyClosedSharedTLS(connection.ConnectionState().TLS, listener.verify)
	if err != nil {
		_ = connection.CloseWithError(1, "carrier-peer-invalid")
		return ClosedSharedCarrier{}, err
	}
	stream, err := connection.AcceptStream(ctx)
	if err != nil {
		_ = connection.CloseWithError(1, "carrier-stream-invalid")
		return ClosedSharedCarrier{}, err
	}
	carrier := &closedRoleQUICCarrier{stream: stream, connection: connection}
	if err := carrier.SetDeadline(deadline); err != nil {
		_ = carrier.Close()
		return ClosedSharedCarrier{}, err
	}
	if err := carrier.SetDeadline(time.Time{}); err != nil {
		_ = carrier.Close()
		return ClosedSharedCarrier{}, err
	}
	classified.Connection = carrier
	return classified, nil
}

func (listener *closedSharedQUICListener) Close() error { return listener.listener.Close() }

func closedSharedServerTLS(certificate tls.Certificate) *tls.Config {
	return &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13, Certificates: []tls.Certificate{certificate},
		ClientAuth: tls.RequestClientCert, SessionTicketsDisabled: true, NextProtos: []string{ClosedRouteProfile},
		CurvePreferences: []tls.CurveID{tls.X25519MLKEM768, tls.X25519}, VerifyConnection: func(state tls.ConnectionState) error {
			if state.Version != tls.VersionTLS13 || state.NegotiatedProtocol != ClosedRouteProfile || len(state.PeerCertificates) > 1 {
				return errors.New("closed shared carrier TLS state is invalid")
			}
			return nil
		}}
}

func classifyClosedSharedTLS(state tls.ConnectionState, verify ClosedSharedPeerVerifier) (ClosedSharedCarrier, error) {
	if state.Version != tls.VersionTLS13 || state.NegotiatedProtocol != ClosedRouteProfile || verify == nil {
		return ClosedSharedCarrier{}, errors.New("closed shared carrier TLS state is invalid")
	}
	if len(state.PeerCertificates) == 0 {
		return ClosedSharedCarrier{Kind: ClosedSharedDirect}, nil
	}
	if len(state.PeerCertificates) != 1 {
		return ClosedSharedCarrier{}, errors.New("closed shared carrier certificate count is invalid")
	}
	certificate := state.PeerCertificates[0]
	if time.Now().Before(certificate.NotBefore) || !time.Now().Before(certificate.NotAfter) {
		return ClosedSharedCarrier{}, errors.New("closed shared carrier certificate is not current")
	}
	public, ok := certificate.PublicKey.(ed25519.PublicKey)
	if !ok || len(public) != ed25519.PublicKeySize {
		return ClosedSharedCarrier{}, errors.New("closed shared carrier certificate key is invalid")
	}
	var key [32]byte
	copy(key[:], public)
	if !verify(key) {
		return ClosedSharedCarrier{}, errors.New("closed shared carrier peer is unavailable")
	}
	return ClosedSharedCarrier{Kind: ClosedSharedNode, NodeKey: key}, nil
}

var _ ClosedSharedCarrierListener = (*closedSharedTCPListener)(nil)
var _ ClosedSharedCarrierListener = (*closedSharedQUICListener)(nil)
