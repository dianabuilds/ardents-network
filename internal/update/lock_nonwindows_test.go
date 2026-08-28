//go:build !windows

package update

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOwnedLockAcceptsDirectRegularFile(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, lockFileName)
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	identity, identityErr := fstatHandle(file)
	closeErr := file.Close()
	if identityErr != nil || closeErr != nil {
		t.Fatalf("inspect lock identity: %v / %v", identityErr, closeErr)
	}
	if err := validateLockIdentity(identity); err != nil {
		t.Fatalf("direct regular lock identity = %+v: %v", identity, err)
	}
	lock, err := acquireOwnedLock(root)
	if err != nil {
		t.Fatalf("acquire direct regular lock: %v", err)
	}
	if err := lock.release(); err != nil {
		t.Fatalf("release direct regular lock: %v", err)
	}
}
