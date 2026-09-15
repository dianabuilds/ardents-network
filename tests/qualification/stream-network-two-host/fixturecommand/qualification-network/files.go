//go:build linux

package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

func derivedKey(seed []byte, label string) ed25519.PrivateKey {
	input := append([]byte("ardents-issue60-fixture-v1"), 0)
	input = append(input, seed...)
	input = append(input, 0)
	input = append(input, label...)
	digest := sha256.Sum256(input)
	return ed25519.NewKeyFromSeed(digest[:])
}

func writePrivateKey(root, relative string, private ed25519.PrivateKey) error {
	body, err := x509.MarshalPKCS8PrivateKey(private)
	if err != nil {
		return err
	}
	return writePrivateKeyBytes(root, relative, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: body}))
}

func writeNodeCertificate(root, relative, name string, serial int64, private ed25519.PrivateKey, from, until time.Time) error {
	template := &x509.Certificate{
		SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: name}, DNSNames: []string{name},
		NotBefore: from, NotAfter: until, KeyUsage: x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	raw, err := x509.CreateCertificate(rand.Reader, template, template, private.Public(), private)
	if err != nil {
		return err
	}
	return writePrivateKeyBytes(root, relative, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: raw}))
}

func writeCertificateAuthority(root, relative, name string, serial int64, private ed25519.PrivateKey, from, until time.Time) ([]byte, error) {
	template := &x509.Certificate{SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: name},
		NotBefore: from, NotAfter: until, IsCA: true, BasicConstraintsValid: true,
		KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature}
	raw, err := x509.CreateCertificate(rand.Reader, template, template, private.Public(), private)
	if err != nil {
		return nil, err
	}
	pemBody := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: raw})
	if err := writePrivateKeyBytes(root, relative, pemBody); err != nil {
		return nil, err
	}
	return raw, nil
}

func writeSignedCertificate(root, relative, name string, serial int64, private, authority ed25519.PrivateKey,
	authorityRaw []byte, usage x509.ExtKeyUsage, from, until time.Time) ([32]byte, error) {
	parent, err := x509.ParseCertificate(authorityRaw)
	if err != nil {
		return [32]byte{}, err
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: name},
		DNSNames: []string{name}, NotBefore: from, NotAfter: until,
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{usage}}
	raw, err := x509.CreateCertificate(rand.Reader, template, parent, private.Public(), authority)
	if err != nil {
		return [32]byte{}, err
	}
	rootPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: authorityRaw})
	chain := append(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: raw}), rootPEM...)
	if err := writePrivateKeyBytes(root, relative, chain); err != nil {
		return [32]byte{}, err
	}
	public := private.Public().(ed25519.PublicKey)
	digestInput := append([]byte("ardents-h3-source-transport-key-v1"), 0)
	digestInput = append(digestInput, public...)
	return sha256.Sum256(digestInput), nil
}
func writePrivateKeyBytes(root, relative string, body []byte) error {
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := file.Write(body); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func writeJSON(root, relative string, value any) error {
	body, err := json.Marshal(value)
	if err != nil {
		return err
	}
	body = append(body, byte(10))
	return writePrivateKeyBytes(root, relative, body)
}

func hex32(value [32]byte) string { return hex.EncodeToString(value[:]) }

func decodeSeed(value string) ([]byte, error) {
	seed, err := hex.DecodeString(value)
	if err != nil || len(seed) != 32 {
		return nil, errors.New("fixture seed is invalid")
	}
	return seed, nil
}
