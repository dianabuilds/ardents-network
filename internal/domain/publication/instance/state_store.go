package instance

import (
	"context"
	"errors"
	"os"
	"path/filepath"
)

// inspectLayout classifies existing bytes without creating a lease or repairing
// missing state. A complete retained root is rechecked under its actual lease.
func inspectLayout(path string) (bool, error) {
	entries, err := os.ReadDir(path)
	if err != nil || len(entries) > 3 {
		return false, ErrInvalid
	}
	if len(entries) == 0 {
		return true, nil
	}
	if len(entries) != 3 {
		return false, ErrInvalid
	}
	for _, entry := range entries {
		if !entry.Type().IsRegular() {
			return false, ErrInvalid
		}
		switch entry.Name() {
		case markerName, lockName, stateName:
		default:
			return false, ErrInvalid
		}
	}
	raw, err := readPrivate(filepath.Join(path, markerName), int64(len(marker)))
	if err != nil || string(raw) != marker {
		return false, ErrInvalid
	}
	return false, nil
}

func (root *Root) prepareLayout(fresh bool) error {
	if fresh {
		entries, err := os.ReadDir(root.path)
		if err != nil || len(entries) != 1 || entries[0].Name() != lockName {
			return ErrInvalid
		}
		if err := writeExclusive(filepath.Join(root.path, markerName), []byte(marker)); err != nil {
			return err
		}
	} else if empty, err := inspectLayout(root.path); err != nil || empty {
		return ErrInvalid
	}
	raw, err := readPrivate(filepath.Join(root.path, markerName), int64(len(marker)))
	if err != nil || string(raw) != marker {
		return ErrInvalid
	}
	file, err := os.OpenFile(filepath.Join(root.path, markerName), os.O_RDWR, 0)
	if err != nil {
		return err
	}
	if err = errors.Join(file.Sync(), file.Close()); err != nil {
		return err
	}
	if err = root.flush(root.path); err != nil {
		return err
	}
	return root.flush(filepath.Dir(root.path))
}

func (root *Root) writeState(ctx context.Context, next generationState) (result error) {
	if err := root.check(ctx); err != nil {
		return err
	}
	raw, err := marshalState(next)
	if err != nil {
		return err
	}
	defer clear(raw)
	defer func() {
		if result != nil {
			root.failure = errors.Join(root.failure, result)
			root.state.redact()
		}
	}()
	file, err := os.CreateTemp(root.path, ".instance-root-staging-")
	if err != nil {
		return err
	}
	path := file.Name()
	defer func() {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			result = errors.Join(result, err)
		}
	}()
	if err = file.Chmod(0600); err == nil {
		_, err = file.Write(raw)
	}
	if err == nil {
		err = file.Sync()
	}
	if err = errors.Join(err, file.Close()); err != nil {
		return err
	}
	if err = root.check(ctx); err != nil {
		return err
	}
	if err = os.Rename(path, filepath.Join(root.path, stateName)); err != nil {
		return err
	}
	if err = root.flush(root.path); err != nil {
		return err
	}
	return root.check(ctx)
}

func writeExclusive(path string, raw []byte) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, err = file.Write(raw)
	if err == nil {
		err = file.Sync()
	}
	return errors.Join(err, file.Close())
}
