//go:build ignore

package main

import (
	"errors"
	"os"
	"syscall"
)

func secureOwnedDirectory(path string, info os.FileInfo) error {
	status, ok := info.Sys().(*syscall.Stat_t)
	if !ok || status.Uid != uint32(os.Geteuid()) {
		return errors.New("directory is not owned by the endpoint user")
	}
	if err := os.Chmod(path, 0o700); err != nil {
		return err
	}
	return validatePrivatePath(path, syscall.S_IFDIR, 0o700)
}

func secureAttachment(path string) error {
	if err := os.Chmod(path, 0o600); err != nil {
		return err
	}
	return validatePrivatePath(path, syscall.S_IFSOCK, 0o600)
}

func validatePrivatePath(path string, kind uint32, permissions uint32) error {
	var status syscall.Stat_t
	if err := syscall.Lstat(path, &status); err != nil {
		return err
	}
	if status.Mode&syscall.S_IFMT != kind || status.Mode&0o777 != permissions ||
		status.Uid != uint32(os.Geteuid()) {
		return errors.New("private path policy did not round-trip")
	}
	return nil
}

func acquireOwnerLock(path string) (func() error, bool, error) {
	fd, err := syscall.Open(path, syscall.O_CREAT|syscall.O_RDWR|syscall.O_CLOEXEC|syscall.O_NOFOLLOW, 0o600)
	if err != nil {
		return nil, false, err
	}
	file := os.NewFile(uintptr(fd), path)
	if file == nil {
		_ = syscall.Close(fd)
		return nil, false, errors.New("create owner lock file")
	}
	var status syscall.Stat_t
	if err := syscall.Fstat(fd, &status); err != nil {
		file.Close()
		return nil, false, err
	}
	if status.Mode&syscall.S_IFMT != syscall.S_IFREG || status.Mode&0o777 != 0o600 || status.Uid != uint32(os.Geteuid()) {
		file.Close()
		return nil, false, errors.New("owner lock must be a private regular file owned by the endpoint user")
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		file.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
			return nil, true, nil
		}
		return nil, false, err
	}
	return func() error {
		unlockErr := syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
		closeErr := file.Close()
		if unlockErr != nil {
			return unlockErr
		}
		return closeErr
	}, false, nil
}
