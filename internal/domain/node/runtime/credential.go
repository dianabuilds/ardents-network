package runtime

import (
	"crypto/ed25519"
	"crypto/tls"
	"errors"
)

// credentialMaterial retains the exact runtime TLS material through Route join. It
// grants no provisioning, offline signing or membership and never hot-reloads.
type credentialMaterial struct {
	certificate tls.Certificate
	key         ed25519.PrivateKey
}

var errInvalidRuntimeKey = errors.New("invalid Node runtime TLS key")

func decodeCredential(cert, key []byte) (*credentialMaterial, error) {
	pair, err := tls.X509KeyPair(cert, key)
	if err != nil {
		return nil, err
	}
	private, ok := pair.PrivateKey.(ed25519.PrivateKey)
	if !ok || len(private) != ed25519.PrivateKeySize {
		return nil, errInvalidRuntimeKey
	}
	return &credentialMaterial{certificate: pair, key: private}, nil
}

// Close erases the owned key after the caller joins its TLS borrowers.
func (key *credentialMaterial) Close() error {
	if key != nil {
		clear(key.key)
		key.certificate.PrivateKey = nil
	}
	return nil
}
