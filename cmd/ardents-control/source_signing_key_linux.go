package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
)

// This codec admits no authority or signing purpose. Each fixed operation
// separately retains original protected files, public pins, purpose and time.
func parseSourceSigningKey(raw []byte) (ed25519.PrivateKey, error) {
	block, rest := pem.Decode(raw)
	if block == nil {
		return nil, errSourceControlInput
	}
	defer clear(block.Bytes)
	canonical := pem.EncodeToMemory(block)
	defer clear(canonical)
	if len(rest) != 0 || block.Type != "PRIVATE KEY" || len(block.Headers) != 0 || !bytes.Equal(raw, canonical) {
		return nil, errSourceControlInput
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	key, ok := parsed.(ed25519.PrivateKey)
	if err != nil || !ok || len(key) != ed25519.PrivateKeySize {
		clear(key)
		return nil, errSourceControlInput
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	defer clear(der)
	if err != nil || !bytes.Equal(der, block.Bytes) {
		clear(key)
		return nil, errSourceControlInput
	}
	derived := ed25519.NewKeyFromSeed(key[:ed25519.SeedSize])
	defer clear(derived)
	if !bytes.Equal(derived, key) {
		clear(key)
		return nil, errSourceControlInput
	}
	return key, nil
}
