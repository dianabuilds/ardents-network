package release

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const floorStoreLeaseContents = "ardents-release-decision-exclusive-lease-v1\n"

func acquireFloorStoreLease(root string) (*os.File, error) {
	path := filepath.Join(root, floorStoreLockName)
	lock, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return nil, errors.New("release: state root already has an active or unrecovered lease")
		}
		return nil, fmt.Errorf("release: acquire state-root lease: %w", err)
	}
	if _, err := lock.WriteString(floorStoreLeaseContents); err != nil {
		return nil, abandonFloorStoreLease(lock, path, fmt.Errorf("release: write state-root lease: %w", err))
	}
	if err := lock.Sync(); err != nil {
		return nil, abandonFloorStoreLease(lock, path, fmt.Errorf("release: sync state-root lease: %w", err))
	}
	return lock, nil
}

func abandonFloorStoreLease(lock *os.File, path string, cause error) error {
	closeErr := lock.Close()
	removeErr := os.Remove(path)
	if errors.Is(removeErr, os.ErrNotExist) {
		removeErr = nil
	}
	return errors.Join(cause, closeErr, removeErr)
}

func (store *floorStore) releaseLease() error {
	if store.lock == nil {
		return nil
	}
	closeErr := store.lock.Close()
	store.lock = nil
	removeErr := os.Remove(filepath.Join(store.path, floorStoreLockName))
	if closeErr != nil || (removeErr != nil && !errors.Is(removeErr, os.ErrNotExist)) {
		return errors.Join(closeErr, removeErr)
	}
	return syncDirectory(store.path)
}
