package instance

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func admittedRoot(path string) (string, error) {
	root, err := filepath.Abs(path)
	if err != nil {
		return "", ErrInvalid
	}
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", ErrInvalid
	}
	if err := validateRootAccess(root, info); err != nil {
		return "", err
	}
	return root, nil
}

func prepareRoot(root string) error {
	markerPath := filepath.Join(root, markerName)
	if raw, err := readBounded(markerPath, 128); err == nil {
		if !bytes.Equal(raw, []byte(marker)) {
			return ErrInvalid
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return ErrInvalid
	} else if err := writeExclusive(markerPath, []byte(marker)); err != nil {
		return fmt.Errorf("create Service Instance marker: %w", err)
	}
	lockPath := filepath.Join(root, lockName)
	if file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600); err == nil {
		if closeErr := file.Close(); closeErr != nil {
			return closeErr
		}
	} else if !errors.Is(err, os.ErrExist) {
		return fmt.Errorf("create Service Instance lock: %w", err)
	}
	return syncDirectory(root)
}

func validateRootEntries(root string, hasState bool) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		return ErrInvalid
	}
	want := map[string]bool{markerName: false, lockName: false}
	if hasState {
		want[stateName] = false
	}
	if len(entries) != len(want) {
		return ErrInvalid
	}
	for _, entry := range entries {
		if _, ok := want[entry.Name()]; !ok || entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			return ErrInvalid
		}
		want[entry.Name()] = true
	}
	for _, found := range want {
		if !found {
			return ErrInvalid
		}
	}
	return nil
}

func validateMarker(root string) error {
	raw, err := readBounded(filepath.Join(root, markerName), 128)
	if err != nil || !bytes.Equal(raw, []byte(marker)) {
		return ErrInvalid
	}
	return nil
}

func readState(root string) (durableState, error) {
	raw, err := readBounded(filepath.Join(root, stateName), 4096)
	if err != nil {
		return durableState{}, err
	}
	return unmarshalState(raw)
}

func writeState(root string, state durableState) error {
	raw, err := marshalState(state)
	if err != nil {
		return err
	}
	temporary, err := os.CreateTemp(root, ".instance-root-staging-")
	if err != nil {
		return fmt.Errorf("create Service Instance staging file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(raw); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := durableRename(temporaryPath, filepath.Join(root, stateName)); err != nil {
		return err
	}
	return syncDirectory(root)
}

func writeExclusive(path string, raw []byte) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := file.Write(raw); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func readBounded(path string, limit int64) ([]byte, error) {
	pathInfo, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !pathInfo.Mode().IsRegular() || pathInfo.Mode()&os.ModeSymlink != 0 ||
		pathInfo.Size() <= 0 || pathInfo.Size() > limit {
		return nil, ErrInvalid
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > limit || !os.SameFile(pathInfo, info) {
		_ = file.Close()
		return nil, ErrInvalid
	}
	raw, readErr := io.ReadAll(io.LimitReader(file, limit+1))
	closeErr := file.Close()
	if readErr != nil || closeErr != nil || int64(len(raw)) > limit {
		return nil, errors.Join(readErr, closeErr, ErrInvalid)
	}
	return raw, nil
}
