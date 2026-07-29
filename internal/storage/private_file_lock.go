package storage

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.etcd.io/bbolt"
)

var ErrPrivateFileLockUnavailable = errors.New("private file lock unavailable")

// PrivateFileLock is an OS-visible exclusive lock scoped to one protected
// state file. The durable lock file is harmless; ownership is the lock held by
// the open bbolt handle and is released by the OS if the process exits.
type PrivateFileLock struct {
	db *bbolt.DB
}

func AcquirePrivateFileLock(path string) (*PrivateFileLock, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("private file lock path is required")
	}
	dir := filepath.Dir(path)
	if err := EnsurePrivateDir(dir); err != nil {
		return nil, fmt.Errorf("prepare private file lock directory: %w", err)
	}
	if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
		if createErr := AtomicCreatePrivateFile(path, nil); createErr != nil {
			if validateErr := ValidateStrictPrivateFile(path); validateErr != nil {
				return nil, fmt.Errorf("create private file lock: %w", createErr)
			}
		}
	} else if err != nil {
		return nil, fmt.Errorf("inspect private file lock: %w", err)
	}
	if err := ValidateStrictPrivateFile(path); err != nil {
		return nil, fmt.Errorf("validate private file lock: %w", err)
	}
	db, err := bbolt.Open(path, 0o600, &bbolt.Options{Timeout: time.Millisecond})
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrPrivateFileLockUnavailable, err)
	}
	if err := ProtectPrivateFile(path); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("protect private file lock: %w", err)
	}
	return &PrivateFileLock{db: db}, nil
}

func (lock *PrivateFileLock) Close() error {
	if lock == nil || lock.db == nil {
		return nil
	}
	err := lock.db.Close()
	lock.db = nil
	if err != nil && !errors.Is(err, bbolt.ErrDatabaseNotOpen) {
		return fmt.Errorf("release private file lock: %w", err)
	}
	return nil
}
