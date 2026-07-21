//go:build e2e

package nodee2e_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	runtimeinfra "ardents/internal/daemon"
	runtimeprocess "ardents/internal/daemon"
	"ardents/tests/testkit"

	"github.com/stretchr/testify/require"
)

func TestNodeRuntimeLifecycleAcrossRestartPreservesPendingTruth(t *testing.T) {
	scenario := testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerE2E,
		Domain:      "node",
		ScenarioID:  "NRE-001",
		Suite:       "e2e",
		Tags:        []string{"integration", "e2e", "node"},
		Speed:       "default",
		Environment: "local",
	})

	ctx := context.Background()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "operations.json"), []byte(seedRecoverableOperation), 0o644))

	cfg := runtimeinfra.Config{
		Name: "node-runtime-e2e",
		Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.DataConfig{Dir: dir},
	}

	var first *runtimeprocess.Node
	var second *runtimeprocess.Node

	scenario.Precondition("start node from persisted recoverable operation", func(t *testing.T) {
		first = testkit.NewRuntime(t, cfg).Runtime
		require.NoError(t, first.Start(ctx))
		t.Cleanup(func() {
			_ = first.Stop(ctx)
		})
	})

	scenario.Step("runtime status and diagnostics expose recovery truth after startup", func(t *testing.T) {
		assertReadyRuntime(t, first)
		assertRecoveringPendingOperation(t, first)
	})

	scenario.Step("graceful shutdown persists terminal shutdown fate", func(t *testing.T) {
		require.NoError(t, first.Stop(ctx))
		assertPersistedShutdownRecord(t, dir)
	})

	scenario.Step("restart node from the same data dir", func(t *testing.T) {
		second = testkit.NewRuntime(t, cfg).Runtime
		require.NoError(t, second.Start(ctx))
		t.Cleanup(func() {
			_ = second.Stop(ctx)
		})
	})

	scenario.Assert("restart keeps pending-operation recovery visible on canonical surfaces", func(t *testing.T) {
		assertReadyRuntime(t, second)
		assertRecoveringPendingOperation(t, second)
	})
}

func assertReadyRuntime(t *testing.T, runtime *runtimeprocess.Node) {
	t.Helper()

	status := runtime.Snapshot()
	require.Equal(t, "ready", status.Node.State)
	require.True(t, status.Node.Ready)
	require.Equal(t, "ready", status.Node.Lifecycle.Current)
}

func assertRecoveringPendingOperation(t *testing.T, runtime *runtimeprocess.Node) {
	t.Helper()

	diag := testkit.Diagnostics(runtime).DiagnosticsSnapshot()
	require.NotEmpty(t, diag.PendingOperations)
	require.Equal(t, "recovering", diag.PendingOperations[0].State)
	require.Equal(t, "node.startup.workloads", diag.PendingOperations[0].Kind)

	pending := testkit.Diagnostics(runtime).PendingOperations()
	require.NotEmpty(t, pending)
	require.Equal(t, "recovering", pending[0].State)
	require.Equal(t, "restart node", pending[0].RecoveryAction)
}

func assertPersistedShutdownRecord(t *testing.T, dir string) {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join(dir, "operations.json"))
	require.NoError(t, err)
	text := string(raw)
	require.Contains(t, text, `"kind": "node.shutdown"`)
	require.Contains(t, text, `"state": "completed"`)
}

const seedRecoverableOperation = `{"operations":[{"id":"op-1","kind":"node.startup.workloads","state":"running","domain":"workload","resource":"workloads","recoverable":true,"recovery_action":"restart node","started_at":"2026-03-20T10:00:00Z","updated_at":"2026-03-20T10:00:00Z"}]}`
