package daemon

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNodeRuntimePreflightBeforeStartupDoesNotOverwriteDurableDiagnostics(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "operations.json")
	const persisted = `{"operations":[{"id":"op-1","kind":"node.startup.workloads","state":"running","domain":"workload","resource":"workloads","recoverable":true,"recovery_action":"restart node","started_at":"2026-03-20T10:00:00Z","updated_at":"2026-03-20T10:00:00Z"}]}`
	require.NoError(t, os.WriteFile(path, []byte(persisted), 0o600))

	node := NewNode(Config{
		Name: "preflight",
		Boot: BootConfig{Sources: []string{"local://bootstrap"}},
		Data: DataConfig{Dir: dir},
	})
	snapshot := node.GetNodeRuntime()
	require.Equal(t, "stopped", snapshot.Node.State)

	got, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, persisted, string(got))
}
