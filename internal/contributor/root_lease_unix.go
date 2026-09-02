//go:build !windows

package contributor

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

type rootLease struct{ file *os.File }

func acquireRootLeaseDirectory(path string) (rootLease, error) {
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_CLOEXEC|syscall.O_DIRECTORY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return rootLease{}, fmt.Errorf("open Contributor root-lease directory: %w", err)
	}
	file := os.NewFile(uintptr(fd), path)
	if file == nil {
		_ = syscall.Close(fd)
		return rootLease{}, errors.New("open Contributor root-lease directory")
	}
	var status syscall.Stat_t
	if err := syscall.Fstat(fd, &status); err != nil || status.Mode&syscall.S_IFMT != syscall.S_IFDIR {
		return rootLease{}, errors.Join(
			errors.New("contributor root-lease directory is invalid"),
			wrapRootLeaseRelease("close", file.Close()),
		)
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		closeErr := wrapRootLeaseRelease("close", file.Close())
		if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
			return rootLease{}, errors.Join(errContributorRootBusy, closeErr)
		}
		return rootLease{}, errors.Join(fmt.Errorf("acquire exclusive Contributor root lease: %w", err), closeErr)
	}
	held, statErr := file.Stat()
	pathInfo, pathErr := os.Lstat(path)
	if statErr != nil || pathErr != nil || !pathInfo.IsDir() || pathInfo.Mode()&os.ModeSymlink != 0 || !os.SameFile(held, pathInfo) {
		return rootLease{}, errors.Join(
			errors.New("contributor root-lease directory changed while acquiring the lease"),
			wrapRootLeaseRelease("unlock", syscall.Flock(int(file.Fd()), syscall.LOCK_UN)),
			wrapRootLeaseRelease("close", file.Close()),
		)
	}
	return rootLease{file: file}, nil
}

func (lease rootLease) release() error {
	if lease.file == nil {
		return nil
	}
	return errors.Join(
		wrapRootLeaseRelease("unlock", syscall.Flock(int(lease.file.Fd()), syscall.LOCK_UN)),
		wrapRootLeaseRelease("close", lease.file.Close()),
	)
}
