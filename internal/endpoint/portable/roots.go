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

// prepareRoots creates and validates only the roots that have a live
// consumer: the state base with its owner-lock and Release-floor parents,
// and the runtime base with the local attachment. ADR-0108 (F-26) stopped
// creating the unconsumed grants, vault, diagnostics, and cache scaffold;
// existing on-disk bytes of former profiles stay untouched.
func prepareRoots(config Config) (localPaths, error) {
	for _, root := range []string{config.StateHome, config.RuntimeHome} {
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
		config.StateHome,
		filepath.Join(config.StateHome, "floors"),
		filepath.Join(config.StateHome, "live"),
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
