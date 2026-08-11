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

func TestRoleEvidenceIsReadableByOwningController(t *testing.T) {
	previous := syscall.Umask(0o077)
	t.Cleanup(func() { syscall.Umask(previous) })
	directory := t.TempDir()
	path := filepath.Join(directory, "publication.json")
	if err := writeAtomicBoundedJSON(path, map[string]string{"status": "published"}); err != nil {
		t.Fatal(err)
	}
	assertFileMode(t, path, 0o644)

	ordinary := filepath.Join(directory, "ordinary.json")
	if err := os.WriteFile(ordinary, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writeBoundedJSON(ordinary, map[string]string{"status": "completed"}); err != nil {
		t.Fatal(err)
	}
	assertFileMode(t, ordinary, 0o644)
}

func assertFileMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("%s permissions = %04o, want %04o", filepath.Base(path), got, want)
	}
}
