package transport

import (
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"errors"
	"net"
	"time"
)

// ClosedSharedCarrierKind distinguishes the only two accepted post-TLS states
// before any Route frame is read. It supplies no role-operation authority.
type ClosedSharedCarrierKind uint8

const (
	ClosedSharedDirect ClosedSharedCarrierKind = iota + 1
	ClosedSharedNode
)

// ClosedSharedCarrier is one classified transport. NodeKey is present only
// after mutual outer Node authentication. Its receiver owns Connection.
type ClosedSharedCarrier struct {
	Kind       ClosedSharedCarrierKind
	Connection net.Conn
	NodeKey    [32]byte
}

// ClosedSharedPeerVerifier checks a single valid certificate's exact key using
// current authenticated State. The transport neither caches nor creates State.
type ClosedSharedPeerVerifier func([32]byte) bool

// ClosedSharedCarrierListener owns one socket and finite handshake capacity.
// Accept bounds authentication from arrival, not listener idle time, by at most
// ten seconds and its caller's context. Close interrupts idle acceptance;
// composition joins returned connections before releasing domain resources.
type ClosedSharedCarrierListener interface {
	Accept(context.Context, time.Duration) (ClosedSharedCarrier, error)
	Close() error
}

type sharedPeerFailure struct{ cause error }

func (failure *sharedPeerFailure) Error() string { return failure.cause.Error() }
func (failure *sharedPeerFailure) Unwrap() error { return failure.cause }

// IsClosedSharedPeerFailure identifies a connection's authentication/handshake
// refusal, distinct from listener failure, without discarding its cause.
func IsClosedSharedPeerFailure(err error) bool {
	if err == nil {
		return false
	}
	if failure, ok := err.(*sharedPeerFailure); ok {
		return failure.cause != nil
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		children := joined.Unwrap()
		if len(children) == 0 {
			return false
		}
		for _, child := range children {
			if !IsClosedSharedPeerFailure(child) {
				return false
			}
		}
		return true
	}
	if wrapped, ok := err.(interface{ Unwrap() error }); ok {
		return IsClosedSharedPeerFailure(wrapped.Unwrap())
	}
	return false
}

// MarkSharedPeerFailure retains a physical adapter's original refused-handshake
// cause. Categorization does not permit an operation or replace joined cleanup.
func MarkSharedPeerFailure(err error) error {
	if err == nil {
		return nil
	}
	return &sharedPeerFailure{cause: err}
}

// ValidateSharedListener checks input before either adapter opens a socket.
func ValidateSharedListener(endpoint string, certificate tls.Certificate, verify ClosedSharedPeerVerifier, handshakeLimit uint16) error {
	if !LiteralEndpoint(endpoint) || certificate.PrivateKey == nil || verify == nil || handshakeLimit == 0 || handshakeLimit > 16 {
		return errors.New("closed shared carrier listener is invalid")
	}
	return nil
}

// ValidateSharedAcceptance checks the same pre-input handshake bounds on both
// physical adapters; their native acceptance and interruption remain separate.
func ValidateSharedAcceptance(ctx context.Context, handshakeTimeout time.Duration) error {
	if ctx == nil || handshakeTimeout <= 0 || handshakeTimeout > 10*time.Second {
		return errors.New("closed shared carrier acceptance is invalid")
	}
	return nil
}

// SharedServerTLS permits certificate-free direct roles or one mutually
// authenticated Node. Classification must precede Route framing on both.
func SharedServerTLS(certificate tls.Certificate) *tls.Config {
	return &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13, Certificates: []tls.Certificate{certificate},
		ClientAuth: tls.RequestClientCert, SessionTicketsDisabled: true, NextProtos: []string{ClosedRouteProfile},
		CurvePreferences: []tls.CurveID{tls.X25519MLKEM768, tls.X25519}, VerifyConnection: func(state tls.ConnectionState) error {
			if state.Version != tls.VersionTLS13 || state.NegotiatedProtocol != ClosedRouteProfile || len(state.PeerCertificates) > 1 {
				return errors.New("closed shared carrier TLS state is invalid")
			}
			return nil
		}}
}

// ClassifySharedTLS applies the same principal and certificate rules to TLS and
// QUIC. A direct connection conveys no stable client identity or Admission right.
func ClassifySharedTLS(state tls.ConnectionState, verify ClosedSharedPeerVerifier) (ClosedSharedCarrier, error) {
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
