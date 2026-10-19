//go:build linux

package carrier

import (
	"context"
	"errors"
	"net"
	"time"
)

// OpenClosedRoleCarrier opens exactly one State-selected TCP/TLS role channel.
// Deferred QUIC profiles refuse without another transport attempt.
func OpenClosedRoleCarrier(ctx context.Context, input ClosedRoleCarrierRequest) (net.Conn, error) {
	if ctx == nil || !LiteralEndpoint(input.Endpoint) || input.ExpectedServer == [32]byte{} || input.Deadline.IsZero() || !time.Now().Before(input.Deadline) {
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

	default:
		return nil, errors.New("closed role carrier profile is unsupported")
	}
}
