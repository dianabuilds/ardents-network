//go:build !windows

package storage

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

func validatePrivateFile(_ string, info os.FileInfo) error {
	if info.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("private state permissions allow group or other access")
	}
	if !ownedByCurrentUser(info) {
		return fmt.Errorf("private state is not owned by the current user")
	}
	return nil
}

func validateStrictPrivateFile(path string, info os.FileInfo) error {
	return validatePrivateFile(path, info)
}

func validatePrivateDirectory(_ string, info os.FileInfo) error {
	if info.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("private state directory permissions allow group or other access")
	}
	if !ownedByCurrentUser(info) {
		return fmt.Errorf("private state directory is not owned by the current user")
	}
	return nil
}

func ownedByCurrentUser(info os.FileInfo) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && stat.Uid == uint32(os.Geteuid())
}

func protectPrivatePath(_ string, _ bool) error { return nil }

func replacePrivateFile(source, target string) error {
	if err := os.Rename(source, target); err != nil {
		return err
	}
	dir, err := os.Open(filepath.Dir(target))
	if err != nil {
		return fmt.Errorf("open private state directory for sync: %w", err)
	}
	if err := dir.Sync(); err != nil {
		_ = dir.Close()
		return fmt.Errorf("sync private state directory: %w", err)
	}
	if err := dir.Close(); err != nil {
		return fmt.Errorf("close private state directory after sync: %w", err)
	}
	return nil
}

func publishPrivateFileNoReplace(source, target string) error {
	if err := os.Link(source, target); err != nil {
		return err
	}
	dir, err := os.Open(filepath.Dir(target))
	if err == nil {
		err = dir.Sync()
		closeErr := dir.Close()
		if err == nil {
			err = closeErr
		}
	}
	if err == nil {
		return nil
	}
	removeErr := os.Remove(target)
	if cleanupDir, openErr := os.Open(filepath.Dir(target)); openErr == nil {
		syncErr := cleanupDir.Sync()
		closeErr := cleanupDir.Close()
		if syncErr != nil {
			removeErr = errors.Join(removeErr, syncErr)
		}
		if closeErr != nil {
			removeErr = errors.Join(removeErr, closeErr)
		}
	} else {
		removeErr = errors.Join(removeErr, openErr)
	}
	return errors.Join(fmt.Errorf("sync published private file: %w", err), removeErr)
}
