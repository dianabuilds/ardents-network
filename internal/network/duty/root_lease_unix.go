//go:build !windows

package duty

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"

	"github.com/dianabuilds/ardents-network/internal/durablefile"
)

type rootLease struct{ file *os.File }

func acquireRootLeaseMode(root string, create bool) (rootLease, error) {
	flags := os.O_RDWR
	if create {
		flags |= os.O_CREATE
	}
	return openRootLease(root, flags)
}

func acquireFreshRootLease(root string) (rootLease, error) {
	return openRootLease(root, os.O_RDWR|os.O_CREATE|os.O_EXCL)
}

func openRootLease(root string, flags int) (rootLease, error) {
	file, err := durablefile.OpenLease(filepath.Join(root, rootLockName), flags|syscall.O_NOFOLLOW, "", "acquire exclusive local role lease")
	if err != nil {
		return rootLease{}, err
	}
	return rootLease{file: file}, nil
}

func (lease rootLease) release() error {
	return durablefile.ReleaseLease(lease.file)
}

func rootLeaseBusy(err error) bool {
	return errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN)
}
