package reachability

import (
	"crypto/ed25519"
	"crypto/tls"
	"errors"
	"net/http"
)

// GatewayHTTPClient creates the one-purpose HTTPS client used by a
// State-authorized opaque forwarder. Gateway certificate identity is pinned to
// the selected Ed25519 Node public key; ambient roots, proxies, compression,
// keep-alives, and HTTP/2 are not used.
func GatewayHTTPClient(expected [32]byte) (*http.Client, error) {
	if expected == [32]byte{} {
		return nil, errors.New("private reachability Gateway key is missing")
	}
	transport := &http.Transport{Proxy: nil, DisableCompression: true, DisableKeepAlives: true, ForceAttemptHTTP2: false,
		MaxConnsPerHost: 1, TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13,
			InsecureSkipVerify: true, SessionTicketsDisabled: true, NextProtos: []string{"http/1.1"}, VerifyConnection: func(state tls.ConnectionState) error {
				if len(state.PeerCertificates) != 1 {
					return errors.New("private reachability Gateway certificate is unavailable")
				}
				public, ok := state.PeerCertificates[0].PublicKey.(ed25519.PublicKey)
				if !ok || len(public) != len(expected) || string(public) != string(expected[:]) {
					return errors.New("private reachability Gateway certificate does not match State")
				}
				return nil
			}}}
	return &http.Client{Transport: transport}, nil
}
