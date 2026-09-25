package durableroot_test

import (
	"path/filepath"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/endpoint/durableroot"
)

func TestPrivateRootLeaseExcludesAnotherOwnerAndCanReopen(t *testing.T) {
	root := t.TempDir()
	if err := durableroot.Secure(root); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "owner.lock")
	first, err := durableroot.Acquire(path)
	if err != nil {
		t.Fatal(err)
	}
	if second, err := durableroot.Acquire(path); err == nil {
		_ = second.Release()
		t.Fatal("second owner acquired the same root")
	}
	if err := first.Release(); err != nil {
		t.Fatal(err)
	}
	reopened, err := durableroot.Acquire(path)
	if err != nil {
		t.Fatalf("released root could not reopen: %v", err)
	}
	if err := durableroot.SyncDirectory(root); err != nil {
		t.Fatal(err)
	}
	if err := reopened.Release(); err != nil {
		t.Fatal(err)
	}
}
