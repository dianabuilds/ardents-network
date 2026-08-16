//go:build linux

package blockedverify

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReplayRegistryRejectsForeignOwner(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("foreign-owner construction requires root")
	}
	root := filepath.Join(t.TempDir(), "registry")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chown(root, 1, 1); err != nil {
		t.Fatal(err)
	}
	if err := protectRegistryTree(root); err == nil {
		t.Fatal("foreign-owned registry was accepted")
	}
}

func TestReplayRegistryTightensPermissiveOwnedRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "registry")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := protectRegistryTree(root); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(root)
	if err != nil || info.Mode().Perm() != 0o700 {
		t.Fatalf("registry mode=%o err=%v", info.Mode().Perm(), err)
	}
}
