//go:build linux

package attempts

import (
	"os"
	"path/filepath"
	"testing"
)

// secureRoot must refuse a symlink root before any modification: the previous
// os.Chmod followed the link and mutated its target (audit AD-9).
func TestSecureRootRefusesSymlinkWithoutMutatingTarget(t *testing.T) {
	target := t.TempDir()
	if err := os.Chmod(target, 0o777); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "root")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if err := secureRoot(link); err == nil {
		t.Fatal("symlink root accepted")
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o777 {
		t.Fatal("secureRoot mutated the symlink target")
	}
}

func TestSecureRootRefusesGroupOtherWrite(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0o777); err != nil {
		t.Fatal(err)
	}
	if err := secureRoot(root); err == nil {
		t.Fatal("group/other-write root accepted")
	}
}

// The writability boundary accepts an owner-private 0755 root (Go 1.27
// t.TempDir) without chmod-ing it, matching the spending family.
func TestSecureRootAcceptsOwnerPrivateRootWithoutChmod(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := secureRoot(root); err != nil {
		t.Fatal("owner-private 0755 root refused", err)
	}
	info, err := os.Stat(root)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Fatal("secureRoot chmod-ed an accepted root")
	}
}
