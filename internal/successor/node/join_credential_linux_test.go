//go:build linux

package node

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func joinCredentialFiles(t *testing.T) (string, string) {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(private)
	template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "local credential custody fixture"}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature}
	der, err := x509.CreateCertificate(rand.Reader, template, template, public, private)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := x509.MarshalPKCS8PrivateKey(private)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(encoded)
	cert, key := filepath.Join(t.TempDir(), "certificate"), filepath.Join(t.TempDir(), "key")
	if err := os.WriteFile(cert, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(key, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: encoded}), 0600); err != nil {
		t.Fatal(err)
	}
	return cert, key
}

func TestJoinCredentialRetainsOriginalCustodyAndFailure(t *testing.T) {
	for _, fault := range []string{"replace", "bytes", "mode", "descriptor"} {
		t.Run(fault, func(t *testing.T) {
			cert, key := joinCredentialFiles(t)
			original, err := loadJoinCredential(cert, key)
			if err != nil {
				t.Fatal(err)
			}
			defer original.close()
			if err := original.check(); err != nil {
				t.Fatal(err)
			}
			raw, err := os.ReadFile(key)
			if err != nil {
				t.Fatal(err)
			}
			defer clear(raw)
			switch fault {
			case "replace":
				if err := os.Rename(key, key+".original"); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(key, raw, 0600); err != nil {
					t.Fatal(err)
				}
			case "bytes":
				changed := append([]byte(nil), raw...)
				changed[len(changed)-1] ^= 1
				if err := os.WriteFile(key, changed, 0600); err != nil {
					t.Fatal(err)
				}
				clear(changed)
			case "mode":
				if err := os.Chmod(key, 0644); err != nil {
					t.Fatal(err)
				}
			case "descriptor":
				if err := original.files[1].Close(); err != nil {
					t.Fatal(err)
				}
			}
			first := original.check()
			if first == nil {
				t.Fatal("lost original custody admitted key")
			}
			if err := os.WriteFile(key, raw, 0600); err != nil {
				t.Fatal(err)
			}
			if original.check() != first {
				t.Fatal("restored files revived original key custody")
			}
			private := original.material.key
			closed := original.close()
			if fault == "descriptor" && !errors.Is(closed, os.ErrClosed) {
				t.Fatal("actual close failure disappeared", closed)
			}
			if original.close() != closed {
				t.Fatal("repeated Close replaced original physical result")
			}
			for _, b := range private {
				if b != 0 {
					t.Fatal("joined key not erased")
				}
			}
		})
	}
}
