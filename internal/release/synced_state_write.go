package release

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func writeSyncedFile(path string, contents []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("release: create %s: %w", filepath.Base(path), err)
	}
	written, err := file.Write(contents)
	if err == nil && written != len(contents) {
		err = errors.New("short write")
	}
	if err != nil {
		return errors.Join(fmt.Errorf("release: write %s: %w", filepath.Base(path), err), file.Close())
	}
	syncErr := file.Sync()
	closeErr := file.Close()
	if syncErr != nil {
		return fmt.Errorf("release: sync %s: %w", filepath.Base(path), syncErr)
	}
	if closeErr != nil {
		return fmt.Errorf("release: close %s: %w", filepath.Base(path), closeErr)
	}
	return nil
}

func writeFloorStoreMarker(root string) error {
	markerPath := filepath.Join(root, floorStoreMarkerName)
	if err := requireRegularFile(markerPath); err == nil {
		contents, readErr := readBoundedFloorFile(markerPath, int64(len(floorStoreMarker)))
		if readErr != nil || string(contents) != floorStoreMarker {
			return errors.New("release: state root marker is invalid")
		}
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("release: inspect state root marker: %w", err)
	}
	entries, err := readFloorStoreDirectory(root, 2)
	if err != nil {
		return fmt.Errorf("release: inspect state root: %w", err)
	}
	if len(entries) > 1 || len(entries) == 1 && entries[0].Name() != floorStoreLockName {
		return errors.New("release: refusing to claim a non-empty unowned state root")
	}
	if err := writeSyncedFile(markerPath, []byte(floorStoreMarker)); err != nil {
		return err
	}
	return syncDirectory(root)
}
