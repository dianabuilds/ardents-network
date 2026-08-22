//go:build !windows

package duty

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

type rootLease struct{ file *os.File }

func acquireRootLease(root string) (rootLease, error) {
	file, err := os.OpenFile(filepath.Join(root, rootLockName), os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return rootLease{}, err
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = file.Close()
		return rootLease{}, fmt.Errorf("acquire exclusive local role lease: %w", err)
	}
	return rootLease{file: file}, nil
}

func (lease rootLease) release() error {
	if lease.file == nil {
		return nil
	}
	unlockErr := syscall.Flock(int(lease.file.Fd()), syscall.LOCK_UN)
	return errorsJoin(unlockErr, lease.file.Close())
}

func errorsJoin(first, second error) error {
	if first != nil {
		return first
	}
	return second
}
