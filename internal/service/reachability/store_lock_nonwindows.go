//go:build !windows

package reachability

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

type storeLease struct{ file *os.File }

func acquireStoreLease(root string) (storeLease, error) {
	file, err := os.OpenFile(filepath.Join(root, storeLockName), os.O_RDWR|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return storeLease{}, fmt.Errorf("open reachability store lease: %w", err)
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		closeErr := file.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
			return storeLease{}, errors.Join(errors.New("reachability store is already owned"), closeErr)
		}
		return storeLease{}, errors.Join(fmt.Errorf("lock reachability store: %w", err), closeErr)
	}
	return storeLease{file: file}, nil
}

func (lease *storeLease) release() error {
	if lease == nil || lease.file == nil {
		return nil
	}
	err := errors.Join(syscall.Flock(int(lease.file.Fd()), syscall.LOCK_UN), lease.file.Close())
	lease.file = nil
	return err
}
