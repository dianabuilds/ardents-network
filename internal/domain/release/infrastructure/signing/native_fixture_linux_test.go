//go:build linux && network_signing_native

package signing

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func sourceArtifactNativeParent(t *testing.T) string {
	t.Helper()
	if os.Geteuid() != 0 {
		t.Fatal("invalid environment: native preparation requires root")
	}
	parent := t.TempDir()
	if err := os.Chmod(parent, 0700); err != nil {
		t.Fatal(err)
	}
	mountType, err := exec.Command("findmnt", "--kernel", "--noheadings", "--output", "FSTYPE", "--target", parent).Output()
	if err != nil || strings.TrimSpace(string(mountType)) != "ext4" {
		t.Fatal("invalid environment: actual ext4 required", err)
	}
	for path := parent; ; path = filepath.Dir(path) {
		info, err := os.Lstat(path)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0022 != 0 || info.Sys().(*syscall.Stat_t).Uid != 0 {
			t.Fatal("invalid environment: protected original ancestor required", path, err)
		}
		if path == "/" {
			break
		}
	}
	return parent
}
