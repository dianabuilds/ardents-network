package daemon

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNodeFailLockedKeepsDiagnosticsHealthFailed(t *testing.T) {
	n := NewNode(Config{
		Name: "failed-health",
		Boot: BootConfig{Sources: []string{"local://bootstrap"}},
		Data: DataConfig{Dir: t.TempDir()},
	})

	n.mu.Lock()
	n.runtimeMgr.FailLocked("node.transport.start_failed", "transport", "transport start failed", "boom", "node startup could not complete", "restart_required")
	snapshot := n.queryService.DiagnosticsSnapshotLocked()
	n.mu.Unlock()

	require.Equal(t, "failed", snapshot.Health.State)
}
