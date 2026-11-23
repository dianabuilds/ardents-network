package durablefile

import (
	"os"
	"path/filepath"
)

// ReplaceFile durably replaces one owned file through a same-directory staging
// file: create, owner-only chmod, write, sync, close, rename, then directory
// sync. The caller retains its own value validation and transition order; the
// staging file is removed on any failure before the rename.
func ReplaceFile(root, temporaryPrefix, finalName string, raw []byte) error {
	temporary, err := os.CreateTemp(root, temporaryPrefix)
	if err != nil {
		return err
	}
	path := temporary.Name()
	defer func() { _ = os.Remove(path) }()
	if err = temporary.Chmod(0o600); err == nil {
		_, err = temporary.Write(raw)
	}
	if err == nil {
		err = temporary.Sync()
	}
	closeErr := temporary.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err := os.Rename(path, filepath.Join(root, finalName)); err != nil {
		return err
	}
	return SyncDirectory(root)
}
