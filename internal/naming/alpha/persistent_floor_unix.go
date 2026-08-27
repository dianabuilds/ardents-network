//go:build !windows

package alpha

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

type persistentFloorLease struct{ file *os.File }

func acquirePersistentFloorLease(root string) (persistentFloorLease, error) {
	file, err := os.OpenFile(filepath.Join(root, persistentFloorLock), os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return persistentFloorLease{}, fmt.Errorf("open alpha persistent floor lease: %w", err)
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = file.Close()
		return persistentFloorLease{}, fmt.Errorf("acquire alpha persistent floor lease: %w", err)
	}
	return persistentFloorLease{file: file}, nil
}

func (lease persistentFloorLease) release() error {
	if lease.file == nil {
		return nil
	}
	if err := syscall.Flock(int(lease.file.Fd()), syscall.LOCK_UN); err != nil {
		_ = lease.file.Close()
		return fmt.Errorf("unlock alpha persistent floor lease: %w", err)
	}
	return lease.file.Close()
}

func durablePersistentFloorRename(oldPath, newPath string) error { return os.Rename(oldPath, newPath) }

func syncPersistentFloorDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	syncErr := directory.Sync()
	closeErr := directory.Close()
	if syncErr != nil {
		return syncErr
	}
	return closeErr
}
