package storage

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAtomicCreatePrivateFileRefusesExistingAndHasOneConcurrentWinner(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "private")
	path := filepath.Join(dir, "signer.json")

	start := make(chan struct{})
	errs := make([]error, 8)
	var wait sync.WaitGroup
	for i := range errs {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			<-start
			errs[index] = AtomicCreatePrivateFile(path, []byte{byte('0' + index)})
		}(i)
	}
	close(start)
	wait.Wait()

	winners := 0
	for _, err := range errs {
		if err == nil {
			winners++
			continue
		}
		require.True(t, errors.Is(err, os.ErrExist), "unexpected loser error: %v", err)
	}
	require.Equal(t, 1, winners)

	before, found, err := ReadPrivateFile(path)
	require.NoError(t, err)
	require.True(t, found)
	require.ErrorIs(t, AtomicCreatePrivateFile(path, []byte("replacement")), os.ErrExist)
	after, found, err := ReadPrivateFile(path)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, before, after)
	assertPrivateMode(t, path)
}

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

func TestReadPrivateFileBoundedRejectsOversizeContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "private", "private.json")
	require.NoError(t, AtomicCreatePrivateFile(path, []byte("12345")))
	_, _, err := ReadPrivateFileBounded(path, 4)
	require.ErrorContains(t, err, "size limit")
	raw, found, err := ReadPrivateFileBounded(path, 5)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "12345", string(raw))
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
