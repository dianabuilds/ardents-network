// Package principal owns principal derivation and canonical identity encoding.
// It does not own credential storage or authorization policy.
package principal

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
)

func FromPublicKey(encoded string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", errors.New("record public key is invalid")
	}
	if len(raw) != ed25519.PublicKeySize {
		return "", errors.New("record public key length is invalid")
	}
	return DeriveID("p", raw), nil
}

func DeriveID(prefix string, raw []byte) string {
	sum := sha256.Sum256(raw)
	return prefix + "_" + hex.EncodeToString(sum[:8])
}
