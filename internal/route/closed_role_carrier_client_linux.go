//go:build linux

package route

import (
	"context"
	"errors"
	"github.com/quic-go/quic-go"
	"net"
	"time"
)

// OpenClosedRoleCarrier opens exactly one State-selected TCP/TLS or QUIC role
// channel. QUIC's TLS handshake is the role TLS; it is not wrapped in a
// second TLS stream.
func OpenClosedRoleCarrier(ctx context.Context, input ClosedRoleCarrierRequest) (net.Conn, error) {
	if ctx == nil || !literalEndpoint(input.Endpoint) || input.ExpectedServer == [32]byte{} || input.Deadline.IsZero() || !time.Now().Before(input.Deadline) {
		return nil, errors.New("closed role carrier request is invalid")
	}
	attempt, cancel := context.WithDeadline(ctx, input.Deadline)
	defer cancel()
	switch input.CarrierProfile {
	case ClosedCarrierTCP:
		raw, err := (&net.Dialer{}).DialContext(attempt, "tcp", input.Endpoint)
		if err != nil {
			return nil, err
		}
		secured, err := OpenClosedRoleTLS(attempt, raw, input.ExpectedServer, input.Deadline)
		if err != nil {
			_ = raw.Close()
			return nil, err
		}
		return secured, nil
	case ClosedCarrierQUIC:
		connection, err := quic.DialAddr(attempt, input.Endpoint, closedRoleClientTLS(input.ExpectedServer), closedRoleQUICConfig())
		if err != nil {
			return nil, err
		}
		if err := validClosedRoleTLSState(connection.ConnectionState().TLS, input.ExpectedServer, false); err != nil {
			_ = connection.CloseWithError(1, "role-peer-invalid")
			return nil, err
		}
		stream, err := connection.OpenStreamSync(attempt)
		if err != nil {
			_ = connection.CloseWithError(1, "role-open-failed")
			return nil, err
		}
		carrier := &closedRoleQUICCarrier{stream: stream, connection: connection}
		if err := carrier.SetDeadline(input.Deadline); err != nil {
			_ = carrier.Close()
			return nil, err
		}
		if err := carrier.SetDeadline(time.Time{}); err != nil {
			_ = carrier.Close()
			return nil, err
		}
		return carrier, nil
	default:
		return nil, errors.New("closed role carrier profile is unsupported")
	}
}
