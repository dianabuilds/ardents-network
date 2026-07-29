//go:build !windows

package storage

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAcquirePrivateFileLockRejectsAndDoesNotRepairExposedParent(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Chmod(dir, 0o777))

	_, err := AcquirePrivateFileLock(filepath.Join(dir, "rollout.lock"))
	require.Error(t, err)

	info, err := os.Stat(dir)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o777), info.Mode().Perm())
}

func TestAcquirePrivateFileLockRejectsSymlinkParent(t *testing.T) {
	target := t.TempDir()
	parent := filepath.Dir(target)
	link := filepath.Join(parent, "rollout-lock-link")
	require.NoError(t, os.Symlink(target, link))
	t.Cleanup(func() { _ = os.Remove(link) })

	_, err := AcquirePrivateFileLock(filepath.Join(link, "rollout.lock"))
	require.Error(t, err)
}
