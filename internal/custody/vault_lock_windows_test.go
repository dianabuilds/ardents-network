//go:build windows

package custody

import (
	"errors"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

// The portable Windows lane runs these against a real runner temp directory,
// whose path may carry 8.3 short-name components. The lock must bind the exact
// expected file without refusing a legitimate root over path-form differences.
func TestVaultOperationLockAcquiresRunnerTempRoot(t *testing.T) {
	root := t.TempDir()
	if err := prepareVaultLock(root); err != nil {
		t.Fatal(err)
	}
	lock, err := acquireVaultOperationLock(root)
	if err != nil {
		t.Fatalf("acquire vault operation lock: %v", err)
	}
	if err := lock.release(); err != nil {
		t.Fatal(err)
	}
}

func TestVaultOperationLockRefusesConcurrentHolder(t *testing.T) {
	root := t.TempDir()
	if err := prepareVaultLock(root); err != nil {
		t.Fatal(err)
	}
	lock, err := acquireVaultOperationLock(root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = lock.release() }()
	if _, err := acquireVaultOperationLock(root); !errors.Is(err, ErrBusy) {
		t.Fatalf("concurrent acquire = %v, want ErrBusy", err)
	}
}

func TestVaultLockNamesPathAcceptsShortAndLongForms(t *testing.T) {
	root := t.TempDir()
	if err := prepareVaultLock(root); err != nil {
		t.Fatal(err)
	}
	lock, err := acquireVaultOperationLock(root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = lock.release() }()
	path := filepath.Join(root, vaultLockName)
	if !vaultLockNamesPath(lock.handle, path) {
		t.Fatal("lock path check refused the long form of its own path")
	}
	encoded, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	buffer := make([]uint16, 512)
	count, err := windows.GetShortPathName(encoded, &buffer[0], uint32(len(buffer)))
	if err != nil || count == 0 || count >= uint32(len(buffer)) {
		t.Fatal("short-path form unavailable:", err)
	}
	short := windows.UTF16ToString(buffer[:count])
	if !vaultLockNamesPath(lock.handle, short) {
		t.Fatalf("lock path check refused the 8.3 short form %q of its own path", short)
	}
}
