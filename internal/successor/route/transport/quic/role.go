package quic

import (
	"context"
	"errors"
	"net"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
	"github.com/quic-go/quic-go"
)

// OpenEndpoint opens one selected role QUIC stream. Its TLS handshake is the
// role authentication itself; this stream is not wrapped in another TLS layer.
func OpenEndpoint(ctx context.Context, input transport.ClosedRoleCarrierRequest) (net.Conn, error) {
	if err := transport.ValidateRoleRequest(ctx, input); err != nil {
		return nil, err
	}
	if input.CarrierProfile != transport.ClosedCarrierQUIC {
		return nil, errors.New("closed role carrier profile is unsupported")
	}
	attempt, cancel := context.WithDeadline(ctx, input.Deadline)
	defer cancel()
	connection, err := quic.DialAddr(attempt, input.Endpoint, transport.RoleClientTLS(input.ExpectedServer), roleConfig())
	if err != nil {
		return nil, err
	}
	if err := transport.ValidateRoleTLS(connection.ConnectionState().TLS, input.ExpectedServer, false); err != nil {
		_ = connection.CloseWithError(1, "role-peer-invalid")
		return nil, err
	}
	stream, err := connection.OpenStreamSync(attempt)
	if err != nil {
		_ = connection.CloseWithError(1, "role-open-failed")
		return nil, err
	}
	carrier := &roleCarrier{stream: stream, connection: connection}
	if err := carrier.SetDeadline(input.Deadline); err != nil {
		_ = carrier.Close()
		return nil, err
	}
	if err := carrier.SetDeadline(time.Time{}); err != nil {
		_ = carrier.Close()
		return nil, err
	}
	return carrier, nil
}
