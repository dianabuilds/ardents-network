//go:build !windows

package duty

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

type rootLease struct{ file *os.File }

func acquireRootLeaseMode(root string, create bool) (rootLease, error) {
	flags := os.O_RDWR
	if create {
		flags |= os.O_CREATE
	}
	file, err := os.OpenFile(filepath.Join(root, rootLockName), flags|syscall.O_NOFOLLOW, 0o600)
	if err != nil {
		return rootLease{}, err
	}
	info, err := file.Stat()
	if err != nil {
		return rootLease{}, errors.Join(err, file.Close())
	}
	native, ok := info.Sys().(*syscall.Stat_t)
	if !ok || !info.Mode().IsRegular() || native.Nlink != 1 {
		return rootLease{}, errors.Join(errors.New("invalid history lease identity"), file.Close())
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
	return errors.Join(unlockErr, lease.file.Close())
}

func rootLeaseBusy(err error) bool {
	return errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN)
}
