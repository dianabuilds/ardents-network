package recoverysmoke

import (
	"errors"
	"os"
	"path/filepath"
)

func ownsPrivateFixture(root string) bool {
	marker, err := os.ReadFile(filepath.Join(root, ".ardents-owned"))
	return err == nil && string(marker) == fixtureMarker
}

func removePrivateFixture(root string) error {
	if root == "" {
		return nil
	}
	if !filepath.IsAbs(root) {
		return errors.New("recovery private fixture cleanup requires an absolute path")
	}
	marker, err := os.ReadFile(filepath.Join(root, ".ardents-owned"))
	if errors.Is(err, os.ErrNotExist) {
		if _, statErr := os.Stat(root); errors.Is(statErr, os.ErrNotExist) {
			return nil
		}
	}
	if err != nil || string(marker) != fixtureMarker {
		return errors.New("recovery private fixture cleanup ownership is invalid")
	}
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return os.Remove(path)
		}
		if entry.IsDir() {
			return os.Chmod(path, 0o700)
		}
		return os.Chmod(path, 0o600)
	}); err != nil {
		return err
	}
	if err := os.RemoveAll(root); err != nil {
		return err
	}
	if _, err := os.Stat(root); !errors.Is(err, os.ErrNotExist) {
		return errors.New("recovery private fixture remains after cleanup")
	}
	return nil
}
