package infrastructure

import (
	"errors"
	"os"
	"syscall"
)

// The caller owns a returned descriptor even on a partial observation failure.
// This physical mechanism grants neither birth provenance nor writer custody.
func openSourceDirectory(path string, mode os.FileMode, uid, gid uint32) (*os.File, os.FileInfo, error) {
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_DIRECTORY, 0)
	if err != nil {
		return nil, nil, err
	}
	info, err := f.Stat()
	named, ne := os.Lstat(path)
	if err != nil || ne != nil || !sourceAccess(info, mode, uid, gid, true) || !sourceAccess(named, mode, uid, gid, true) || !os.SameFile(info, named) {
		return f, nil, errors.Join(ErrBinding, err, ne)
	}
	return f, info, nil
}

func observeSourceDirectory(path string, file *os.File, original os.FileInfo) error {
	if file == nil || original == nil {
		return ErrInput
	}
	native, ok := original.Sys().(*syscall.Stat_t)
	if !ok {
		return ErrBinding
	}
	opened, err := file.Stat()
	named, ne := os.Lstat(path)
	if err != nil || ne != nil || !os.SameFile(original, opened) || !os.SameFile(original, named) ||
		!sourceAccess(opened, original.Mode().Perm(), native.Uid, native.Gid, true) || !sourceAccess(named, original.Mode().Perm(), native.Uid, native.Gid, true) {
		return errors.Join(ErrBinding, err, ne)
	}
	return nil
}
