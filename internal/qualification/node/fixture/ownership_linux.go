//go:build linux

package fixture

import (
	"errors"
	"os"
	"path/filepath"
)

func assignNodeOwnership(root string) error {
	for _, name := range []string{".", "plans", "secrets", "state", "clock"} {
		if err := os.Chmod(filepath.Join(root, name), 0o711); err != nil {
			return err
		}
	}
	owned := []struct {
		uid   int
		paths []string
	}{
		{10001, []string{"plans/endpoint.json", "secrets/e", "state/e", "clock/e.observation"}},
		{11001, []string{"plans/source-1.json", "secrets/s1", "state/s1"}},
		{11002, []string{"plans/source-2.json", "secrets/s2", "state/s2"}},
		{12001, []string{"plans/node-1.json", "secrets/n1", "state/n1", "clock/n1.observation"}},
		{12002, []string{"plans/node-2.json", "secrets/n2", "state/n2", "clock/n2.observation"}},
		{13001, []string{"plans/harness.json", "secrets/h"}},
	}
	for _, ownership := range owned {
		for _, relative := range ownership.paths {
			if err := chownNodeTree(filepath.Join(root, filepath.FromSlash(relative)), ownership.uid); err != nil {
				return err
			}
		}
	}
	return nil
}

func chownNodeTree(root string, uid int) error {
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return errors.New("node owned path contains a symbolic link")
		}
		return os.Chown(path, uid, uid)
	})
}
