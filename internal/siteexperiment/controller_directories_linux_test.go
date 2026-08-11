//go:build linux

package siteexperiment

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestReferenceRoleEvidenceDirectoriesIgnoreHostUmask(t *testing.T) {
	previous := syscall.Umask(0o077)
	t.Cleanup(func() { syscall.Umask(previous) })

	directories, err := prepareReferenceDirectories(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, role := range referenceRoles {
		path := filepath.Join(directories["evidence"], role)
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != 0o777 {
			t.Errorf("%s permissions = %04o, want 0777", role, got)
		}
	}
}
