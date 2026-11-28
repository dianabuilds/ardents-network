package publisher

import (
	"context"
	"errors"
	"io"
	"os"
	"sync/atomic"
	"syscall"

	"github.com/dianabuilds/ardents-network/internal/application/document"
	"github.com/dianabuilds/ardents-network/internal/platform/artifacts"
)

// SnapshotStorage retains the installed Endpoint's checked private directory.
type SnapshotStorage struct {
	root  *artifacts.DirectoryChain
	quota int64
	busy  atomic.Bool
}

// OpenSnapshotStorage borrows the original Endpoint lifetime. Close after
// Administration has stopped accepting work and joined its requests.
func OpenSnapshotStorage(ctx context.Context, path string, quota int64) (*SnapshotStorage, error) {
	if ctx == nil || quota <= 0 || path == "/" {
		return nil, errSnapshotStage
	}
	uid := uint32(os.Geteuid())
	root, err := artifacts.OpenDirectoryChain(ctx, path, &uid)
	if err != nil {
		return nil, err
	}
	info, err := root.Descriptor().Stat()
	if err != nil {
		return nil, errors.Join(err, root.Close())
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != uid || info.Mode().Perm() != 0o700 {
		return nil, errors.Join(errSnapshotStage, root.Close())
	}
	return &SnapshotStorage{root: root, quota: quota}, nil
}

func (storage *SnapshotStorage) Close() error { return storage.root.Close() }

// StageSnapshot reserves the declared size before opening the provisional
// input. The reservation is released only after the file descriptors close.
func (owner *Publisher) StageSnapshot(ctx context.Context, storage *SnapshotStorage, size int64, receive func(io.Writer) error) (result error) {
	if owner == nil || owner.boundary == nil || ctx == nil || storage == nil || receive == nil || size < 0 || size > storage.quota || size > document.MaximumBytes {
		return errSnapshotStage
	}
	if err := errors.Join(ctx.Err(), owner.ctx.Err(), owner.boundary()); err != nil {
		return err
	}
	if !storage.busy.CompareAndSwap(false, true) {
		return errSnapshotStage
	}
	var cleanupErr error
	canRelease := true
	defer func() {
		if canRelease && cleanupErr == nil {
			storage.busy.Store(false)
		}
		result = errors.Join(result, cleanupErr)
	}()
	if err := storage.root.CheckOriginalAccess(); err != nil {
		return err
	}
	stage, err := openSnapshotStage(storage.root.Descriptor(), size)
	if err != nil {
		return err
	}
	defer func() { cleanupErr = errors.Join(cleanupErr, stage.Close()) }()
	if err := receive(stage); err != nil {
		return err
	}
	if err := errors.Join(ctx.Err(), storage.root.CheckOriginalAccess()); err != nil {
		return err
	}
	source, err := stage.finish()
	if err != nil {
		// A failed descriptor handoff can leave cleanup uncertain.
		canRelease = false
		return err
	}
	defer func() { cleanupErr = errors.Join(cleanupErr, source.Close()) }()

	// The existing worker initialization consumes a bounded slice. The
	// Administration ceiling applies until the streaming worker is wired.
	body := make([]byte, int(size))
	defer clear(body)
	if _, err := io.ReadFull(source, body); err != nil {
		return err
	}
	return owner.PublishSnapshot(ctx, body)
}
