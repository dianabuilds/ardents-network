//go:build linux && (document_worker_installed || endpoint_launch_native)

package endpoint

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// WriteStartupFile creates and syncs a fixture input under its declared name.
func WriteStartupFile(t testing.TB, path string, raw []byte, mode os.FileMode) {
	t.Helper()
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		t.Fatal(err)
	}
	n, writeErr := file.Write(raw)
	modeErr := file.Chmod(mode)
	syncErr := file.Sync()
	closeErr := file.Close()
	if n != len(raw) || writeErr != nil || modeErr != nil || syncErr != nil || closeErr != nil {
		t.Fatal("durable fixture write", writeErr, modeErr, syncErr, closeErr)
	}
	SyncStartupPath(t, filepath.Dir(path))
}

// SyncStartupPath flushes a fixture directory after startup input publication.
func SyncStartupPath(t testing.TB, path string) {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	syncErr, closeErr := file.Sync(), file.Close()
	if syncErr != nil || closeErr != nil {
		t.Fatal(syncErr, closeErr)
	}
}

// WriteLaunchFile creates parent directories before writing a fixture input.
func WriteLaunchFile(t testing.TB, path string, raw []byte, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	WriteStartupFile(t, path, raw, mode)
}

// PublishPermissionResponse atomically exposes the synced response without
// overwriting a prior response at the original participant's handover path.
func PublishPermissionResponse(t testing.TB, path string, response []byte, uid, gid uint32) (resultErr error) {
	t.Helper()
	pending := path + ".pending"
	WriteLaunchFile(t, pending, response, 0600)
	defer func() {
		if err := os.Remove(pending); err != nil && !os.IsNotExist(err) {
			resultErr = errors.Join(resultErr, fmt.Errorf("remove pending permission response: %w", err))
		}
		SyncStartupPath(t, filepath.Dir(path))
	}()
	if err := os.Chown(pending, int(uid), int(gid)); err != nil {
		return err
	}
	SyncStartupPath(t, pending)
	if err := os.Link(pending, path); err != nil {
		return fmt.Errorf("publish original permission response without replacement: %w", err)
	}
	return nil
}
