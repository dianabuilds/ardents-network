package state_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func makeAuthority(t *testing.T, name string) processCert {
	t.Helper()
	public, private, _ := ed25519.GenerateKey(rand.Reader)
	template := &x509.Certificate{SerialNumber: big.NewInt(time.Now().UnixNano()), Subject: pkix.Name{CommonName: name},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), IsCA: true,
		BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature}
	raw, err := x509.CreateCertificate(rand.Reader, template, template, public, private)
	if err != nil {
		t.Fatal(err)
	}
	return processCert{root: writePEM(t, name+".pem", "CERTIFICATE", raw), private: private}
}

func makeLeaf(t *testing.T, authority processCert, name string, server bool) processCert {
	t.Helper()
	public, private, _ := ed25519.GenerateKey(rand.Reader)
	usage := x509.ExtKeyUsageClientAuth
	if server {
		usage = x509.ExtKeyUsageServerAuth
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(time.Now().UnixNano()), Subject: pkix.Name{CommonName: name},
		DNSNames: []string{name}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{usage}}
	rootRaw, _ := os.ReadFile(authority.root)
	block, _ := pem.Decode(rootRaw)
	parent, _ := x509.ParseCertificate(block.Bytes)
	raw, err := x509.CreateCertificate(rand.Reader, template, parent, public, authority.private)
	if err != nil {
		t.Fatal(err)
	}
	keyRaw, _ := x509.MarshalPKCS8PrivateKey(private)
	pinRaw, _ := x509.MarshalPKIXPublicKey(public)
	sourceRaw := append([]byte("ardents-h3-source-transport-key-v1\x00"), public...)
	certificatePath := filepath.Join(t.TempDir(), name+"-cert.pem")
	chain := append(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: raw}), rootRaw...)
	if err := os.WriteFile(certificatePath, chain, 0o600); err != nil {
		t.Fatal(err)
	}
	return processCert{certificate: certificatePath,
		key: writePEM(t, name+"-key.pem", "PRIVATE KEY", keyRaw), root: authority.root,
		pin: sha256.Sum256(pinRaw), sourcePin: sha256.Sum256(sourceRaw)}
}

func loadCertificate(t *testing.T, value processCert) tls.Certificate {
	t.Helper()
	certificate, err := tls.LoadX509KeyPair(value.certificate, value.key)
	if err != nil {
		t.Fatal(err)
	}
	return certificate
}

func writePrivateKey(t *testing.T, name string, private ed25519.PrivateKey) string {
	t.Helper()
	raw, _ := x509.MarshalPKCS8PrivateKey(private)
	return writePEM(t, name, "PRIVATE KEY", raw)
}

func writePEM(t *testing.T, name, kind string, raw []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: kind, Bytes: raw}), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
