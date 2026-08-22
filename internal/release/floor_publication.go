package release

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func publishFloorGeneration(generations, name string, payload []byte, roots []rootArchiveEntry) error {
	staging, err := os.MkdirTemp(generations, ".stage-")
	if err != nil {
		return fmt.Errorf("release: create generation staging: %w", err)
	}
	if err := writeSyncedFile(filepath.Join(staging, "state.bin"), payload); err != nil {
		return cleanupStagedGeneration(staging, err)
	}
	if err := writeRootArchive(staging, roots); err != nil {
		return cleanupStagedGeneration(staging, err)
	}
	if err := syncDirectory(staging); err != nil {
		return cleanupStagedGeneration(staging, fmt.Errorf("release: sync staged generation: %w", err))
	}
	final := filepath.Join(generations, name)
	if err := durableRename(staging, final); err != nil {
		return cleanupStagedGeneration(staging, fmt.Errorf("release: publish generation: %w", err))
	}
	if err := syncDirectory(generations); err != nil {
		return fmt.Errorf("release: sync generations directory: %w", err)
	}
	return nil
}

func cleanupStagedGeneration(path string, cause error) error {
	return errors.Join(cause, os.RemoveAll(path))
}

func writeFloorStorePointer(root, name string) (resultErr error) {
	temporary, err := os.CreateTemp(root, ".current-")
	if err != nil {
		return fmt.Errorf("release: create current pointer staging: %w", err)
	}
	path := temporary.Name()
	defer func() {
		removeErr := os.Remove(path)
		if errors.Is(removeErr, os.ErrNotExist) {
			removeErr = nil
		}
		resultErr = errors.Join(resultErr, removeErr)
	}()
	if err = temporary.Chmod(0o600); err == nil {
		_, err = temporary.WriteString(name + "\n")
	}
	if err == nil {
		err = temporary.Sync()
	}
	closeErr := temporary.Close()
	if err != nil {
		return fmt.Errorf("release: write current pointer: %w", err)
	}
	if closeErr != nil {
		return fmt.Errorf("release: close current pointer: %w", closeErr)
	}
	if err := durableRename(path, filepath.Join(root, "current")); err != nil {
		return fmt.Errorf("release: replace current pointer: %w", err)
	}
	if err := syncDirectory(root); err != nil {
		return fmt.Errorf("release: sync state-root pointer: %w", err)
	}
	return nil
}

func floorGenerationID(payload []byte, roots []rootArchiveEntry) string {
	hash := sha256.New()
	_, _ = hash.Write(payload)
	for _, root := range roots {
		_, _ = hash.Write(root.bytes)
	}
	return hex.EncodeToString(hash.Sum(nil))
}
