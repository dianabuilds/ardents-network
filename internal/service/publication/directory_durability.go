package publication

import (
	"errors"
	"os"
	"path/filepath"
)

// Each newly created ancestor is linked durably before its child is created.
func createPublicationDirectory(path string, syncDirectory func(string) error) error {
	info, err := os.Lstat(path)
	if err == nil {
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("publication ancestor is not a directory")
		}
		return syncDirectory(filepath.Dir(path))
	}
	if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	parent := filepath.Dir(path)
	if parent == path {
		return err
	}
	if err := createPublicationDirectory(parent, syncDirectory); err != nil {
		return err
	}
	if err := os.Mkdir(path, 0700); err != nil {
		return err
	}
	return syncDirectory(parent)
}
