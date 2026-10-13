package reachability

import (
	"errors"
	"os"
	"syscall"
)

const storePlatformSupported = true

func retainStoreSyncDirectory(root *os.Root) (*os.File, error) { return root.Open(".") }

type storeLease struct{ file *os.File }

func (lease *storeLease) check(expected os.FileInfo) error {
	if lease.file == nil {
		return errors.New("reachability original lease unavailable")
	}
	info, err := lease.file.Stat()
	if err != nil || !os.SameFile(info, expected) {
		return errors.Join(errors.New("reachability original lease lost"), err)
	}
	// The private descriptor is unlocked only by release. Never reacquire a
	// possibly lost original lease as a Check/recovery shortcut.
	return nil
}

func acquireStoreLease(root *os.Root) (storeLease, error) {
	file, err := root.OpenFile(storeLockName, os.O_RDWR|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return storeLease{}, err
	}
	if err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return storeLease{}, errors.Join(err, file.Close())
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

func syncStoreDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	return errors.Join(directory.Sync(), directory.Close())
}
