package transport

import (
	"crypto/tls"
	"errors"
)

// RoleClientTLS supplies the same exact-key, profile and version requirements
// to TLS streams and QUIC handshakes. A role client presents no certificate.
func RoleClientTLS(expected [32]byte) *tls.Config {
	return &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13, InsecureSkipVerify: true,
		SessionTicketsDisabled: true, ClientSessionCache: nil, NextProtos: []string{ClosedRouteProfile},
		CurvePreferences: []tls.CurveID{tls.X25519MLKEM768, tls.X25519}, VerifyConnection: ExactPeer(expected)}
}

// RoleServerTLS accepts fresh inner role TLS without a stable client transport
// identity. Shared direct/Node listeners retain their distinct classification.
func RoleServerTLS(certificate tls.Certificate) *tls.Config {
	return &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13, Certificates: []tls.Certificate{certificate},
		ClientAuth: tls.NoClientCert, SessionTicketsDisabled: true, NextProtos: []string{ClosedRouteProfile},
		CurvePreferences: []tls.CurveID{tls.X25519MLKEM768, tls.X25519}, VerifyConnection: func(state tls.ConnectionState) error {
			if len(state.PeerCertificates) != 0 {
				return errors.New("closed role TLS client certificate is forbidden")
			}
			return nil
		}}
}

// ValidateRoleTLS checks the negotiated state after either physical adapter's
// handshake. The server and client have deliberately different identities.
func ValidateRoleTLS(state tls.ConnectionState, expected [32]byte, server bool) error {
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
	return ExactPeer(expected)(state)
}
