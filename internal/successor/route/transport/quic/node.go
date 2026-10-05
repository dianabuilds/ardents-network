package quic

import (
	"context"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
	"github.com/quic-go/quic-go"
	"time"
)

func OpenNode(ctx context.Context, input transport.ClosedNodeCarrierRequest) (transport.Carrier, error) {
	if err := transport.ValidateNodeRequest(ctx, input); err != nil {
		return nil, err
	}
	if input.CarrierProfile != transport.ClosedCarrierQUIC {
		return nil, errors.New("closed Node Carrier profile is unsupported")
	}
	attempt, cancel := context.WithDeadline(ctx, input.Deadline)
	defer cancel()
	connection, err := quic.DialAddr(attempt, input.Endpoint, transport.NodeClientTLS(input.Certificate, input.ExpectedPeerKey), nodeConfig())
	if err != nil {
		return nil, err
	}
	if err := transport.ValidateNodeTLS(connection.ConnectionState().TLS, input.ExpectedPeerKey); err != nil {
		_ = connection.CloseWithError(1, "carrier-peer-invalid")
		return nil, err
	}
	stream, err := connection.OpenStreamSync(attempt)
	if err != nil {
		_ = connection.CloseWithError(1, "carrier-open-failed")
		return nil, err
	}
	lane := &nodeCarrier{stream: stream, connection: connection}
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
