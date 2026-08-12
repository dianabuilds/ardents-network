package store

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

var generationName = regexp.MustCompile(`^[0-9a-f]{64}$`)

func writeSynced(path string, contents []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create immutable state file: %w", err)
	}
	if _, err = file.Write(contents); err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return fmt.Errorf("write immutable state file: %w", err)
	}
	if closeErr != nil {
		return fmt.Errorf("close immutable state file: %w", closeErr)
	}
	return nil
}

func replacePointer(root, name, generation string) error {
	temporary, err := os.CreateTemp(root, ".current-")
	if err != nil {
		return fmt.Errorf("create current pointer staging: %w", err)
	}
	temporaryPath := temporary.Name()
	defer func() { _ = os.Remove(temporaryPath) }()
	if err = temporary.Chmod(0o600); err == nil {
		_, err = temporary.WriteString(generation + "\n")
	}
	if err == nil {
		err = temporary.Sync()
	}
	closeErr := temporary.Close()
	if err != nil {
		return fmt.Errorf("write current pointer staging: %w", err)
	}
	if closeErr != nil {
		return fmt.Errorf("close current pointer staging: %w", closeErr)
	}
	if err := os.Rename(temporaryPath, filepath.Join(root, name)); err != nil {
		return fmt.Errorf("replace current pointer: %w", err)
	}
	if err := syncDirectory(root); err != nil {
		return fmt.Errorf("sync current pointer: %w", err)
	}
	return nil
}
