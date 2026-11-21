package instance

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"
)

const nativeSupported = true

type rootLease struct{ file *os.File }

func privateAccess(info os.FileInfo) error {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != uint32(os.Geteuid()) || info.Mode().Perm()&0077 != 0 || (!info.IsDir() && stat.Nlink != 1) {
		return ErrInvalid
	}
	return nil
}

func prepareDirectory(path string) error {
	// The private root needs an existing parent; no ambient recursive adoption.
	for current := filepath.Dir(path); ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return ErrInvalid
		}
		if filepath.Dir(current) == current {
			break
		}
	}
	if err := os.Mkdir(path, 0700); err != nil && !errors.Is(err, os.ErrExist) {
		return err
	}
	return syncDirectory(filepath.Dir(path))
}

func acquireLease(path string) (rootLease, error) {
	file, err := os.OpenFile(filepath.Join(path, lockName), os.O_RDWR|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return rootLease{}, err
	}
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() != 0 {
		return rootLease{}, errors.Join(ErrInvalid, err, file.Close())
	}
	if err = privateAccess(info); err != nil {
		return rootLease{}, errors.Join(err, file.Close())
	}
	if err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return rootLease{}, errors.Join(err, file.Close())
	}
	lease := rootLease{file: file}
	if err = lease.check(path); err != nil {
		return rootLease{}, errors.Join(err, lease.release())
	}
	return lease, nil
}

func (lease *rootLease) check(path string) error {
	if lease.file == nil {
		return ErrUnavailable
	}
	info, err := lease.file.Stat()
	if err != nil {
		return err
	}
	current, err := os.Lstat(filepath.Join(path, lockName))
	if err != nil || !current.Mode().IsRegular() || !os.SameFile(info, current) {
		return ErrUnavailable
	}
	return privateAccess(current)
}

func (lease *rootLease) release() error {
	if lease.file == nil {
		return nil
	}
	err := errors.Join(syscall.Flock(int(lease.file.Fd()), syscall.LOCK_UN), lease.file.Close())
	lease.file = nil
	return err
}

func syncDirectory(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	return errors.Join(file.Sync(), file.Close())
}

func readPrivate(path string, maximum int64) ([]byte, error) {
	file, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, err
	}
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > maximum {
		return nil, errors.Join(ErrInvalid, err, file.Close())
	}
	if err = privateAccess(info); err != nil {
		return nil, errors.Join(err, file.Close())
	}
	raw, readErr := io.ReadAll(io.LimitReader(file, maximum+1))
	current, pathErr := os.Lstat(path)
	if pathErr != nil || !current.Mode().IsRegular() || !os.SameFile(info, current) {
		readErr = errors.Join(readErr, ErrUnavailable, pathErr)
	}
	if int64(len(raw)) > maximum {
		readErr = errors.Join(readErr, ErrInvalid)
	}
	closeErr := file.Close()
	if err = errors.Join(readErr, closeErr); err != nil {
		clear(raw)
		return nil, err
	}
	return raw, nil
}
