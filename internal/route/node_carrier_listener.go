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

// PendingCarrier is one accepted but not yet Node-admitted transport. Node
// owns the finite admission reservation before calling Authenticate.
type PendingCarrier interface {
	Authenticate(context.Context, time.Time) (Carrier, tls.ConnectionState, error)
	Close() error
}

// CarrierListener accepts one exact configured profile. It exposes no socket,
// QUIC connection, profile negotiation, or alternate listener.
type CarrierListener interface {
	Accept(context.Context) (PendingCarrier, error)
	Close() error
}

// ListenNodeCarrier binds one literal endpoint for one selected profile.
func ListenNodeCarrier(profile CarrierProfile, endpoint string, certificate tls.Certificate) (CarrierListener, error) {
	if !literalEndpoint(endpoint) || certificate.PrivateKey == nil {
		return nil, carrierFailure(CarrierFailureIncompatible, errors.New("native Node Carrier listener configuration is invalid"))
	}
	switch profile {
	case CarrierTCP:
		listener, err := net.Listen("tcp", endpoint)
		if err != nil {
			return nil, classifyCarrierFailure(err)
		}
		return &tcpNodeCarrierListener{listener: listener, certificate: certificate}, nil
	case CarrierQUIC:
		listener, err := quic.ListenAddrEarly(endpoint, nativeNodeServerTLS(certificate), nodeQUICServerConfig())
		if err != nil {
			return nil, classifyCarrierFailure(err)
		}
		return &quicNodeCarrierListener{listener: listener}, nil
	default:
		return nil, carrierFailure(CarrierFailureIncompatible, errors.New("native Node Carrier listener Profile is unsupported"))
	}
}

type tcpNodeCarrierListener struct {
	listener    net.Listener
	certificate tls.Certificate
}

func (listener *tcpNodeCarrierListener) Accept(ctx context.Context) (PendingCarrier, error) {
	if ctx == nil {
		return nil, carrierFailure(CarrierFailureIncompatible, errors.New("native Node Carrier accept context is missing"))
	}
	raw, err := listener.listener.Accept()
	if err != nil {
		return nil, classifyCarrierFailure(err)
	}
	return &tcpPendingCarrier{raw: raw, certificate: listener.certificate}, nil
}

func (listener *tcpNodeCarrierListener) Close() error { return listener.listener.Close() }

type tcpPendingCarrier struct {
	raw         net.Conn
	certificate tls.Certificate
}

func (pending *tcpPendingCarrier) Authenticate(ctx context.Context, deadline time.Time) (Carrier, tls.ConnectionState, error) {
	if ctx == nil || deadline.IsZero() {
		return nil, tls.ConnectionState{}, carrierFailure(CarrierFailureIncompatible, errors.New("native Node Carrier authentication input is invalid"))
	}
	if !time.Now().Before(deadline) {
		return nil, tls.ConnectionState{}, carrierFailure(CarrierFailureStale, errors.New("native Node Carrier authentication deadline is stale"))
	}
	secured := tls.Server(pending.raw, nativeNodeServerTLS(pending.certificate))
	if err := secured.SetDeadline(deadline); err != nil {
		return nil, tls.ConnectionState{}, classifyCarrierFailure(err)
	}
	if err := secured.HandshakeContext(ctx); err != nil {
		return nil, tls.ConnectionState{}, classifyCarrierFailure(err)
	}
	return authenticatedCarrier(secured), secured.ConnectionState(), nil
}

func (pending *tcpPendingCarrier) Close() error { return pending.raw.Close() }

type quicNodeCarrierListener struct{ listener *quic.EarlyListener }

func (listener *quicNodeCarrierListener) Accept(ctx context.Context) (PendingCarrier, error) {
	if ctx == nil {
		return nil, carrierFailure(CarrierFailureIncompatible, errors.New("native Node Carrier accept context is missing"))
	}
	connection, err := listener.listener.Accept(ctx)
	if err != nil {
		return nil, classifyCarrierFailure(err)
	}
	return &quicPendingCarrier{connection: connection}, nil
}

func (listener *quicNodeCarrierListener) Close() error { return listener.listener.Close() }

type quicPendingCarrier struct{ connection *quic.Conn }

func (pending *quicPendingCarrier) Authenticate(ctx context.Context, deadline time.Time) (Carrier, tls.ConnectionState, error) {
	if ctx == nil || deadline.IsZero() {
		return nil, tls.ConnectionState{}, carrierFailure(CarrierFailureIncompatible, errors.New("native Node Carrier authentication input is invalid"))
	}
	if !time.Now().Before(deadline) {
		return nil, tls.ConnectionState{}, carrierFailure(CarrierFailureStale, errors.New("native Node Carrier authentication deadline is stale"))
	}
	select {
	case <-ctx.Done():
		return nil, tls.ConnectionState{}, classifyCarrierFailure(ctx.Err())
	case <-pending.connection.Context().Done():
		return nil, tls.ConnectionState{}, classifyCarrierFailure(context.Cause(pending.connection.Context()))
	case <-pending.connection.HandshakeComplete():
	}
	stream, err := pending.connection.AcceptStream(ctx)
	if err != nil {
		return nil, tls.ConnectionState{}, classifyCarrierFailure(err)
	}
	lane := &quicNodeCarrier{stream: stream, connection: pending.connection}
	if err := lane.SetDeadline(deadline); err != nil {
		return nil, tls.ConnectionState{}, classifyCarrierFailure(err)
	}
	return authenticatedCarrier(lane), pending.connection.ConnectionState().TLS, nil
}

func (pending *quicPendingCarrier) Close() error {
	return pending.connection.CloseWithError(1, "carrier-admission-closed")
}

func nativeNodeServerTLS(certificate tls.Certificate) *tls.Config {
	return &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13, Certificates: []tls.Certificate{certificate},
		ClientAuth: tls.RequireAnyClientCert, SessionTicketsDisabled: true, NextProtos: []string{Profile},
		VerifyConnection: func(state tls.ConnectionState) error {
			if len(state.PeerCertificates) != 1 {
				return errors.New("native Node Carrier client certificate is missing")
			}
			public, ok := state.PeerCertificates[0].PublicKey.(ed25519.PublicKey)
			if !ok || len(public) != ed25519.PublicKeySize {
				return errors.New("native Node Carrier client key is invalid")
			}
			return nil
		}}
}

func nodeQUICServerConfig() *quic.Config {
	profile := nodeQUICConfig()
	profile.MaxIncomingStreams = 1
	return profile
}
