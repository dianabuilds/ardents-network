package join

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

// statRuntimeName owns one fresh descriptor solely for metadata observation.
// Keep the original nonblocking, no-follow and close-on-exec opening, including
// its access checks. No os.File/netpoll lifetime is needed for this synchronous
// Fstat. The descriptor is closed before returning, with its actual error kept.
func statRuntimeName(parent *inputDirectory, name string, directory bool) (syscall.Stat_t, error) {
	var info syscall.Stat_t
	if parent == nil || parent.file == nil {
		return info, ErrBinding
	}
	flags := syscall.O_RDONLY | syscall.O_NOFOLLOW | syscall.O_NONBLOCK | syscall.O_CLOEXEC
	if directory {
		flags |= syscall.O_DIRECTORY
	}
	fd, err := syscall.Openat(int(parent.file.Fd()), name, flags, 0)
	if err != nil {
		return info, err
	}
	for {
		err = syscall.Fstat(fd, &info)
		if err != syscall.EINTR {
			break
		}
	}
	closed := syscall.Close(fd)
	if err != nil || closed != nil {
		return info, errors.Join(ErrBinding, err, closed)
	}
	return info, nil
}

func sameRuntimeDirectory(before os.FileInfo, after syscall.Stat_t) bool {
	if before == nil || !before.IsDir() {
		return false
	}
	original, ok := before.Sys().(*syscall.Stat_t)
	// Directory contents may change at their own owner. As before, directory
	// size, link count and timestamps supply no immutable-file restriction.
	return ok && original != nil && original.Dev == after.Dev && original.Ino == after.Ino &&
		original.Mode == after.Mode && original.Uid == after.Uid && original.Gid == after.Gid
}

func sameRuntimeFile(before os.FileInfo, after syscall.Stat_t) bool {
	if before == nil || !before.Mode().IsRegular() {
		return false
	}
	original, ok := before.Sys().(*syscall.Stat_t)
	return ok && original != nil && original.Dev == after.Dev && original.Ino == after.Ino &&
		original.Mode == after.Mode && original.Uid == after.Uid && original.Gid == after.Gid &&
		original.Size == after.Size && original.Nlink == after.Nlink &&
		original.Ctim == after.Ctim && original.Mtim == after.Mtim
}

// checkRuntimeClockFile observes one current root writer file while its original
// opened descriptor remains held through the separate pathname observation.
// Clock contents and timestamps may advance; access, type, link count and
// opened/named identity still have to match before the physical close joins.
func checkRuntimeClockFile(parent *inputDirectory, path string) (returned error) {
	if parent == nil || parent.file == nil {
		return ErrBinding
	}
	name := filepath.Base(path)
	fd, err := syscall.Openat(int(parent.file.Fd()), name, syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
	if err != nil {
		return err
	}
	defer func() {
		if err := syscall.Close(fd); err != nil {
			returned = errors.Join(returned, &os.PathError{Op: "close", Path: name, Err: err})
		}
	}()
	var info syscall.Stat_t
	for {
		err = syscall.Fstat(fd, &info)
		if err != syscall.EINTR {
			break
		}
	}
	if err != nil {
		return errors.Join(ErrBinding, &os.PathError{Op: "stat", Path: name, Err: err})
	}
	named, err := os.Lstat(path)
	if err != nil {
		return errors.Join(ErrBinding, err)
	}
	current, ok := named.Sys().(*syscall.Stat_t)
	if !ok || current == nil || info.Dev != current.Dev || info.Ino != current.Ino ||
		info.Mode != syscall.S_IFREG|0644 || info.Uid != 0 || info.Gid != 0 || info.Nlink != 1 ||
		!nodeAccess(named, 0644, 0, 0, false) {
		return ErrBinding
	}
	return nil
}
