package participation

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMessageProviderDSNCreatesPrivateRegularStore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "waku", "store.db")
	dsn, err := MessageProviderDSN(path)
	require.NoError(t, err)
	require.Equal(t, path, dsn)

	info, err := os.Lstat(path)
	require.NoError(t, err)
	require.True(t, info.Mode().IsRegular())
	if runtime.GOOS != "windows" {
		require.Zero(t, info.Mode().Perm()&0o077)
	}
	exists, err := MessageProviderExists(path)
	require.NoError(t, err)
	require.True(t, exists)
}

func TestMessageProviderExistsRejectsDirectory(t *testing.T) {
	_, err := MessageProviderExists(t.TempDir())
	require.ErrorContains(t, err, "regular file")
}
