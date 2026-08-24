//go:build !windows

package portable

import (
	"errors"
	"os"
	"syscall"
)

type ownerLease struct{ file *os.File }

func acquireOwnerLease(path string) (ownerLease, error) {
	fd, err := syscall.Open(path, syscall.O_CREAT|syscall.O_RDWR|syscall.O_CLOEXEC|syscall.O_NOFOLLOW, 0o600)
	if err != nil {
		return ownerLease{}, lifecycleError(ReasonLockError, err)
	}
	file := os.NewFile(uintptr(fd), path)
	if file == nil {
		_ = syscall.Close(fd)
		return ownerLease{}, lifecycleError(ReasonLockError, errors.New("open owner lock"))
	}
	var status syscall.Stat_t
	if err := syscall.Fstat(fd, &status); err != nil || status.Mode&syscall.S_IFMT != syscall.S_IFREG ||
		status.Mode&0o777 != 0o600 || status.Uid != uint32(os.Geteuid()) {
		_ = file.Close()
		return ownerLease{}, lifecycleError(ReasonLockError, errors.New("owner lock is not a private regular file"))
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = file.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
			return ownerLease{}, lifecycleError(ReasonOwnerBusy, err)
		}
		return ownerLease{}, lifecycleError(ReasonLockError, err)
	}
	return ownerLease{file: file}, nil
}

func (lease ownerLease) release() error {
	if lease.file == nil {
		return nil
	}
	return errors.Join(syscall.Flock(int(lease.file.Fd()), syscall.LOCK_UN), lease.file.Close())
}
