package modulecache

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func pruneVolatileState(root string) error {
	download := filepath.Join(root, "cache", "download")
	if _, err := os.Lstat(download); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	if err := os.RemoveAll(filepath.Join(download, "sumdb")); err != nil {
		return err
	}
	return filepath.WalkDir(download, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		name := entry.Name()
		if name == "list" || strings.HasSuffix(name, ".lock") || strings.HasSuffix(name, ".tmp") ||
			strings.HasSuffix(name, ".partial") {
			return os.Remove(path)
		}
		return nil
	})
}
