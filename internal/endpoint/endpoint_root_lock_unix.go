//go:build !windows

package endpoint

import (
	"errors"
	"os"
	"syscall"
)

type endpointRootLease struct{ file *os.File }

func acquireEndpointRootLease(path string) (endpointRootLease, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return endpointRootLease{}, err
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = file.Close()
		return endpointRootLease{}, errors.New("Endpoint root is already owned")
	}
	return endpointRootLease{file: file}, nil
}

func (lease endpointRootLease) release() error {
	if lease.file == nil {
		return nil
	}
	return errors.Join(syscall.Flock(int(lease.file.Fd()), syscall.LOCK_UN), lease.file.Close())
}
