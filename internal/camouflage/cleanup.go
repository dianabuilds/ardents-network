package camouflage

import (
	"errors"
	"os"
	"path/filepath"
	"time"
)

var errCleanupFailed = errors.New("adapter cleanup failed")

func cleanupFailure(err error) error {
	if err == nil {
		return nil
	}
	return errors.Join(errCleanupFailed, err)
}

func cleanupDeadline(parent time.Time) time.Time {
	deadline := time.Now().Add(6 * time.Second)
	if parent.Before(deadline) {
		return parent
	}
	return deadline
}

func removeAndVerifyState(path string, deadline time.Time) error {
	paths, bytes, err := inspectState(path, deadline)
	if err != nil {
		return err
	}
	if len(paths)-1 > 32 || bytes > 1<<20 {
		return errors.New("adapter state exceeded its resource bound")
	}
	for index := len(paths) - 1; index >= 0; index-- {
		if !time.Now().Before(deadline) {
			return errors.New("adapter cleanup deadline expired")
		}
		if err := os.Remove(paths[index]); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	if !time.Now().Before(deadline) {
		return errors.New("adapter cleanup deadline expired")
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		return errors.New("adapter state residue remains")
	}
	return nil
}

func inspectState(root string, deadline time.Time) ([]string, int64, error) {
	paths := make([]string, 0, 33)
	var bytes int64
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !time.Now().Before(deadline) {
			return errors.New("adapter cleanup deadline expired")
		}
		paths = append(paths, path)
		if len(paths) > 33 || entry.Type()&os.ModeSymlink != 0 {
			return errors.New("adapter state exceeded its resource bound")
		}
		if path != root && !entry.IsDir() {
			info, err := entry.Info()
			if err != nil || !info.Mode().IsRegular() {
				return errors.New("adapter state contains an unsupported entry")
			}
			bytes += info.Size()
		}
		return nil
	})
	return paths, bytes, err
}
