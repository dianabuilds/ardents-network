//go:build linux

package blockedverify

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

type registryLease struct{ file *os.File }

func acquireRegistryLock(path string) (*registryLease, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := unix.Flock(int(file.Fd()), unix.LOCK_EX); err != nil {
		_ = file.Close()
		return nil, err
	}
	return &registryLease{file: file}, nil
}

func (lease *registryLease) close() error {
	unlockErr := unix.Flock(int(lease.file.Fd()), unix.LOCK_UN)
	return errors.Join(unlockErr, lease.file.Close())
}

func replaceRegistryFile(source, target string) error { return os.Rename(source, target) }

func syncDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}
