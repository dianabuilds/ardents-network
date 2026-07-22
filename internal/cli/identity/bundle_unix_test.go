//go:build !windows

package identity

import (
	"bytes"
	"crypto/ed25519"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSignerReadRefusesPermissiveExistingPrivateFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "identity", "principal.json")
	_, err := CreatePrincipal(path, bytes.NewReader(bytes.Repeat([]byte{0xa1}, ed25519.SeedSize)))
	require.NoError(t, err)
	require.NoError(t, os.Chmod(path, 0o644))

	_, err = ShowPrincipal(path)
	require.ErrorIs(t, err, ErrSignerFileUnsafe)
	require.NotContains(t, err.Error(), "root_private_seed")
}

func TestSignerReadRefusesPermissiveParentWithoutChangingIt(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "identity")
	path := filepath.Join(parent, "principal.json")
	_, err := CreatePrincipal(path, bytes.NewReader(bytes.Repeat([]byte{0xa2}, ed25519.SeedSize)))
	require.NoError(t, err)
	require.NoError(t, os.Chmod(parent, 0o755))

	_, err = ShowPrincipal(path)
	require.ErrorIs(t, err, ErrSignerFileUnsafe)
	info, statErr := os.Stat(parent)
	require.NoError(t, statErr)
	require.Equal(t, os.FileMode(0o755), info.Mode().Perm())
}
