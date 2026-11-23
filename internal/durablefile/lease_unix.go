//go:build !windows

package durablefile

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

// OpenLease opens one exclusive root lease file and retains it only after the
// strict identity check: the opened target must be a regular file with exactly
// one link, so a symlink (refused by the caller's O_NOFOLLOW), a non-regular
// file or a multiply-linked inode can never become the store's exclusive fence.
// openWrap, when non-empty, wraps an open failure; busyWrap wraps a contended
// flock. The caller keeps ownership of the returned file and its lease lifecycle.
func OpenLease(path string, flags int, openWrap, busyWrap string) (*os.File, error) {
	file, err := os.OpenFile(path, flags, 0o600)
	if err != nil {
		if openWrap != "" {
			return nil, fmt.Errorf("%s: %w", openWrap, err)
		}
		return nil, err
	}
	info, err := file.Stat()
	if err != nil {
		return nil, errors.Join(err, file.Close())
	}
	native, ok := info.Sys().(*syscall.Stat_t)
	if !ok || !info.Mode().IsRegular() || native.Nlink != 1 {
		return nil, errors.Join(errors.New("invalid history lease identity"), file.Close())
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("%s: %w", busyWrap, err)
	}
	return file, nil
}

// ReleaseLease unlocks and closes one retained lease file, joining both results
// so a physical close failure is never hidden behind an unlock failure.
func ReleaseLease(file *os.File) error {
	if file == nil {
		return nil
	}
	return errors.Join(syscall.Flock(int(file.Fd()), syscall.LOCK_UN), file.Close())
}
