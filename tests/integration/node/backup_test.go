//go:build integration

package node_test

import (
	"os"
	"path/filepath"

	"ardents/internal/storage"
)

func copyStoppedBackup(destination, source string) error {
	if err := storage.EnsurePrivateDir(destination); err != nil {
		return err
	}
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
		target := filepath.Join(destination, relative)
		if info.IsDir() {
			return storage.EnsurePrivateDir(target)
		}
		return storage.ProtectPrivateFile(target)
	})
}
