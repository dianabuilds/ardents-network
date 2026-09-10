package reachability

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Provision only the directory and stable lock before taking the lease.
// The accepted marker and records are checked/created under that lease.
func prepareStoreRoot(root string) error {
	if err := createStoreDirectory(root); err != nil {
		return err
	}
	lock := filepath.Join(root, storeLockName)
	if _, err := os.Lstat(filepath.Join(root, storeMarkerName)); err == nil {
		info, err := os.Lstat(lock)
		if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("initialized reachability store lock is missing or invalid")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := ensureStoreFile(lock, nil); err != nil {
		return err
	}
	return syncStoreDirectory(root)
}

// Sync every newly created directory link, including a pre-existing root's
// link, before its Store can acknowledge a publication.
func createStoreDirectory(root string) error {
	info, err := os.Lstat(root)
	if errors.Is(err, os.ErrNotExist) {
		parent := filepath.Dir(root)
		if parent == root {
			return errors.New("reachability root has no existing parent")
		}
		if err := createStoreDirectory(parent); err != nil {
			return err
		}
		if err := os.Mkdir(root, 0700); err != nil {
			return fmt.Errorf("create reachability store directory: %w", err)
		}
	} else if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("reachability store directory is invalid")
	}
	return syncStoreDirectory(filepath.Dir(root))
}

func initializeStoreRoot(root string) error {
	marker := filepath.Join(root, storeMarkerName)
	records := filepath.Join(root, storeRecords)
	if _, err := os.Lstat(marker); err == nil {
		if err := ensureStoreFile(marker, []byte(storeMarker)); err != nil {
			return err
		}
		info, err := os.Lstat(records)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("initialized reachability records are missing or invalid")
		}
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.Name() == storeLockName {
			continue
		}
		if entry.Name() != storeRecords || !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			return errors.New("unmarked reachability root is not empty")
		}
		contents, err := os.ReadDir(records)
		if err != nil || len(contents) != 0 {
			return errors.New("unmarked reachability root contains retained state")
		}
	}
	if err := os.Mkdir(records, 0700); err != nil && !errors.Is(err, os.ErrExist) {
		return err
	}
	if err := syncStoreDirectory(records); err != nil {
		return err
	}
	if err := syncStoreDirectory(root); err != nil {
		return err
	}
	// The marker follows durable records creation, so a missing records directory
	// in a marked root is damage, never permission to reset the generation floor.
	if err := ensureStoreFile(marker, []byte(storeMarker)); err != nil {
		return err
	}
	return syncStoreDirectory(root)
}
func ensureStoreFile(path string, expected []byte) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		file, createErr := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if createErr != nil {
			return createErr
		}
		if len(expected) > 0 {
			_, createErr = file.Write(expected)
		}
		if createErr == nil {
			createErr = file.Sync()
		}
		return errors.Join(createErr, file.Close())
	}
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("reachability store root file is invalid")
	}
	if len(expected) == 0 {
		return nil
	}
	raw, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(raw, expected) {
		return errors.New("reachability store marker is invalid")
	}
	return nil
}
