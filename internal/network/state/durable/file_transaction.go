package durable

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

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

// ErrPointerSyncUncertain means the pointer rename succeeded but syncing its
// directory failed. The new pointer can already be visible; callers must not
// continue from an in-memory predecessor; reopen must verify the floor.
var ErrPointerSyncUncertain = errors.New("current pointer durability is uncertain after rename")

func replacePointer(root, name, generation string) error {
	return replacePointerWithSync(root, name, generation, syncDirectory)
}

// The final sync is passed explicitly so the post-rename failure boundary can
// be exercised without depending on filesystem permissions or timing.
func replacePointerWithSync(root, name, generation string, sync func(string) error) error {
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
	if err := sync(root); err != nil {
		return fmt.Errorf("sync current pointer: %w: %w", ErrPointerSyncUncertain, err)
	}
	return nil
}
