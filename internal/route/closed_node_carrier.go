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

// OpenClosedNodeCarrier opens only the State-selected v3 TCP/TLS or QUIC
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
	case ClosedCarrierQUIC:
		return openClosedQUICNodeCarrier(attempt, input)
	default:
		return nil, errors.New("closed Node Carrier profile is unsupported")
	}
}

func validateClosedNodeCarrierRequest(ctx context.Context, input ClosedNodeCarrierRequest) error {
	if ctx == nil || !literalEndpoint(input.Endpoint) || input.Certificate.PrivateKey == nil || input.ExpectedPeerKey == [32]byte{} ||
		input.Deadline.IsZero() || !time.Now().Before(input.Deadline) {
		return errors.New("closed Node Carrier request is invalid")
	}
	if input.CarrierProfile != ClosedCarrierTCP && input.CarrierProfile != ClosedCarrierQUIC {
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

func openClosedQUICNodeCarrier(ctx context.Context, input ClosedNodeCarrierRequest) (Carrier, error) {
	connection, err := quic.DialAddr(ctx, input.Endpoint, closedNodeClientTLS(input.Certificate, input.ExpectedPeerKey), closedNodeQUICConfig())
	if err != nil {
		return nil, err
	}
	if err := validClosedNodeTLSState(connection.ConnectionState().TLS, input.ExpectedPeerKey); err != nil {
		_ = connection.CloseWithError(1, "carrier-peer-invalid")
		return nil, err
	}
	stream, err := connection.OpenStreamSync(ctx)
	if err != nil {
		_ = connection.CloseWithError(1, "carrier-open-failed")
		return nil, err
	}
	lane := &quicNodeCarrier{stream: stream, connection: connection}
	if err := lane.SetDeadline(input.Deadline); err != nil {
		_ = lane.Close()
		return nil, err
	}
	if err := lane.SetDeadline(time.Time{}); err != nil {
		_ = lane.Close()
		return nil, err
	}
	return lane, nil
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

func closedNodeQUICConfig() *quic.Config {
	return &quic.Config{Versions: []quic.Version{quic.Version1}, HandshakeIdleTimeout: time.Second,
		MaxIdleTimeout: 5 * time.Second, KeepAlivePeriod: time.Second, MaxIncomingStreams: -1, MaxIncomingUniStreams: -1,
		InitialPacketSize: 1200, InitialStreamReceiveWindow: 32 << 10, MaxStreamReceiveWindow: 32 << 10,
		InitialConnectionReceiveWindow: 64 << 10, MaxConnectionReceiveWindow: 64 << 10,
		AllowConnectionWindowIncrease: func(*quic.Conn, uint64) bool { return false }, EnableDatagrams: false, Allow0RTT: false}
}
