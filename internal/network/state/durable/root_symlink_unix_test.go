//go:build !windows

package durable

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRootRecoveryDoesNotFollowSymlinkedGenerations(t *testing.T) {
	path := t.TempDir()
	root, err := Open(path, testLimits())
	if err != nil {
		t.Fatal(err)
	}
	if err := root.Close(); err != nil {
		t.Fatal(err)
	}

	generations := filepath.Join(path, "generations")
	if err := os.Remove(generations); err != nil {
		t.Fatal(err)
	}
	external := t.TempDir()
	foreignStaging := filepath.Join(external, ".stage-unowned")
	if err := os.Mkdir(foreignStaging, 0o700); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(foreignStaging, "keep")
	if err := os.WriteFile(sentinel, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, generations); err != nil {
		t.Fatal(err)
	}

	if reopened, err := Open(path, testLimits()); err == nil {
		_ = reopened.Close()
		t.Fatal("followed an unowned generations directory")
	}
	if raw, err := os.ReadFile(sentinel); err != nil || string(raw) != "keep" {
		t.Fatalf("foreign staging was removed: %q, %v", raw, err)
	}
}
