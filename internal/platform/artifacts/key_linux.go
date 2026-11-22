package artifacts

import (
	"bytes"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
)

// ParseEd25519Key decodes an exact canonical PKCS#8 Ed25519 private key.
// The caller owns the key bytes and must independently verify its purpose.
func ParseEd25519Key(raw []byte) (ed25519.PrivateKey, error) {
	block, rest := pem.Decode(raw)
	if block == nil {
		return nil, ErrInput
	}
	defer clear(block.Bytes)
	canonical := pem.EncodeToMemory(block)
	defer clear(canonical)
	if len(rest) != 0 || block.Type != "PRIVATE KEY" || len(block.Headers) != 0 || !bytes.Equal(raw, canonical) {
		return nil, ErrInput
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	key, ok := parsed.(ed25519.PrivateKey)
	if err != nil || !ok || len(key) != ed25519.PrivateKeySize {
		clear(key)
		return nil, ErrInput
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	defer clear(der)
	if err != nil || !bytes.Equal(der, block.Bytes) {
		clear(key)
		return nil, ErrInput
	}
	derived := ed25519.NewKeyFromSeed(key[:ed25519.SeedSize])
	defer clear(derived)
	if !bytes.Equal(derived, key) {
		clear(key)
		return nil, ErrInput
	}
	return key, nil
}
