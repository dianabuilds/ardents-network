//go:build linux

package attempts

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

// Root access, lease and directory persistence are private mechanisms of the
// one attempt owner. They grant no separate public root-management capability.
type rootLease struct{ file *os.File }

func secureRoot(root string) error {
	if err := os.Chmod(root, 0o700); err != nil {
		return err
	}
	var status syscall.Stat_t
	if err := syscall.Lstat(root, &status); err != nil {
		return err
	}
	if status.Mode&syscall.S_IFMT != syscall.S_IFDIR || status.Mode&0o777 != 0o700 || status.Uid != uint32(os.Geteuid()) {
		return errors.New("endpoint root is not private to the endpoint user")
	}
	return nil
}

func acquireLease(path string, fresh bool) (*rootLease, error) {
	flags := os.O_RDWR
	if fresh {
		flags |= os.O_CREATE | os.O_EXCL
	}
	file, err := os.OpenFile(path, flags, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = file.Close()
		return nil, errors.New("endpoint root is already owned")
	}
	return &rootLease{file: file}, nil
}

func (lease *rootLease) release() error {
	if lease == nil || lease.file == nil {
		return nil
	}
	return errors.Join(syscall.Flock(int(lease.file.Fd()), syscall.LOCK_UN), lease.file.Close())
}

func syncDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	return errors.Join(directory.Sync(), directory.Close())
}

func syncJournalRoot(root string) error {
	for _, name := range []string{"owner.lock", "root.marker", "attempts"} {
		file, err := os.OpenFile(filepath.Join(root, name), os.O_RDWR, 0)
		if err != nil {
			return err
		}
		if err := errors.Join(file.Sync(), file.Close()); err != nil {
			return err
		}
	}
	return syncDirectory(root)
}
