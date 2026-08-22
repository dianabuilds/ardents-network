//go:build !windows

package update

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

var errLockBusy = errors.New("update transaction busy")
var errLockIdentity = errors.New("update transaction lock identity is invalid")

const lockFileName = ".ardents-update-transaction-lock"

type ownedLock struct {
	path string
	file *os.File
}

type lockIdentity struct {
	dev, inode, mode, nlink uint64
	size                    int64
}

// acquireOwnedLock opens the existing permanent lock and proves
// identity through held-handle fstat plus a non-following path lstat
// after non-blocking Flock acquisition. Only EWOULDBLOCK or EAGAIN
// proves contention. The lock is never created, repaired, replaced,
// retried, or unlinked by the Module; installer/portable bootstrap
// owns its lifecycle.
func acquireOwnedLock(root string) (*ownedLock, error) {
	lockPath := filepath.Join(root, lockFileName)
	file, err := os.OpenFile(lockPath, os.O_RDWR|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, fmt.Errorf("%w: open: %v", errLockIdentity, err)
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		flockCloseErr := file.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
			return nil, errors.Join(fmt.Errorf("%w: %v", errLockBusy, err), flockCloseErr)
		}
		return nil, errors.Join(fmt.Errorf("%w: flock: %v", errLockIdentity, err), flockCloseErr)
	}
	handle, handleErr := fstatHandle(file)
	if handleErr != nil {
		unlockErr := syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
		closeErr := file.Close()
		return nil, errors.Join(fmt.Errorf("%w: handle: %v", errLockIdentity, handleErr), unlockErr, closeErr)
	}
	if err := validateLockIdentity(handle); err != nil {
		unlockErr := syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
		closeErr := file.Close()
		return nil, errors.Join(fmt.Errorf("%w: %v", errLockIdentity, err), unlockErr, closeErr)
	}
	pathIdentity, pathErr := lstatPath(lockPath)
	if pathErr != nil {
		unlockErr := syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
		closeErr := file.Close()
		return nil, errors.Join(fmt.Errorf("%w: path lstat: %v", errLockIdentity, pathErr), unlockErr, closeErr)
	}
	if !identityMatches(handle, pathIdentity) {
		unlockErr := syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
		closeErr := file.Close()
		return nil, errors.Join(fmt.Errorf("%w: handle/path identity mismatch", errLockIdentity), unlockErr, closeErr)
	}
	return &ownedLock{path: lockPath, file: file}, nil
}

func fstatHandle(file *os.File) (lockIdentity, error) {
	var raw syscall.Stat_t
	if err := syscall.Fstat(int(file.Fd()), &raw); err != nil {
		return lockIdentity{}, err
	}
	return lockIdentity{dev: uint64(raw.Dev), inode: uint64(raw.Ino), mode: uint64(raw.Mode), nlink: uint64(raw.Nlink), size: raw.Size}, nil
}

func lstatPath(path string) (lockIdentity, error) {
	var raw syscall.Stat_t
	if err := syscall.Lstat(path, &raw); err != nil {
		return lockIdentity{}, err
	}
	return lockIdentity{dev: uint64(raw.Dev), inode: uint64(raw.Ino), mode: uint64(raw.Mode), nlink: uint64(raw.Nlink), size: raw.Size}, nil
}

func validateLockIdentity(identity lockIdentity) error {
	if identity.mode&syscall.S_IFLNK != 0 || identity.mode&syscall.S_IFREG == 0 {
		return errors.New("not a direct regular file")
	}
	if identity.size != 0 {
		return errors.New("not empty")
	}
	if identity.nlink != 1 {
		return fmt.Errorf("nlink=%d", identity.nlink)
	}
	return nil
}

func identityMatches(handle, path lockIdentity) bool {
	return handle.dev == path.dev && handle.inode == path.inode &&
		handle.mode == path.mode && handle.nlink == path.nlink
}

// release joins unlock and close errors without ever removing or
// replacing the permanent lock path. The installer/portable bootstrap
// owns the lock's lifecycle; this Module only observes failures.
func (l *ownedLock) release() error {
	if l == nil || l.file == nil {
		return nil
	}
	unlockErr := syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN)
	closeErr := l.file.Close()
	l.file = nil
	return errors.Join(unlockErr, closeErr)
}
