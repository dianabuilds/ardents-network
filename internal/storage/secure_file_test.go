package storage

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAtomicWritePrivateFileReplacesCompleteContent(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "private")
	path := filepath.Join(dir, "state.json")

	require.NoError(t, AtomicWritePrivateFile(path, []byte("first")))
	require.NoError(t, AtomicWritePrivateFile(path, []byte("second")))
	raw, found, err := ReadPrivateFile(path)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "second", string(raw))

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	for _, entry := range entries {
		require.False(t, strings.HasPrefix(entry.Name(), ".ardents-private-"))
	}
	assertPrivateMode(t, path)
}

func TestReadPrivateFileRejectsNonRegularState(t *testing.T) {
	_, _, err := ReadPrivateFile(t.TempDir())
	require.ErrorContains(t, err, "regular file")
}

func TestReadProtectedFileUpgradesRetainedDataPermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ledger.json")
	require.NoError(t, os.WriteFile(path, []byte("{}"), 0o644))

	raw, found, err := ReadProtectedFile(path)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "{}", string(raw))
	assertPrivateMode(t, path)
}

func assertPrivateMode(t *testing.T, path string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		return
	}
	info, err := os.Stat(path)
	require.NoError(t, err)
	require.Zero(t, info.Mode().Perm()&0o077)
}
