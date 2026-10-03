//go:build linux

package issuer

import (
	"bytes"
	"crypto/ed25519"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestOpenClosedIssuerRootJoinsReleaseFailure(t *testing.T) {
	path := closedIssuerFixtureRoot(t)
	retained := []byte("unowned evidence")
	if err := os.WriteFile(filepath.Join(path, "foreign"), retained, 0600); err != nil {
		t.Fatal(err)
	}
	root, lease, err := openClosedIssuerRootWithLease(path, func(path string) (issuerRootLease, error) {
		lease, err := acquireIssuerRootLease(path)
		if err != nil {
			return lease, err
		}
		if err := lease.file.Close(); err != nil {
			t.Fatal(err)
		}
		return lease, nil
	})
	if root != "" || lease.file != nil || err == nil || !strings.Contains(err.Error(), "non-fresh") || !errors.Is(err, syscall.EBADF) {
		t.Fatalf("open = %q, %v; want primary refusal and physical release failure", root, err)
	}
	got, err := os.ReadFile(filepath.Join(path, "foreign"))
	if err != nil || !bytes.Equal(got, retained) {
		t.Fatalf("retained evidence changed: %q, %v", got, err)
	}
}

func TestInitializeClosedIssuerRootReportsReleaseFailure(t *testing.T) {
	now := time.Unix(1800000000, 0).UTC()
	config := ClosedIssuerRootConfig{Root: closedIssuerFixtureRoot(t), NetworkID: credentialID(1), NodeID: credentialID(2),
		IdentityKey: ed25519.NewKeyFromSeed(bytes.Repeat([]byte{19}, 32)), NotBefore: now, NotAfter: now.Add(time.Hour), Clock: func() time.Time { return now.Add(time.Minute) }}
	first, err := InitializeClosedIssuerRoot(config)
	if err != nil {
		t.Fatal(err)
	}
	material, err := os.ReadFile(filepath.Join(config.Root, closedIssuerMaterialName))
	if err != nil {
		t.Fatal(err)
	}
	for _, refuse := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "refusal"}[refuse], func(t *testing.T) {
			selected := config
			if refuse {
				selected.NetworkID = credentialID(99)
			}
			receipt, err := initializeClosedIssuerRoot(selected, func(path string) (string, issuerRootLease, error) {
				root, lease, err := openClosedIssuerRoot(path)
				if err != nil {
					return root, lease, err
				}
				if err := lease.file.Close(); err != nil {
					t.Fatal(err)
				}
				return root, lease, nil
			})
			if !errors.Is(err, syscall.EBADF) {
				t.Fatalf("initialize = %d bytes, %v; physical release failure missing", len(receipt.Profile), err)
			}
			if refuse && !strings.Contains(err.Error(), "cannot be replaced") {
				t.Fatalf("primary refusal missing: %v", err)
			}
			if len(receipt.Profile) != 0 || receipt.ProfileDigest != [32]byte{} {
				t.Fatal("failed initialization returned success receipt")
			}
		})
	}
	got, err := os.ReadFile(filepath.Join(config.Root, closedIssuerMaterialName))
	if err != nil || !bytes.Equal(got, material) {
		t.Fatal("immutable material changed")
	}
	reopened, err := InitializeClosedIssuerRoot(config)
	if err != nil || !bytes.Equal(reopened.Profile, first.Profile) {
		t.Fatalf("exact reopen = %v", err)
	}
}
