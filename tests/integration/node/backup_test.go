//go:build integration

package node_test

import (
	"os"
	"path/filepath"
)

func copyStoppedBackup(destination, source string) error {
	if err := os.CopyFS(destination, os.DirFS(source)); err != nil {
		return err
	}
	return filepath.Walk(source, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil || relative == "." {
			return err
		}
		return os.Chmod(filepath.Join(destination, relative), info.Mode().Perm())
	})
}
