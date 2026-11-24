package node

import (
	"crypto/ed25519"
	"crypto/tls"
	"errors"
)

// credential retains the exact runtime TLS material through Route join. It
// grants no provisioning, offline signing or membership and never hot-reloads.
type credential struct {
	certificate tls.Certificate
	key         ed25519.PrivateKey
}

var errInvalidRuntimeKey = errors.New("invalid Node runtime TLS key")

func decodeCredential(cert, key []byte) (*credential, error) {
	pair, err := tls.X509KeyPair(cert, key)
	if err != nil {
		return nil, err
	}
	private, ok := pair.PrivateKey.(ed25519.PrivateKey)
	if !ok || len(private) != ed25519.PrivateKeySize {
		return nil, errInvalidRuntimeKey
	}
	return &credential{certificate: pair, key: private}, nil
}

func (key *credential) close() error {
	if key != nil {
		clear(key.key)
		key.certificate.PrivateKey = nil
	}
	return nil
}
