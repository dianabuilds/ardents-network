package identity

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
)

func PrincipalFromPublicKey(encoded string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", errors.New("record public key is invalid")
	}
	if len(raw) != ed25519.PublicKeySize {
		return "", errors.New("record public key length is invalid")
	}
	return deriveID("p", raw), nil
}

func deriveID(prefix string, raw []byte) string {
	sum := sha256.Sum256(raw)
	return prefix + "_" + hex.EncodeToString(sum[:8])
}
