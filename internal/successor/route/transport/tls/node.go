package tls

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
)

// OpenNode opens the exact literal TCP/TLS Node peer within the original bound.
// It authenticates both Node certificates and returns physical socket-close
// ownership, rather than directional TLS shutdown. It supplies no role right.
func OpenNode(ctx context.Context, input transport.ClosedNodeCarrierRequest) (transport.Carrier, error) {
	if err := transport.ValidateNodeRequest(ctx, input); err != nil {
		return nil, err
	}
	if input.CarrierProfile != transport.ClosedCarrierTCP {
		return nil, errors.New("closed Node Carrier profile is unsupported")
	}
	attempt, cancel := context.WithDeadline(ctx, input.Deadline)
	defer cancel()
	raw, err := (&net.Dialer{}).DialContext(attempt, "tcp", input.Endpoint)
	if err != nil {
		return nil, err
	}
	socket := &nativeSocket{Conn: raw}
	secured := tls.Client(socket, transport.NodeClientTLS(input.Certificate, input.ExpectedPeerKey))
	if err := secured.SetDeadline(input.Deadline); err != nil {
		_ = raw.Close()
		return nil, err
	}
	if err := secured.HandshakeContext(attempt); err != nil {
		_ = raw.Close()
		return nil, err
	}
	if err := transport.ValidateNodeTLS(secured.ConnectionState(), input.ExpectedPeerKey); err != nil {
		_ = raw.Close()
		return nil, err
	}
	if err := secured.SetDeadline(time.Time{}); err != nil {
		_ = raw.Close()
		return nil, err
	}
	return &nodeCarrier{Conn: secured, socket: socket}, nil
}
