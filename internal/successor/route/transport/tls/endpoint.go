package tls

import (
	"context"
	"errors"
	"net"

	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
)

// OpenEndpoint opens one selected literal TCP role endpoint and authenticates
// its inner TLS. Unlike OpenRole, this adapter owns the dial and closes its
// socket if setup refuses; a completed connection transfers to its caller.
func OpenEndpoint(ctx context.Context, input transport.ClosedRoleCarrierRequest) (net.Conn, error) {
	if err := transport.ValidateRoleRequest(ctx, input); err != nil {
		return nil, err
	}
	if input.CarrierProfile != transport.ClosedCarrierTCP {
		return nil, errors.New("closed role carrier profile is unsupported")
	}
	attempt, cancel := context.WithDeadline(ctx, input.Deadline)
	defer cancel()
	raw, err := (&net.Dialer{}).DialContext(attempt, "tcp", input.Endpoint)
	if err != nil {
		return nil, err
	}
	secured, err := OpenRole(attempt, raw, input.ExpectedServer, input.Deadline)
	if err != nil {
		_ = raw.Close()
		return nil, err
	}
	return secured, nil
}
