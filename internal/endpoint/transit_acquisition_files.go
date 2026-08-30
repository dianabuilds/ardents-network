package endpoint

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func prepareTransitAcquisitionRoot(root string, create bool) error {
	info, err := os.Lstat(root)
	if errors.Is(err, os.ErrNotExist) && create {
		if err := os.MkdirAll(root, 0o700); err != nil {
			return err
		}
		info, err = os.Lstat(root)
	}
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("transit acquisition root is not an owned directory")
	}
	if err := secureTransitAcquisitionRoot(root, info); err != nil {
		return err
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || entry.Name() != "owner.lock" && entry.Name() != "root.marker" && entry.Name() != "current.json" {
			return fmt.Errorf("unknown transit acquisition root entry %q", entry.Name())
		}
	}
	markerPath := filepath.Join(root, "root.marker")
	marker, err := os.ReadFile(markerPath)
	if errors.Is(err, os.ErrNotExist) {
		if !create || len(entries) != 0 {
			return errors.New("transit acquisition root is not initialized")
		}
		return nil
	}
	if err != nil || !bytes.Equal(marker, []byte(transitAcquisitionMarker)) {
		return errors.New("transit acquisition root marker is invalid")
	}
	return nil
}

func initializeTransitAcquisitionRoot(root string, create bool) error {
	markerPath := filepath.Join(root, "root.marker")
	marker, err := os.ReadFile(markerPath)
	if err == nil {
		if !bytes.Equal(marker, []byte(transitAcquisitionMarker)) {
			return errors.New("transit acquisition root marker is invalid")
		}
		return nil
	}
	if !errors.Is(err, os.ErrNotExist) || !create {
		return errors.New("transit acquisition root is not initialized")
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 1 || entries[0].Name() != "owner.lock" || entries[0].IsDir() {
		return errors.New("refusing to claim a non-empty transit acquisition root")
	}
	file, err := os.OpenFile(markerPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	_, writeErr := file.WriteString(transitAcquisitionMarker)
	if writeErr == nil {
		writeErr = file.Sync()
	}
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		return errors.Join(writeErr, closeErr)
	}
	return endpointSyncDirectory(root)
}

func loadTransitAcquisitionState(root string) (transitAcquisitionState, error) {
	path := filepath.Join(root, "current.json")
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return transitAcquisitionState{}, nil
	}
	if err != nil {
		return transitAcquisitionState{}, err
	}
	raw, readErr := io.ReadAll(io.LimitReader(file, transitAcquisitionMaximum+1))
	closeErr := file.Close()
	if readErr != nil || closeErr != nil {
		return transitAcquisitionState{}, errors.Join(readErr, closeErr)
	}
	if int64(len(raw)) > transitAcquisitionMaximum {
		return transitAcquisitionState{}, errors.New("transit acquisition state exceeds its bound")
	}
	var state transitAcquisitionState
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&state); err != nil || !validTransitAcquisitionState(state) {
		return transitAcquisitionState{}, errors.New("transit acquisition state is invalid")
	}
	return state, nil
}

func replaceTransitAcquisitionState(root string, raw []byte) error {
	if len(raw) == 0 || int64(len(raw)) > transitAcquisitionMaximum {
		return errors.New("transit acquisition state exceeds its bound")
	}
	file, err := os.CreateTemp(root, ".current-")
	if err != nil {
		return err
	}
	path := file.Name()
	defer os.Remove(path)
	if err = file.Chmod(0o600); err == nil {
		_, err = file.Write(raw)
	}
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil || closeErr != nil {
		return errors.Join(err, closeErr)
	}
	if err := os.Rename(path, filepath.Join(root, "current.json")); err != nil {
		return err
	}
	return endpointSyncDirectory(root)
}
