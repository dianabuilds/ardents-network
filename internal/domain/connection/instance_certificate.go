package connection

import (
	"crypto/ed25519"
	"crypto/subtle"
	"crypto/tls"
	"errors"
)

// verifyInstanceCertificate is the TLS VerifyConnection check for the exact
// independently verified Instance public key. TLS itself verifies possession
// through CertificateVerify. Public PKI, names and peer-supplied roots cannot
// replace the expected key. This check supplies no Publication authority or
// established stream receipt; protocol InstanceProof and Continuity, completed
// handshake and original live owner observations remain separately required.
// VerifyConnection runs before HandshakeComplete becomes true.
func verifyInstanceCertificate(state tls.ConnectionState, expected [32]byte) error {
	if expected == [32]byte{} || state.Version != tls.VersionTLS13 || state.DidResume ||
		len(state.PeerCertificates) != 1 || state.PeerCertificates[0] == nil {
		return errors.New("service Instance TLS certificate or profile is unavailable")
	}
	public, ok := state.PeerCertificates[0].PublicKey.(ed25519.PublicKey)
	if !ok || len(public) != ed25519.PublicKeySize || subtle.ConstantTimeCompare(public, expected[:]) != 1 {
		return errors.New("service Instance TLS key differs from verified Publication")
	}
	return nil
}
