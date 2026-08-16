package blockedverify

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

func finish(path string, result Result, cause error) (Result, error) {
	if path == "" {
		return result, errors.Join(cause, errors.New("verifier output path is empty"))
	}
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		return result, errors.Join(cause, errors.New("verifier output already exists or cannot be inspected"))
	}
	raw, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return result, errors.Join(cause, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return result, errors.Join(cause, err)
	}
	temporary := path + ".tmp"
	file, err := os.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return result, errors.Join(cause, err)
	}
	removeTemporary := true
	defer func() {
		if removeTemporary {
			_ = os.Remove(temporary)
		}
	}()
	if _, err := file.Write(append(raw, '\n')); err != nil {
		_ = file.Close()
		return result, errors.Join(cause, err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return result, errors.Join(cause, err)
	}
	if err := file.Close(); err != nil {
		return result, errors.Join(cause, err)
	}
	// Linking publishes without replacing a concurrently created verdict.
	if err := os.Link(temporary, path); err != nil {
		return result, errors.Join(cause, err)
	}
	if err := syncDirectory(filepath.Dir(path)); err != nil {
		return result, errors.Join(cause, err)
	}
	if err := os.Remove(temporary); err != nil {
		return result, errors.Join(cause, err)
	}
	if err := syncDirectory(filepath.Dir(path)); err != nil {
		return result, errors.Join(cause, err)
	}
	removeTemporary = false
	return result, cause
}
