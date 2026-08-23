package node

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"io"
	"os"
)

// IdentityKey reads one bounded PKCS#8 PEM Ed25519 Node identity.
func IdentityKey(path string) (ed25519.PrivateKey, error) {
	raw, err := readIdentityFile(path)
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

func readIdentityFile(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	raw, readErr := io.ReadAll(io.LimitReader(file, (64<<10)+1))
	closeErr := file.Close()
	if readErr != nil {
		return nil, readErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if len(raw) > 64<<10 {
		return nil, errors.New("node identity key exceeds its bound")
	}
	return raw, nil
}
