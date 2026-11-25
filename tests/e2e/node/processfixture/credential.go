package processfixture

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// WriteNativeCredential installs only the fixture leaf and its original test key.
func WriteNativeCredential(t testing.TB, certificate tls.Certificate) (string, string) {
	t.Helper()
	root := TempDir(t)
	certPath, keyPath := filepath.Join(root, "certificate"), filepath.Join(root, "key")
	if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificate.Certificate[0]}), 0600); err != nil {
		t.Fatal(err)
	}
	key, err := x509.MarshalPKCS8PrivateKey(certificate.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(key)
	encoded := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: key})
	defer clear(encoded)
	if err := os.WriteFile(keyPath, encoded, 0600); err != nil {
		t.Fatal(err)
	}
	return certPath, keyPath
}

// RoleCertificate creates a leaf for the original fixture Node key.
func RoleCertificate(t testing.TB, key ed25519.PrivateKey) tls.Certificate {
	t.Helper()
	template := &x509.Certificate{SerialNumber: big.NewInt(1), NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}}
	leaf, err := x509.CreateCertificate(rand.Reader, template, template, key.Public(), key)
	if err != nil {
		t.Fatal(err)
	}
	return tls.Certificate{Certificate: [][]byte{leaf}, PrivateKey: key}
}
