//go:build !windows

package publication

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

type rootLease struct{ file *os.File }

func acquireRootLease(root string) (rootLease, error) {
	file, err := os.OpenFile(filepath.Join(root, rootLockName), os.O_RDWR|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return rootLease{}, fmt.Errorf("open publication root lease: %w", err)
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		closeErr := file.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
			return rootLease{}, errors.Join(errors.New("publication root is already owned"), closeErr)
		}
		return rootLease{}, errors.Join(fmt.Errorf("lock publication root: %w", err), closeErr)
	}
	return rootLease{file: file}, nil
}

func (lease *rootLease) release() error {
	if lease == nil || lease.file == nil {
		return nil
	}
	err := errors.Join(syscall.Flock(int(lease.file.Fd()), syscall.LOCK_UN), lease.file.Close())
	lease.file = nil
	return err
}
