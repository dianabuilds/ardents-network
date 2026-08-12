//go:build !windows

package store

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

type rootLease struct {
	file *os.File
}

func acquireRootLease(root string) (rootLease, error) {
	file, err := os.OpenFile(filepath.Join(root, rootLockName), os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return rootLease{}, fmt.Errorf("open state-root lease: %w", err)
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = file.Close()
		return rootLease{}, fmt.Errorf("acquire exclusive state-root lease: %w", err)
	}
	return rootLease{file: file}, nil
}

func (lease rootLease) release() error {
	if lease.file == nil {
		return nil
	}
	if err := syscall.Flock(int(lease.file.Fd()), syscall.LOCK_UN); err != nil {
		_ = lease.file.Close()
		return fmt.Errorf("unlock state-root lease: %w", err)
	}
	if err := lease.file.Close(); err != nil {
		return fmt.Errorf("release state-root lease: %w", err)
	}
	return nil
}
