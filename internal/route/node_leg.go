package route

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"time"
)

// NodeLegRequest is one State-pinned adjacent Node leg. It is usable only by
// a transit duty that already owns its peer selection and resource admission.
// It contains no Service, Application, or retry policy.
type NodeLegRequest struct {
	Endpoint        string
	Certificate     tls.Certificate
	ExpectedPeerKey [32]byte
	Binding         LegBinding
	Deadline        time.Time
}

// OpenNodeLeg dials one literal State-authorized endpoint, completes native
// mutual TLS, and confirms the reciprocal LegBinding before returning its
// ordered carrier. It never selects or retries another peer.
func OpenNodeLeg(ctx context.Context, input NodeLegRequest) (net.Conn, error) {
	if ctx == nil || !literalEndpoint(input.Endpoint) || input.Certificate.PrivateKey == nil || input.ExpectedPeerKey == [32]byte{} ||
		input.Deadline.IsZero() || !time.Now().Before(input.Deadline) || validLegBinding(input.Binding) != nil {
		return nil, errors.New("native Node leg request is invalid")
	}
	connection, err := (&net.Dialer{}).DialContext(ctx, "tcp", input.Endpoint)
	if err != nil {
		return nil, err
	}
	secured := tls.Client(connection, nativeNodeTLS(input.Certificate, input.ExpectedPeerKey))
	if err := secured.SetDeadline(input.Deadline); err != nil {
		_ = connection.Close()
		return nil, err
	}
	if err := secured.HandshakeContext(ctx); err != nil {
		_ = connection.Close()
		return nil, err
	}
	if secured.ConnectionState().NegotiatedProtocol != Profile {
		_ = connection.Close()
		return nil, errors.New("native Node TLS ALPN is invalid")
	}
	if err := ConfirmNodeLegBinding(secured, input.Binding); err != nil {
		_ = connection.Close()
		return nil, err
	}
	if err := secured.SetDeadline(time.Time{}); err != nil {
		_ = connection.Close()
		return nil, err
	}
	return secured, nil
}

func nativeNodeTLS(certificate tls.Certificate, expected [32]byte) *tls.Config {
	return &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13, Certificates: []tls.Certificate{certificate},
		InsecureSkipVerify: true, SessionTicketsDisabled: true, NextProtos: []string{Profile}, VerifyConnection: exactPeer(expected)}
}
