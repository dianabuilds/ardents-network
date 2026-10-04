//go:build linux

package carrier

import (
	"bytes"
	"crypto/ed25519"
	"crypto/tls"
	"errors"
	"time"
)

func exactPeer(expected [32]byte) func(tls.ConnectionState) error {
	return func(state tls.ConnectionState) error {
		if expected == [32]byte{} || len(state.PeerCertificates) != 1 {
			return errors.New("carrier peer identity is missing")
		}
		certificate := state.PeerCertificates[0]
		if time.Now().Before(certificate.NotBefore) || !time.Now().Before(certificate.NotAfter) {
			return errors.New("carrier peer certificate is not current")
		}
		public, ok := state.PeerCertificates[0].PublicKey.(ed25519.PublicKey)
		if !ok || len(public) != ed25519.PublicKeySize || !bytes.Equal(public, expected[:]) {
			return errors.New("carrier peer identity does not match authenticated state")
		}
		return nil
	}
}
