//go:build !windows

package durableroot

import (
	"errors"
	"os"
	"syscall"
)

type Lease struct{ file *os.File }

func Acquire(path string) (*Lease, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = file.Close()
		return nil, errors.New("endpoint root is already owned")
	}
	return &Lease{file: file}, nil
}

func (lease *Lease) Release() error {
	if lease == nil || lease.file == nil {
		return nil
	}
	return errors.Join(syscall.Flock(int(lease.file.Fd()), syscall.LOCK_UN), lease.file.Close())
}
