package route

import (
	"bytes"
	"crypto/ed25519"
	"crypto/tls"
	"errors"
)

func exactPeer(expected [32]byte) func(tls.ConnectionState) error {
	return func(state tls.ConnectionState) error {
		if expected == [32]byte{} || len(state.PeerCertificates) != 1 {
			return errors.New("carrier peer identity is missing")
		}
		public, ok := state.PeerCertificates[0].PublicKey.(ed25519.PublicKey)
		if !ok || len(public) != ed25519.PublicKeySize || !bytes.Equal(public, expected[:]) {
			return errors.New("carrier peer identity does not match authenticated state")
		}
		return nil
	}
}
