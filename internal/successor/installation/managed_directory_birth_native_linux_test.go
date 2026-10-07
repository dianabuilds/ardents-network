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

func TestInstallationNativePreparationFailureAfterDurableCompletion(t *testing.T) {
	directory := filepath.Join(nativeRequestDirectory(t), "preparation")
	r := preparationRecordFixture()
	j, err := createPreparationJournal(t.Context(), directory, r)
	if err != nil {
		t.Fatal(err)
	}
	r.Phase, r.UID, r.GID = "creating-mutable-roots", 1000, 1001
	if err := j.append(t.Context(), r); err != nil {
		t.Fatal(err)
	}
	r.Phase = "mutable-roots-prepared"
	if err := j.append(t.Context(), r); err != nil {
		t.Fatal(err)
	}
	original, cancel := context.WithCancel(t.Context())
	cancel()
	if err := j.recordFailure(original, context.Canceled); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(directory, "0003.json")); err != nil {
		t.Fatal("actual completed phase erased")
	}
	if _, err := os.Stat(filepath.Join(directory, "failure.json")); err != nil {
		t.Fatal("original failure not retained")
	}
	if !errors.Is(j.append(t.Context(), preparationRecordFixture()), context.Canceled) || !errors.Is(j.close(), context.Canceled) {
		t.Fatal("cleanup renewed authority or lost terminal error")
	}
}
