//go:build !windows

package contributor

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

type rootLease struct{ file *os.File }

func acquireRootLeaseFile(path string) (rootLease, error) {
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return rootLease{}, fmt.Errorf("open Contributor root lease: %w", err)
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = file.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
			return rootLease{}, errContributorRootBusy
		}
		return rootLease{}, fmt.Errorf("acquire exclusive Contributor root lease: %w", err)
	}
	return rootLease{file: file}, nil
}

func (lease rootLease) release() error {
	if lease.file == nil {
		return nil
	}
	return errors.Join(syscall.Flock(int(lease.file.Fd()), syscall.LOCK_UN), lease.file.Close())
}
