//go:build linux

package installation

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

// acquireInstallationLease serializes root writers. The retained empty file
// carries no transition or recovery authority.
func acquireInstallationLease(root string) (*os.File, error) {
	return acquireRootLease(filepath.Join(root, "writer.lock"))
}

func acquireRootLease(path string) (*os.File, error) {
	root := filepath.Dir(path)
	if err := checkRootAncestors(root, false); err != nil {
		return nil, err
	}
	fd, err := syscall.Open(path, syscall.O_RDWR|syscall.O_CREAT|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0600)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), path)
	var identity syscall.Stat_t
	if err := syscall.Fstat(fd, &identity); err != nil {
		return nil, errors.Join(err, file.Close())
	}
	if identity.Mode&syscall.S_IFMT != syscall.S_IFREG || identity.Mode&0777 != 0600 || identity.Uid != 0 || identity.Gid != 0 || identity.Nlink != 1 || identity.Size != 0 {
		return nil, errors.Join(errors.New("installation writer lease file is untrusted"), file.Close())
	}
	if err := syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return nil, errors.Join(err, file.Close())
	}
	if err := syncDirectory(root); err != nil {
		return nil, errors.Join(err, file.Close())
	}
	return file, nil
}
