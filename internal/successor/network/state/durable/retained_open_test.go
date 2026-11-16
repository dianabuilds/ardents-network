package durable

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestRetainedRootRequiresOriginalStructureAndSecurityJournal(t *testing.T) {
	for _, missing := range []string{rootMarkerName, rootLockName, filepath.Join("distribution", "current")} {
		t.Run(missing, func(t *testing.T) {
			path := t.TempDir()
			owner, err := Open(path, testLimits())
			if err != nil {
				t.Fatal(err)
			}
			control := []byte("independent opaque security bytes")
			name := fmt.Sprintf("%x", sha256.Sum256(control))
			if err = owner.CommitControl(name, control); err != nil {
				t.Fatal(err)
			}
			if err = owner.Close(); err != nil {
				t.Fatal(err)
			}
			if err = os.Remove(filepath.Join(path, missing)); err != nil {
				t.Fatal(err)
			}
			if root, err := OpenRetained(path, testLimits()); err == nil {
				_ = root.Close()
				t.Fatal("missing original history was initialized")
			}
			if _, err := os.Lstat(filepath.Join(path, missing)); !os.IsNotExist(err) {
				t.Fatal("retained refusal recreated missing entry", err)
			}
			actual, err := os.ReadFile(filepath.Join(path, "distribution", "generations", name, "state.bin"))
			if err != nil || !bytes.Equal(actual, control) {
				t.Fatal("refusal changed retained security bytes", err)
			}
		})
	}
}

func TestRetainedLeaseNeverCreatesMissingLock(t *testing.T) {
	root := t.TempDir()
	if lease, err := acquireRetainedRootLease(root); err == nil {
		_ = lease.release()
		t.Fatal("missing retained lock was accepted")
	}
	if _, err := os.Lstat(filepath.Join(root, rootLockName)); !os.IsNotExist(err) {
		t.Fatalf("retained lease created a missing lock: %v", err)
	}
}

func TestRetainedRootDoesNotReinitializeErasedHistory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state")
	owner, err := Open(path, testLimits())
	if err != nil {
		t.Fatal(err)
	}
	control := []byte("previously accepted state")
	name := fmt.Sprintf("%x", sha256.Sum256(control))
	if err := owner.CommitControl(name, control); err != nil {
		t.Fatal(err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	original, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if err := os.RemoveAll(filepath.Join(path, entry.Name())); err != nil {
			t.Fatal(err)
		}
	}
	if reopened, err := OpenRetained(path, testLimits()); err == nil {
		_ = reopened.Close()
		t.Fatal("erased retained history was accepted")
	}
	after, err := os.Stat(path)
	if err != nil || !os.SameFile(original, after) {
		t.Fatal("refusal changed the original directory", err)
	}
	entries, err = os.ReadDir(path)
	if err != nil || len(entries) != 0 {
		t.Fatal("refusal recreated erased history", entries, err)
	}
}
