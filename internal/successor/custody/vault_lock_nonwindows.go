//go:build !windows

package custody

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

type vaultOperationLock struct{ file *os.File }

func acquireVaultOperationLock(root string) (*vaultOperationLock, error) {
	path := filepath.Join(root, vaultLockName)
	file, err := os.OpenFile(path, os.O_RDWR|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, ErrInvalid
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = file.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
			return nil, ErrBusy
		}
		return nil, ErrInvalid
	}
	held, heldErr := file.Stat()
	pathInfo, pathErr := os.Lstat(path)
	if heldErr != nil || pathErr != nil || !held.Mode().IsRegular() || held.Size() != 0 || !os.SameFile(held, pathInfo) {
		_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
		_ = file.Close()
		return nil, ErrInvalid
	}
	return &vaultOperationLock{file: file}, nil
}

func (lock *vaultOperationLock) release() error {
	if lock == nil || lock.file == nil {
		return nil
	}
	unlockErr := syscall.Flock(int(lock.file.Fd()), syscall.LOCK_UN)
	closeErr := lock.file.Close()
	lock.file = nil
	return errors.Join(unlockErr, closeErr)
}
