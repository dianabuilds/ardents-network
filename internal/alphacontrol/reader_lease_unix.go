//go:build !windows

package alphacontrol

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

type readerLease struct{ file *os.File }

func acquireReaderLease(root string) (readerLease, error) {
	file, err := os.OpenFile(filepath.Join(root, readerLockName), os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return readerLease{}, fmt.Errorf("open alpha control reader lease: %w", err)
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = file.Close()
		return readerLease{}, fmt.Errorf("acquire alpha control reader lease: %w", err)
	}
	return readerLease{file: file}, nil
}

func (lease readerLease) release() error {
	if lease.file == nil {
		return nil
	}
	if err := syscall.Flock(int(lease.file.Fd()), syscall.LOCK_UN); err != nil {
		_ = lease.file.Close()
		return fmt.Errorf("unlock alpha control reader lease: %w", err)
	}
	return lease.file.Close()
}
