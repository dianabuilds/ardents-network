//go:build !windows

package update

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

type recoveryFileIdentity struct {
	device, inode uint64
	mode, links   uint64
	size          int64
}

func recoveryOpen(path string, directory bool) (*os.File, error) {
	file, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, fmt.Errorf("%w: open %s: %v", errInventoryInvalid, path, err)
	}
	if err := recoveryRevalidate(file, path, directory, -1); err != nil {
		return nil, errors.Join(err, file.Close())
	}
	return file, nil
}

func recoveryRevalidate(file *os.File, path string, directory bool, expectedSize int64) error {
	handle, err := recoveryHandleIdentity(file)
	if err != nil {
		return err
	}
	var raw syscall.Stat_t
	if err := syscall.Lstat(path, &raw); err != nil {
		return err
	}
	current := recoveryIdentityFromStat(raw)
	if handle.device != current.device || handle.inode != current.inode || handle.mode != current.mode || handle.links != current.links {
		return fmt.Errorf("%w: handle/path identity mismatch", errInventoryInvalid)
	}
	fileType := handle.mode & syscall.S_IFMT
	if directory {
		if fileType != syscall.S_IFDIR {
			return fmt.Errorf("%w: expected directory", errInventoryInvalid)
		}
		return nil
	}
	if fileType != syscall.S_IFREG || handle.links != 1 {
		return fmt.Errorf("%w: expected direct single-link regular file", errInventoryInvalid)
	}
	if expectedSize >= 0 && handle.size != expectedSize {
		return fmt.Errorf("%w: file size changed", errInventoryInvalid)
	}
	return nil
}

func recoveryHandleIdentity(file *os.File) (recoveryFileIdentity, error) {
	var raw syscall.Stat_t
	if err := syscall.Fstat(int(file.Fd()), &raw); err != nil {
		return recoveryFileIdentity{}, err
	}
	return recoveryIdentityFromStat(raw), nil
}

func recoveryIdentityFromStat(raw syscall.Stat_t) recoveryFileIdentity {
	return recoveryFileIdentity{device: uint64(raw.Dev), inode: uint64(raw.Ino), mode: uint64(raw.Mode), links: uint64(raw.Nlink), size: raw.Size}
}
