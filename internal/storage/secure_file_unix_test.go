//go:build !windows

package storage

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAtomicCreatePrivateFileRefusesAndDoesNotRewritePermissiveParent(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "shared")
	require.NoError(t, os.Mkdir(parent, 0o755))
	require.NoError(t, os.Chmod(parent, 0o755))
	path := filepath.Join(parent, "signer.json")
	original, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(parent))
	t.Cleanup(func() { require.NoError(t, os.Chdir(original)) })

	err = AtomicCreatePrivateFile("signer.json", []byte("secret"))
	require.ErrorContains(t, err, "directory permissions")
	info, statErr := os.Stat(parent)
	require.NoError(t, statErr)
	require.Equal(t, os.FileMode(0o755), info.Mode().Perm())
	_, statErr = os.Stat(path)
	require.ErrorIs(t, statErr, os.ErrNotExist)
}

func TestAtomicCreatePrivateFileAcceptsExistingPrivateParent(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "private")
	require.NoError(t, os.Mkdir(parent, 0o700))
	require.NoError(t, os.Chmod(parent, 0o700))
	require.NoError(t, AtomicCreatePrivateFile(filepath.Join(parent, "signer.json"), []byte("secret")))
}
