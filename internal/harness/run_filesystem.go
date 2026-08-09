package harness

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var runIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

func carrierLabComposePath(repositoryRoot string) string {
	return filepath.Join(repositoryRoot, "carrier-lab", "compose.yaml")
}

func requireCanonicalDirectory(path string) error {
	if path == "" || !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return errors.New("path must be absolute and clean")
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return errors.New("path must name a real directory, not a symlink")
	}
	return requireNoSymlinkComponents(path)
}

func requireNoSymlinkComponents(path string) error {
	volume := filepath.VolumeName(path)
	remainder := strings.TrimPrefix(path, volume)
	current := volume
	if strings.HasPrefix(remainder, string(filepath.Separator)) {
		current += string(filepath.Separator)
		remainder = strings.TrimLeft(remainder, string(filepath.Separator))
	}
	for _, part := range strings.Split(remainder, string(filepath.Separator)) {
		if part == "" {
			continue
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return errors.New("path contains a symbolic link")
		}
	}
	return nil
}

func writeAtomic(path string, data []byte, mode os.FileMode) (runErr error) {
	directory := filepath.Dir(path)
	if err := requireCanonicalDirectory(directory); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(directory, ".carrier-lab-evidence-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer func() {
		if cleanupErr := os.Remove(temporaryPath); cleanupErr != nil && !os.IsNotExist(cleanupErr) {
			runErr = errors.Join(runErr, cleanupErr)
		}
	}()
	if err := temporary.Chmod(mode); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("publish evidence: %w", err)
	}
	return nil
}

func removeSmokeRunDirectory(layout runLayout) error {
	if _, err := ownedLayout(layout.identity, false, false); err != nil {
		return err
	}
	if info, err := os.Lstat(layout.runDir); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return errors.New("smoke run path is not an owned directory")
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.RemoveAll(layout.runDir); err != nil {
		return err
	}
	if err := os.RemoveAll(layout.runDir); err != nil {
		return err
	}
	if _, err := os.Stat(layout.runDir); !os.IsNotExist(err) {
		return errors.New("smoke run directory remains after cleanup")
	}
	return nil
}
