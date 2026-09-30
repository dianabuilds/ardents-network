//go:build linux

package installation

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestPrivateManagedDirectoriesPreserveForeignPaths(t *testing.T) {
	if os.Geteuid() != 0 {
		if err := createPrivateManagedDirectory(t.TempDir(), 1001, 1001, map[string]bool{}); err == nil {
			t.Fatal("existing directory adopted")
		}
		return
	}
	base, err := os.MkdirTemp("/root", "ardents-private-directory-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(base); err != nil {
			t.Error(err)
		}
	})
	path := filepath.Join(base, "new-parent", "state")
	created := map[string]bool{}
	if err := createPrivateManagedDirectory(path, 1001, 1001, created); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	identity, ok := info.Sys().(*syscall.Stat_t)
	if !ok || identity.Uid != 1001 || identity.Gid != 1001 || info.Mode().Perm() != 0700 {
		t.Fatal("private root ownership differs")
	}
	if err := createPrivateManagedDirectory(filepath.Join(path, "nested"), 1001, 1001, created); err != nil {
		t.Fatal(err)
	}
	if err := createPrivateManagedDirectory(path, 1001, 1001, map[string]bool{}); err == nil {
		t.Fatal("foreign existing root adopted")
	}
	if err := os.Chmod(path, 0755); err != nil {
		t.Fatal(err)
	}
	if err := createPrivateManagedDirectory(path, 1001, 1001, created); err == nil {
		t.Fatal("modified owned root accepted")
	}
	link := filepath.Join(base, "link")
	if err := os.Symlink(filepath.Dir(path), link); err != nil {
		t.Fatal(err)
	}
	if err := createPrivateManagedDirectory(filepath.Join(link, "new-child"), 1001, 1001, created); err == nil {
		t.Fatal("symlinked parent accepted")
	}
}
