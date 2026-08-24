package portable

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type localPaths struct {
	runtime string
	lock    string
}

func prepareRoots(config Config) (localPaths, error) {
	for _, root := range []string{config.ConfigHome, config.StateHome, config.CacheHome, config.RuntimeHome} {
		if !filepath.IsAbs(root) {
			return localPaths{}, errors.New("local profile root is not absolute")
		}
		if err := ensureBaseDirectory(root); err != nil {
			return localPaths{}, err
		}
	}
	if err := validateRuntimeBase(config.RuntimeHome); err != nil {
		return localPaths{}, err
	}
	for _, path := range []string{
		config.ConfigHome,
		filepath.Join(config.ConfigHome, "grants"),
		config.StateHome,
		filepath.Join(config.StateHome, "vault"),
		filepath.Join(config.StateHome, "floors"),
		filepath.Join(config.StateHome, "diagnostics"),
		filepath.Join(config.StateHome, "live"),
		config.CacheHome,
		config.RuntimeHome,
	} {
		if err := ensureOwnedDirectory(path); err != nil {
			return localPaths{}, err
		}
	}
	return localPaths{runtime: config.RuntimeHome, lock: filepath.Join(config.StateHome, "live", "owner.lock")}, nil
}

func ensureBaseDirectory(path string) error {
	if err := os.MkdirAll(path, 0o700); err != nil {
		return fmt.Errorf("create local profile base: %w", err)
	}
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("inspect local profile base: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return errors.New("local profile base is not a directory")
	}
	return nil
}

func ensureOwnedDirectory(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		if err := os.Mkdir(path, 0o700); err != nil {
			return fmt.Errorf("create owned local directory: %w", err)
		}
		info, err = os.Lstat(path)
	}
	if err != nil {
		return fmt.Errorf("inspect owned local directory: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return errors.New("owned local path is not a directory")
	}
	return secureOwnedDirectory(path, info)
}
