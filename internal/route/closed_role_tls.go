package route

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"time"
)

// OpenClosedRoleTLS creates one fresh Endpoint-to-role inner TLS channel over
// an already selected raw carrier. Endpoint presents no stable client
// certificate; only the State-selected server Ed25519 key authenticates it.
func OpenClosedRoleTLS(ctx context.Context, raw net.Conn, expectedServer [32]byte, deadline time.Time) (*tls.Conn, error) {
	if ctx == nil || raw == nil || expectedServer == [32]byte{} || deadline.IsZero() || !time.Now().Before(deadline) {
		return nil, errors.New("closed role TLS request is invalid")
	}
	secured := tls.Client(raw, closedRoleClientTLS(expectedServer))
	if err := secured.SetDeadline(deadline); err != nil {
		return nil, err
	}
	if err := secured.HandshakeContext(ctx); err != nil {
		return nil, err
	}
	if err := validClosedRoleTLSState(secured.ConnectionState(), expectedServer, false); err != nil {
		return nil, err
	}
	if err := secured.SetDeadline(time.Time{}); err != nil {
		return nil, err
	}
	return secured, nil
}

// AcceptClosedRoleTLS accepts one fresh inner Endpoint TLS channel. It rejects
// client certificates so a caller cannot turn this local privacy boundary into
// a stable Endpoint transport identity.
func AcceptClosedRoleTLS(ctx context.Context, raw net.Conn, certificate tls.Certificate, deadline time.Time) (*tls.Conn, error) {
	if ctx == nil || raw == nil || certificate.PrivateKey == nil || deadline.IsZero() || !time.Now().Before(deadline) {
		return nil, errors.New("closed role TLS acceptance is invalid")
	}
	secured := tls.Server(raw, closedRoleServerTLS(certificate))
	if err := secured.SetDeadline(deadline); err != nil {
		return nil, err
	}
	if err := secured.HandshakeContext(ctx); err != nil {
		return nil, err
	}
	if err := validClosedRoleTLSState(secured.ConnectionState(), [32]byte{}, true); err != nil {
		return nil, err
	}
	if err := secured.SetDeadline(time.Time{}); err != nil {
		return nil, err
	}
	return secured, nil
}

func closedRoleClientTLS(expected [32]byte) *tls.Config {
	return &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13, InsecureSkipVerify: true,
		SessionTicketsDisabled: true, ClientSessionCache: nil, NextProtos: []string{ClosedRouteProfile},
		CurvePreferences: []tls.CurveID{tls.X25519MLKEM768, tls.X25519}, VerifyConnection: exactPeer(expected)}
}

func closedRoleServerTLS(certificate tls.Certificate) *tls.Config {
	return &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13, Certificates: []tls.Certificate{certificate},
		ClientAuth: tls.NoClientCert, SessionTicketsDisabled: true, NextProtos: []string{ClosedRouteProfile},
		CurvePreferences: []tls.CurveID{tls.X25519MLKEM768, tls.X25519}, VerifyConnection: func(state tls.ConnectionState) error {
			if len(state.PeerCertificates) != 0 {
				return errors.New("closed role TLS client certificate is forbidden")
			}
			return nil
		}}
}

func validClosedRoleTLSState(state tls.ConnectionState, expected [32]byte, server bool) error {
	if state.Version != tls.VersionTLS13 || state.NegotiatedProtocol != ClosedRouteProfile {
		return errors.New("closed role TLS state is invalid")
	}
	if server {
		if len(state.PeerCertificates) != 0 {
			return errors.New("closed role TLS client certificate is forbidden")
		}
		return nil
	}
	if len(state.PeerCertificates) != 1 {
		return errors.New("closed role TLS server certificate is invalid")
	}
	return exactPeer(expected)(state)
}
