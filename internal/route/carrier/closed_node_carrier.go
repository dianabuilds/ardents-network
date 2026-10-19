package carrier

import (
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"errors"
	"net"
	"time"
)

const (
	ClosedCarrierTCP   CarrierProfile = "ardents-carrier-tcp-tls-v2"
	ClosedCarrierQUIC  CarrierProfile = "ardents-carrier-quic-v2"
	ClosedRouteProfile                = "ardents-route-v3"
)

// ClosedNodeCarrierRequest is one exact successor Node-to-Node Carrier
// attempt. It intentionally has no generation-2 LegBinding, Endpoint
// credential, retry, or peer-choice input.
type ClosedNodeCarrierRequest struct {
	CarrierProfile  CarrierProfile
	Endpoint        string
	Certificate     tls.Certificate
	ExpectedPeerKey [32]byte
	Deadline        time.Time
}

// OpenClosedNodeCarrier opens only the State-selected v3 TCP/TLS
// Carrier. A negotiation, fallback, or generation-2 binding is unavailable.
func OpenClosedNodeCarrier(ctx context.Context, input ClosedNodeCarrierRequest) (Carrier, error) {
	if err := validateClosedNodeCarrierRequest(ctx, input); err != nil {
		return nil, err
	}
	attempt, cancel := context.WithDeadline(ctx, input.Deadline)
	defer cancel()
	switch input.CarrierProfile {
	case ClosedCarrierTCP:
		return openClosedTCPNodeCarrier(attempt, input)

	default:
		return nil, errors.New("closed Node Carrier profile is unsupported")
	}
}

func validateClosedNodeCarrierRequest(ctx context.Context, input ClosedNodeCarrierRequest) error {
	if ctx == nil || !LiteralEndpoint(input.Endpoint) || input.Certificate.PrivateKey == nil || input.ExpectedPeerKey == [32]byte{} ||
		input.Deadline.IsZero() || !time.Now().Before(input.Deadline) {
		return errors.New("closed Node Carrier request is invalid")
	}
	if input.CarrierProfile != ClosedCarrierTCP {
		return errors.New("closed Node Carrier profile is unsupported")
	}
	return nil
}

func openClosedTCPNodeCarrier(ctx context.Context, input ClosedNodeCarrierRequest) (Carrier, error) {
	raw, err := (&net.Dialer{}).DialContext(ctx, "tcp", input.Endpoint)
	if err != nil {
		return nil, err
	}
	secured := tls.Client(raw, closedNodeClientTLS(input.Certificate, input.ExpectedPeerKey))
	if err := secured.SetDeadline(input.Deadline); err != nil {
		_ = raw.Close()
		return nil, err
	}
	if err := secured.HandshakeContext(ctx); err != nil {
		_ = raw.Close()
		return nil, err
	}
	if err := validClosedNodeTLSState(secured.ConnectionState(), input.ExpectedPeerKey); err != nil {
		_ = raw.Close()
		return nil, err
	}
	if err := secured.SetDeadline(time.Time{}); err != nil {
		_ = raw.Close()
		return nil, err
	}
	return &closedTCPNodeTransport{Conn: secured}, nil
}

func closedNodeClientTLS(certificate tls.Certificate, expected [32]byte) *tls.Config {
	return &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13, Certificates: []tls.Certificate{certificate},
		InsecureSkipVerify: true, SessionTicketsDisabled: true, ClientSessionCache: nil, NextProtos: []string{ClosedRouteProfile},
		CurvePreferences: []tls.CurveID{tls.X25519MLKEM768, tls.X25519}, VerifyConnection: exactPeer(expected)}
}

func validClosedNodeTLSState(state tls.ConnectionState, expected [32]byte) error {
	if state.Version != tls.VersionTLS13 || state.NegotiatedProtocol != ClosedRouteProfile || len(state.PeerCertificates) != 1 {
		return errors.New("closed Node Carrier TLS state is invalid")
	}
	if expected != [32]byte{} {
		return exactPeer(expected)(state)
	}
	public, ok := state.PeerCertificates[0].PublicKey.(ed25519.PublicKey)
	if !ok || len(public) != ed25519.PublicKeySize {
		return errors.New("closed Node Carrier peer key is invalid")
	}
	return nil
}
