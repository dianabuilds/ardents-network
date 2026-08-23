//go:build !windows

package epoch

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

type namespaceRootLease struct{ file *os.File }

func acquireNamespaceRootLease(root string) (namespaceRootLease, error) {
	file, err := os.OpenFile(filepath.Join(root, namespaceRootLockName), os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return namespaceRootLease{}, fmt.Errorf("open naming state-root lease: %w", err)
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = file.Close()
		return namespaceRootLease{}, fmt.Errorf("acquire exclusive naming state-root lease: %w", err)
	}
	return namespaceRootLease{file: file}, nil
}

func (lease namespaceRootLease) release() error {
	if lease.file == nil {
		return nil
	}
	if err := syscall.Flock(int(lease.file.Fd()), syscall.LOCK_UN); err != nil {
		_ = lease.file.Close()
		return fmt.Errorf("unlock naming state-root lease: %w", err)
	}
	if err := lease.file.Close(); err != nil {
		return fmt.Errorf("release naming state-root lease: %w", err)
	}
	return nil
}

func syncNamespaceDirectory(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	err = file.Sync()
	closeErr := file.Close()
	if err != nil {
		return err
	}
	return closeErr
}
