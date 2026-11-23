package tls

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route/transport"
)

// OpenRole creates one fresh Endpoint-to-role inner TLS channel over
// an already selected raw carrier. Endpoint presents no stable client
// certificate; only the State-selected server Ed25519 key authenticates it.
// The caller retains the raw carrier's physical close ownership on both
// success and failure; the returned TLS connection wraps that same carrier.
func OpenRole(ctx context.Context, raw net.Conn, expectedServer [32]byte, deadline time.Time) (*tls.Conn, error) {
	if ctx == nil || raw == nil || expectedServer == [32]byte{} || deadline.IsZero() || !time.Now().Before(deadline) {
		return nil, errors.New("closed role TLS request is invalid")
	}
	secured := tls.Client(newRoleInput(raw), transport.RoleClientTLS(expectedServer))
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
