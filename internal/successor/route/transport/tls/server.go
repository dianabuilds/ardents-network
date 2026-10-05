package tls

import (
	"context"
	"crypto/tls"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
	"net"
	"time"
)

// AcceptRole accepts one fresh inner Endpoint TLS channel. It rejects
// client certificates so a caller cannot turn this local privacy boundary into
// a stable Endpoint transport identity.
func AcceptRole(ctx context.Context, raw net.Conn, certificate tls.Certificate, deadline time.Time) (*tls.Conn, error) {
	if ctx == nil || raw == nil || certificate.PrivateKey == nil || deadline.IsZero() || !time.Now().Before(deadline) {
		return nil, errors.New("closed role TLS acceptance is invalid")
	}
	secured := tls.Server(raw, transport.RoleServerTLS(certificate))
	if err := secured.SetDeadline(deadline); err != nil {
		return nil, err
	}
	if err := secured.HandshakeContext(ctx); err != nil {
		return nil, err
	}
	if err := transport.ValidateRoleTLS(secured.ConnectionState(), [32]byte{}, true); err != nil {
		return nil, err
	}
	if err := secured.SetDeadline(time.Time{}); err != nil {
		return nil, err
	}
	return secured, nil
}
