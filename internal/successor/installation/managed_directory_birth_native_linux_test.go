//go:build installation_native

package installation

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// UID/GID here select a real kernel filesystem owner, not an Endpoint account
// or installation authorization. The actual account constructor is separate.
func TestInstallationNativeManagedDirectoryBirthAndRefusal(t *testing.T) {
	parent := nativeRequestDirectory(t)
	path := filepath.Join(parent, "nested", "state")
	created := make(map[string]os.FileInfo)
	if err := createManagedDirectory(t.Context(), path, 65534, 65534, created); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(path)
	if err != nil || !managedDirectory(info, 65534, 65534) || !os.SameFile(created[path], info) {
		t.Fatal("native birth identity differs")
	}
	if err := createManagedDirectory(t.Context(), path, 65534, 65534, created); err != nil {
		t.Fatal("own sibling reuse refused")
	}
	if err := createManagedDirectory(t.Context(), path, 65534, 65534, make(map[string]os.FileInfo)); err == nil {
		t.Fatal("foreign root adopted")
	}
	if err := os.Rename(path, path+".original"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chown(path, 65534, 65534); err != nil {
		t.Fatal(err)
	}
	if err := createManagedDirectory(t.Context(), path, 65534, 65534, created); err == nil {
		t.Fatal("matching mode/owner substituted an inode")
	}
}

func TestInstallationNativeManagedDirectoryOriginalCancellation(t *testing.T) {
	parent := nativeRequestDirectory(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	path := filepath.Join(parent, "never-born")
	if err := createManagedDirectory(ctx, path, 65534, 65534, make(map[string]os.FileInfo)); !errors.Is(err, context.Canceled) {
		t.Fatal("original cancellation lost")
	}
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatal("cancelled work created a root")
	}
}
