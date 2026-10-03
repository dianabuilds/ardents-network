//go:build !windows

package issuer

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

type issuerRootLease struct {
	file     *os.File
	root     string
	rootInfo os.FileInfo
}

func acquireIssuerRootLease(root string) (issuerRootLease, error) {
	rootInfo, err := os.Lstat(root)
	if err != nil {
		return issuerRootLease{}, err
	}
	retained, err := issuerRootRetained(root)
	if err != nil {
		return issuerRootLease{}, err
	}
	flags := os.O_RDWR | syscall.O_NOFOLLOW | syscall.O_NONBLOCK
	path := filepath.Join(root, issuerRootLockName)
	file, err := os.OpenFile(path, flags, 0o600)
	if errors.Is(err, os.ErrNotExist) && !retained {
		file, err = os.OpenFile(path, flags|os.O_CREATE|os.O_EXCL, 0o600)
	}
	if err != nil {
		return issuerRootLease{}, err
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = file.Close()
		return issuerRootLease{}, fmt.Errorf("acquire exclusive transit grant issuer lease: %w", err)
	}
	lease := issuerRootLease{file: file, root: root, rootInfo: rootInfo}
	if err := lease.current(); err != nil {
		return issuerRootLease{}, errors.Join(err, lease.release())
	}
	return lease, nil
}

func (lease issuerRootLease) current() error {
	if lease.file == nil {
		return errors.New("issuer lease is unavailable")
	}
	root, rootErr := os.Lstat(lease.root)
	opened, openErr := lease.file.Stat()
	retained, pathErr := os.Lstat(filepath.Join(lease.root, issuerRootLockName))
	if rootErr != nil || openErr != nil || pathErr != nil {
		return errors.Join(rootErr, openErr, pathErr)
	}
	if !root.IsDir() || !os.SameFile(root, lease.rootInfo) || root.Mode().Perm()&0o077 != 0 ||
		!retained.Mode().IsRegular() || !os.SameFile(opened, retained) || retained.Size() != 0 || retained.Mode().Perm()&0o077 != 0 {
		return errors.New("issuer retained lease identity changed")
	}
	if stat, ok := opened.Sys().(*syscall.Stat_t); !ok || stat.Nlink != 1 {
		return errors.New("issuer lease has unexpected links")
	}
	return nil
}

func (lease issuerRootLease) release() error {
	if lease.file == nil {
		return nil
	}
	return errors.Join(syscall.Flock(int(lease.file.Fd()), syscall.LOCK_UN), lease.file.Close())
}
