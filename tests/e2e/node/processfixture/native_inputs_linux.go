//go:build linux && node_process_native

package processfixture

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
)

// WriteNativeCredential installs only the fixture leaf and its original test key.
func WriteNativeCredential(t testing.TB, certificate tls.Certificate) (string, string) {
	t.Helper()
	root := t.TempDir()
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
