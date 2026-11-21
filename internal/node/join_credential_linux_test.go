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
	"syscall"
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

// Detached access-grammar control, never successful startup or file authority.
func TestJoinCredentialPinPreservesProtectedDeliveryAndRefusesForeignAccess(t *testing.T) {
	good := credentialMetadata{mode: 0640, native: syscall.Stat_t{Uid: 0, Gid: 988, Nlink: 1}}
	if !joinPrivateCredentialAccess(good, 988, 988) {
		t.Fatal("JOIN pin rejects the reader's exact protected delivery")
	}
	for _, principal := range [][2]uint32{{0, 988}, {988, 989}, {989, 989}, {99, 99}, {1000, 1000}} {
		if joinPrivateCredentialAccess(good, principal[0], principal[1]) {
			t.Fatal("foreign principal admitted", principal)
		}
	}
	for _, mode := range []os.FileMode{0644, 0660, 0440, 0640 | os.ModeSetuid, 0640 | os.ModeSymlink} {
		other := good
		other.mode = mode
		if joinPrivateCredentialAccess(other, 988, 988) {
			t.Fatal("weaker delivery admitted", mode)
		}
	}
	for _, change := range []func(*syscall.Stat_t){func(s *syscall.Stat_t) { s.Gid = 989 }, func(s *syscall.Stat_t) { s.Nlink = 2 }} {
		other := good
		change(&other.native)
		if joinPrivateCredentialAccess(other, 988, 988) {
			t.Fatal("foreign group or alias admitted")
		}
	}
	owned := credentialMetadata{mode: 0600, native: syscall.Stat_t{Uid: 988, Gid: 988, Nlink: 1}}
	if !joinPrivateCredentialAccess(owned, 988, 988) {
		t.Fatal("existing runtime-owned policy lost")
	}
}
