//go:build linux

package client

// Shared peer-identity fixtures for the retained Linux-only client-path
// tests. entryBindingCertificate and identifierFromKey were relocated from
// the retired generation-2 EntryBinding and LegBinding test files when
// ADR-0093 deleted those grammars; the Carrier-side copy of this fixture
// lives in internal/route/carrier beside the moved Carrier tests.

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"testing"
	"time"
)

func entryBindingCertificate(t *testing.T, serial int64) tls.Certificate {
	t.Helper()
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{byte(serial)}, ed25519.SeedSize))
	template := &x509.Certificate{SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: "route.test"},
		NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Minute), KeyUsage: x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth}}
	raw, err := x509.CreateCertificate(rand.Reader, template, template, private.Public(), private)
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := x509.ParseCertificate(raw)
	if err != nil {
		t.Fatal(err)
	}
	return tls.Certificate{Certificate: [][]byte{raw}, PrivateKey: private, Leaf: leaf}
}

func identifierFromKey(key ed25519.PublicKey) [32]byte {
	var result [32]byte
	copy(result[:], key)
	return result
}
