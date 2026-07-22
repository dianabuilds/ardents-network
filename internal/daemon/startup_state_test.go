package daemon

import (
	"encoding/json"
	"path/filepath"
	"testing"

	runtimeconfig "ardents/internal/config"
	"ardents/internal/storage"
	"github.com/stretchr/testify/require"
)

func TestStartupStateDirectoryUsesPureOperatorConfigDecode(t *testing.T) {
	dir := t.TempDir()
	stateDir := filepath.Join(dir, "state")
	doc := runtimeconfig.Defaults()
	doc.Node.DataDir = stateDir
	raw, err := json.Marshal(doc)
	require.NoError(t, err)
	path := filepath.Join(dir, "operator.json")
	require.NoError(t, storage.AtomicWritePrivateFile(path, raw))
	t.Setenv(runtimeconfig.OperatorFileEnv, path)

	got, err := startupStateDirectory()
	require.NoError(t, err)
	require.Equal(t, stateDir, got)
}
