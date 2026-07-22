package storage

import (
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"go.etcd.io/bbolt"
)

const stateLockFileName = ".ardents-state.lock"

// StateDirLock is the process-wide ownership proof shared by the daemon and
// offline state tools. The lock file is durable; ownership is the OS lock held
// by the open bbolt handle, not the file's mere existence.
type StateDirLock struct {
	db *bbolt.DB
}

func AcquireStateDirLock(dir string) (*StateDirLock, error) {
	if dir == "" {
		return nil, fmt.Errorf("state directory is required")
	}
	if err := EnsurePrivateDir(dir); err != nil {
		return nil, fmt.Errorf("prepare state directory lock: %w", err)
	}
	path := filepath.Join(dir, stateLockFileName)
	db, err := bbolt.Open(path, 0o600, &bbolt.Options{Timeout: time.Millisecond})
	if err != nil {
		return nil, fmt.Errorf("state directory is in use or unavailable")
	}
	if err := ProtectPrivateFile(path); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("protect state directory lock: %w", err)
	}
	return &StateDirLock{db: db}, nil
}

func (l *StateDirLock) Close() error {
	if l == nil || l.db == nil {
		return nil
	}
	err := l.db.Close()
	l.db = nil
	if err != nil && !errors.Is(err, bbolt.ErrDatabaseNotOpen) {
		return fmt.Errorf("release state directory lock: %w", err)
	}
	return nil
}
