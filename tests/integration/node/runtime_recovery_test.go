//go:build integration

package node_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	runtimeinfra "ardents/internal/daemon"
	persistence "ardents/internal/diagnostics"
	operations "ardents/internal/diagnostics/operation"
	"ardents/tests/testkit"

	"github.com/stretchr/testify/require"
)

func TestNodeRuntimeRecoveryShowsPendingOperationAfterRestart(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "node",
		ScenarioID:  "NRI-001",
		Suite:       "integration",
		Tags:        []string{"integration", "node"},
		Speed:       "default",
		Environment: "local",
	})
	dir := t.TempDir()
	raw := []byte(`{"operations":[{"id":"op-1","kind":"node.startup.workloads","state":"running","domain":"workload","resource":"workloads","recoverable":true,"recovery_action":"restart node","started_at":"2026-03-20T10:00:00Z","updated_at":"2026-03-20T10:00:00Z"}]}`)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "operations.json"), raw, 0o644))

	n := testkit.StartNode(t, runtimeinfra.Config{
		Name: "runtime-recovery",
		Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.DataConfig{Dir: dir},
	})

	pending := testkit.Diagnostics(n).PendingOperations()
	require.Len(t, pending, 1)
	require.Equal(t, "recovering", pending[0].State)
}

func TestNodeRuntimeShutdownPersistsCompletedOperation(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "node",
		ScenarioID:  "NRI-001",
		Suite:       "integration",
		Tags:        []string{"integration", "node"},
		Speed:       "default",
		Environment: "local",
	})
	dir := t.TempDir()
	n := testkit.StartNode(t, runtimeinfra.Config{
		Name: "runtime-shutdown",
		Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.DataConfig{Dir: dir},
	})
	require.NoError(t, n.Stop(context.Background()))

	raw, err := os.ReadFile(filepath.Join(dir, "operations.json"))
	require.NoError(t, err)
	text := string(raw)
	require.Contains(t, text, `"kind": "node.shutdown"`)
	require.Contains(t, text, `"state": "completed"`)
}

func TestNodeRuntimeStartupFailureRemainsExplainable(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "node",
		ScenarioID:  "NRI-001",
		Suite:       "integration",
		Tags:        []string{"integration", "node"},
		Speed:       "default",
		Environment: "local",
	})
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ardents.db"), []byte("{invalid"), 0o644))

	n := testkit.NewRuntime(t, runtimeinfra.Config{
		Name: "runtime-startup-failure",
		Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.DataConfig{Dir: dir},
	}).Node
	err := n.Start(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "node start failed")

	snapshot := testkit.Diagnostics(n).DiagnosticsSnapshot()
	require.NotNil(t, snapshot.Health.PrimaryReason)
	require.Equal(t, "node.state.load_failed", snapshot.Health.PrimaryReason.Code)
}

func TestNodeRuntimeRestartCompactsClosedOperationsLedger(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "node",
		ScenarioID:  "NRI-001",
		Suite:       "integration",
		Tags:        []string{"integration", "node"},
		Speed:       "default",
		Environment: "local",
	})
	dir := t.TempDir()
	start := time.Date(2026, 3, 22, 10, 0, 0, 0, time.UTC)
	ledger := persistence.Ledger{Operations: make([]operations.Record, 0, 40)}
	for i := 0; i < 40; i++ {
		started := start.Add(time.Duration(i) * time.Minute)
		finished := started.Add(30 * time.Second)
		ledger.Operations = append(ledger.Operations, operations.Record{
			ID:         "closed-" + started.Format("150405"),
			Kind:       "node.shutdown",
			State:      operations.Completed,
			Domain:     "node",
			Resource:   "node",
			StartedAt:  started,
			UpdatedAt:  finished,
			FinishedAt: &finished,
		})
	}
	openStarted := start.Add(24 * time.Hour)
	ledger.Operations = append(ledger.Operations, operations.Record{
		ID:             "open-running",
		Kind:           "node.startup.workloads",
		State:          operations.Running,
		Domain:         "workload",
		Resource:       "workloads",
		Recoverable:    true,
		RecoveryAction: "restart node",
		StartedAt:      openStarted,
		UpdatedAt:      openStarted,
	})
	raw, err := json.Marshal(ledger)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "operations.json"), raw, 0o644))

	n := testkit.StartNode(t, runtimeinfra.Config{
		Name: "runtime-compaction",
		Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.DataConfig{Dir: dir},
	})

	pending := testkit.Diagnostics(n).PendingOperations()
	require.Len(t, pending, 1)
	require.Equal(t, "recovering", pending[0].State)
	require.Equal(t, "open-running", pending[0].ID)

	require.NoError(t, n.Stop(context.Background()))

	stored, err := persistence.Load(filepath.Join(dir, "operations.json"))
	require.NoError(t, err)
	require.LessOrEqual(t, len(stored.Operations), 33)

	openCount := 0
	foundShutdown := false
	foundRecovered := false
	for _, item := range stored.Operations {
		if operations.IsOpen(item.State) {
			openCount++
			if item.ID == "open-running" && item.State == operations.Recovering {
				foundRecovered = true
			}
		}
		if item.Kind == "node.shutdown" && item.State == operations.Completed {
			foundShutdown = true
		}
		require.NotEqual(t, "closed-100000", item.ID)
		require.NotEqual(t, "closed-100100", item.ID)
	}
	require.Equal(t, 1, openCount)
	require.True(t, foundRecovered)
	require.True(t, foundShutdown)
}
