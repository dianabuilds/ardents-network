package tls

import (
	"context"
	"crypto/tls"
	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
	"net"
	"time"
)

// ListenShared opens one literal TCP socket for direct role and mutual Node TLS.
// It owns bounded handshakes; accepted connections remain the caller's lifetime.
func ListenShared(endpoint string, certificate tls.Certificate, verify transport.ClosedSharedPeerVerifier, handshakeLimit uint16) (transport.ClosedSharedCarrierListener, error) {
	if err := transport.ValidateSharedListener(endpoint, certificate, verify, handshakeLimit); err != nil {
		return nil, err
	}
	listener, err := net.Listen("tcp", endpoint)
	if err != nil {
		return nil, err
	}
	return &sharedListener{listener: listener, certificate: certificate, verify: verify, handshakes: make(chan struct{}, handshakeLimit)}, nil
}

type sharedListener struct {
	listener    net.Listener
	certificate tls.Certificate
	verify      transport.ClosedSharedPeerVerifier
	handshakes  chan struct{}
}

func (listener *sharedListener) Accept(ctx context.Context, handshakeTimeout time.Duration) (transport.ClosedSharedCarrier, error) {
	if err := transport.ValidateSharedAcceptance(ctx, handshakeTimeout); err != nil {
		return transport.ClosedSharedCarrier{}, err
	}
	var raw net.Conn
	for {
		var err error
		raw, err = listener.listener.Accept()
		if err != nil {
			return transport.ClosedSharedCarrier{}, err
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
	deadline := time.Now().Add(handshakeTimeout)
	secured := tls.Server(raw, transport.SharedServerTLS(listener.certificate))
	if err := secured.SetDeadline(deadline); err != nil {
		_ = raw.Close()
		return transport.ClosedSharedCarrier{}, transport.MarkSharedPeerFailure(err)
	}
	if err := secured.HandshakeContext(ctx); err != nil {
		_ = raw.Close()
		return transport.ClosedSharedCarrier{}, transport.MarkSharedPeerFailure(err)
	}
	classified, err := transport.ClassifySharedTLS(secured.ConnectionState(), listener.verify)
	if err != nil {
		_ = raw.Close()
		return transport.ClosedSharedCarrier{}, transport.MarkSharedPeerFailure(err)
	}
	if err := secured.SetDeadline(time.Time{}); err != nil {
		_ = raw.Close()
		return transport.ClosedSharedCarrier{}, transport.MarkSharedPeerFailure(err)
	}
	classified.Connection = secured
	if classified.Kind == transport.ClosedSharedNode {
		classified.Connection = &nodeCarrier{Conn: secured}
	}
	return classified, nil
}

func (listener *sharedListener) Close() error { return listener.listener.Close() }

var _ transport.ClosedSharedCarrierListener = (*sharedListener)(nil)
