package tls

import (
	"context"
	"crypto/tls"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
	"net"
	"time"
)

// OpenRole creates one fresh Endpoint-to-role inner TLS channel over
// an already selected raw carrier. Endpoint presents no stable client
// certificate; only the State-selected server Ed25519 key authenticates it.
func OpenRole(ctx context.Context, raw net.Conn, expectedServer [32]byte, deadline time.Time) (*tls.Conn, error) {
	if ctx == nil || raw == nil || expectedServer == [32]byte{} || deadline.IsZero() || !time.Now().Before(deadline) {
		return nil, errors.New("closed role TLS request is invalid")
	}
	secured := tls.Client(raw, transport.RoleClientTLS(expectedServer))
	if err := secured.SetDeadline(deadline); err != nil {
		return nil, err
	}
	if err := secured.HandshakeContext(ctx); err != nil {
		return nil, err
	}
	if err := transport.ValidateRoleTLS(secured.ConnectionState(), expectedServer, false); err != nil {
		return nil, err
	}
	if err := secured.SetDeadline(time.Time{}); err != nil {
		return nil, err
	}
	return secured, nil
}
