package networkstate

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	rootMarkerName = ".ardents-network-state-v1"
	rootMarker     = "ardents-network-state-v1\n"
	rootLockName   = ".ardents-network-state-lock"
)

func inspectRoot(root string) error {
	info, err := os.Lstat(root)
	if os.IsNotExist(err) {
		if err := os.MkdirAll(root, 0o700); err != nil {
			return fmt.Errorf("create state root: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("inspect state root: %w", err)
	} else if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("state root is not an owned directory")
	}
	markerInfo, markerErr := os.Lstat(filepath.Join(root, rootMarkerName))
	if markerErr == nil {
		if !markerInfo.Mode().IsRegular() || markerInfo.Mode()&os.ModeSymlink != 0 {
			return errors.New("state root ownership marker is not a regular file")
		}
		return nil
	}
	if !os.IsNotExist(markerErr) {
		return fmt.Errorf("inspect state root ownership marker: %w", markerErr)
	}
	entries, readErr := readBoundedDirectory(root, 2)
	if readErr != nil || len(entries) > 1 || len(entries) == 1 && entries[0].Name() != rootLockName {
		return errors.New("refusing to claim a non-empty unowned state root")
	}
	return nil
}

func prepareRoot(root string) error {
	if err := ensureRootMarker(root); err != nil {
		return err
	}
	generations := filepath.Join(root, "generations")
	if err := os.MkdirAll(generations, 0o700); err != nil {
		return fmt.Errorf("create generations directory: %w", err)
	}
	if err := cleanupOwnedStaging(root, generations); err != nil {
		return err
	}
	return syncDirectory(root)
}

func ensureRootMarker(root string) error {
	markerPath := filepath.Join(root, rootMarkerName)
	info, err := os.Lstat(markerPath)
	if err == nil {
		if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("state root ownership marker is not a regular file")
		}
		contents, readErr := readBoundedFile(markerPath, int64(len(rootMarker)))
		if readErr != nil || !bytes.Equal(contents, []byte(rootMarker)) {
			return errors.New("state root ownership marker is invalid")
		}
		return nil
	}
	if !os.IsNotExist(err) {
		return fmt.Errorf("inspect state root ownership marker: %w", err)
	}
	entries, readErr := readBoundedDirectory(root, 2)
	if readErr != nil || len(entries) != 1 || entries[0].Name() != rootLockName {
		return errors.New("refusing to claim a non-empty unowned state root")
	}
	if err := writeSynced(markerPath, []byte(rootMarker)); err != nil {
		return fmt.Errorf("create state root ownership marker: %w", err)
	}
	return syncDirectory(root)
}

func cleanupOwnedStaging(root, generations string) error {
	for directory, prefix := range map[string]string{root: ".current-", generations: ".stage-"} {
		entries, err := readBoundedDirectory(directory, 128)
		if err != nil {
			return fmt.Errorf("scan owned state root: %w", err)
		}
		for _, entry := range entries {
			if !strings.HasPrefix(entry.Name(), prefix) {
				continue
			}
			if err := os.RemoveAll(filepath.Join(directory, entry.Name())); err != nil {
				return fmt.Errorf("remove interrupted owned state %q: %w", entry.Name(), err)
			}
		}
	}
	return nil
}
