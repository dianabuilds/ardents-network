package node

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/planfile"
)

// IdentityKey reads one bounded PKCS#8 PEM Ed25519 Node identity.
func IdentityKey(path string) (ed25519.PrivateKey, error) {
	raw, err := planfile.Read(path, 64<<10)
	if err != nil {
		return nil, err
	}
	block, rest := pem.Decode(raw)
	if block == nil || len(rest) != 0 {
		return nil, errors.New("node identity key is invalid")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	identity, ok := parsed.(ed25519.PrivateKey)
	if !ok {
		return nil, errors.New("node identity key is not Ed25519")
	}
	return identity, nil
}
