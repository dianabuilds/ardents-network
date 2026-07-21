package provision

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEnsureTokenAcceptsInstalledApplicationPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not expose Unix group permission bits")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "application-token")
	require.NoError(t, os.WriteFile(path, []byte("existing-token"), 0o640))

	require.NoError(t, ensureToken(dir, "application-token", 0o027))
}

func TestEnsureTokenRejectsWritableApplicationToken(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not expose Unix group permission bits")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "application-token")
	require.NoError(t, os.WriteFile(path, []byte("existing-token"), 0o660))
	require.NoError(t, os.Chmod(path, 0o660))

	err := ensureToken(dir, "application-token", 0o027)
	require.ErrorContains(t, err, "permissions are invalid")
}
