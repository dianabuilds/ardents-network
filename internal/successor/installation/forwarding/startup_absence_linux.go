package forwarding

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

// observeStartupAbsences performs fresh named observations, never a cached
// absence verdict. Contiguous names beneath one directory share only a newly
// opened physical descriptor, closed before the next group. A fresh ENOENT for
// that directory proves every descendant absent at that observation point.
// The caller brackets this scan with original/named metadata and mount custody.
func observeStartupAbsences(paths []string) error {
	for start := 0; start < len(paths); {
		directory := filepath.Dir(paths[start])
		end := start + 1
		for end < len(paths) && filepath.Dir(paths[end]) == directory {
			end++
		}
		if err := observeStartupAbsentNames(directory, paths[start:end]); err != nil {
			return err
		}
		start = end
	}
	return nil
}

func observeStartupAbsentNames(directory string, paths []string) (returned error) {
	fd, err := syscall.Open(directory, syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return errors.Join(ErrBinding, err)
	}
	defer func() {
		if err := syscall.Close(fd); err != nil {
			returned = errors.Join(returned, ErrBinding, err)
		}
	}()
	for _, path := range paths {
		// Linux AT_SYMLINK_NOFOLLOW | AT_EACCESS. F_OK tests presence of the
		// named inode, including a dangling symlink, using effective credentials.
		// Go falls back to a fresh no-follow fstatat if faccessat2 is unavailable.
		err := syscall.Faccessat(fd, filepath.Base(path), 0, 0x100|0x200)
		if !errors.Is(err, os.ErrNotExist) {
			return errors.Join(ErrBinding, err)
		}
	}
	return nil
}
