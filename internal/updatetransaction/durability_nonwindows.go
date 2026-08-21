//go:build !windows

package updatetransaction

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

type durabilityOps struct {
	replaceCurrent    func(string, string) error
	publishGeneration func(string, string) error
	syncDirectory     func(string) error
}

func nativeDurability() durabilityOps {
	return durabilityOps{
		replaceCurrent:    nonWindowsReplaceCurrent,
		publishGeneration: nonWindowsPublishGeneration,
		syncDirectory:     nonWindowsSyncDirectory,
	}
}

func nonWindowsReplaceCurrent(temporary, current string) error {
	if err := os.Rename(temporary, current); err != nil {
		return fmt.Errorf("replace current: %w", err)
	}
	return nonWindowsSyncDirectory(filepath.Dir(current))
}

func nonWindowsPublishGeneration(staging, generation string) error {
	if _, err := os.Lstat(generation); !errors.Is(err, os.ErrNotExist) {
		if err == nil {
			return errors.New("generation already exists")
		}
		return fmt.Errorf("inspect generation target: %w", err)
	}
	info, err := os.Lstat(staging)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.Join(errors.New("staging is not a direct directory"), err)
	}
	if err := os.Rename(staging, generation); err != nil {
		return fmt.Errorf("publish generation: %w", err)
	}
	return errors.Join(nonWindowsSyncDirectory(filepath.Dir(staging)), nonWindowsSyncDirectory(filepath.Dir(generation)))
}

func nonWindowsSyncDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	return errors.Join(directory.Sync(), directory.Close())
}

func validateOwnedPath(path string) error {
	if len(path) == 0 || len(path) > 512 || !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return errors.New("owned path is invalid")
	}
	current := string(filepath.Separator)
	for _, component := range strings.Split(strings.TrimPrefix(path, current), current) {
		if len(component) == 0 || len(component) > 64 {
			return errors.New("owned path component is invalid")
		}
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return errors.New("owned path crosses a symbolic link")
		}
	}
	return nil
}

func validateOwnedEntry(path string) error {
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink != 0 {
		return errors.Join(errRecordInvalid, err)
	}
	if info.Mode().IsRegular() {
		stat, ok := info.Sys().(*syscall.Stat_t)
		if !ok || stat.Nlink != 1 {
			return errRecordInvalid
		}
	}
	return nil
}
