//go:build !windows

package durable

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/dianabuilds/ardents-network/internal/durablefile"
)

type rootLease struct{ file *os.File }

func acquireRootLease(root string) (rootLease, error) {
	return openRootLease(root, os.O_RDWR|os.O_CREATE)
}

func acquireFreshRootLease(root string) (rootLease, error) {
	return openRootLease(root, os.O_RDWR|os.O_CREATE|os.O_EXCL)
}

func acquireRetainedRootLease(root string) (rootLease, error) {
	return openRootLease(root, os.O_RDWR)
}

func openRootLease(root string, flags int) (rootLease, error) {
	file, err := durablefile.OpenLease(filepath.Join(root, rootLockName), flags|syscall.O_NOFOLLOW, "open state-root lease", "acquire exclusive state-root lease")
	if err != nil {
		return rootLease{}, err
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

func syncDirectory(path string) error {
	return durablefile.SyncDirectory(path)
}
