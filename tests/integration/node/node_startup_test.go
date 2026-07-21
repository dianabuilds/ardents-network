//go:build integration

package node_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	appdata "ardents/internal/content"
	runtimeinfra "ardents/internal/daemon"
	discoveryapi "ardents/internal/discovery"
	workloadapi "ardents/internal/workload"
	workloadcontroller "ardents/internal/workload/execution"
	"ardents/tests/testkit"

	"github.com/stretchr/testify/require"
)

func TestNodeStartDegraded(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "node",
		ScenarioID:  "NRI-001",
		Suite:       "integration",
		Tags:        []string{"integration", "node"},
		Speed:       "default",
		Environment: "local",
	})
	testkit.ConfigureLoopbackTransport(t)
	harness := testkit.NewRuntime(t, runtimeinfra.Config{
		Name: "test",
		Boot: runtimeinfra.BootConfig{Sources: []string{"/ip4/127.0.0.1/tcp/9000"}},
		Data: runtimeinfra.DataConfig{Dir: t.TempDir()},
	})
	n := harness.Node
	{

		err := n.Start(context.Background())
		require.NoErrorf(t, err, "start node: %v", err)
	}
	t.Cleanup(func() { _ = n.Stop(context.Background()) })

	got := n.Snapshot()
	require.Falsef(t, got.Node.State != "degraded", "state = %q, want degraded", got.Node.State)
	require.False(t, got.Node.Reason == "", "expected degraded reason")
	require.Falsef(t, got.Disco.State != "degraded", "disco state = %q, want degraded", got.Disco.State)
	require.Falsef(t, got.Diag.Health.State !=
		"degraded", "diag health = %q, want degraded", got.Diag.Health.State)
	require.False(t, got.Diag.Health.PrimaryReason ==
		nil || got.
		Diag.Health.
		PrimaryReason.
		Code == "", "expected structured degraded reason")
	require.False(t, got.Boot.Joined, "expected boot joined=false without observed peer connectivity")
	require.Falsef(t, got.Boot.State != "degraded", "boot state = %q, want degraded", got.Boot.State)

}

func TestNodeFailsWhenStateLoadIsCorrupt(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "node",
		ScenarioID:  "NRI-001",
		Suite:       "integration",
		Tags:        []string{"integration", "node"},
		Speed:       "default",
		Environment: "local",
	})
	testkit.ConfigureLoopbackTransport(t)
	dir := t.TempDir()
	{
		err := os.WriteFile(filepath.Join(dir, "ardents.db"), []byte("{invalid"), 0o644)
		require.NoErrorf(t, err, "write corrupt state: %v", err)
	}

	harness := testkit.NewRuntime(t, runtimeinfra.Config{
		Name: "broken",
		Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.DataConfig{Dir: dir},
	})
	n := harness.Node
	err := n.Start(context.Background())
	require.Error(t, err, "expected startup error for corrupt state")
	require.Truef(t, strings.Contains(err.Error(), "node start failed"), "error = %v, want node start failed", err)

	got := n.Snapshot()
	require.Falsef(t, got.Node.State != "failed", "state = %q, want failed", got.Node.State)
	require.Falsef(t, got.Diag.Health.PrimaryReason ==
		nil || got.
		Diag.Health.
		PrimaryReason.
		Code != "node.state.load_failed", "primary reason = %#v, want node.state.load_failed", got.Diag.Health.PrimaryReason)

}

func TestNodeStopReturnsErrorWhenShutdownFails(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "node",
		ScenarioID:  "NRI-001",
		Suite:       "integration",
		Tags:        []string{"integration", "node"},
		Speed:       "default",
		Environment: "local",
	})
	testkit.ConfigureLoopbackTransport(t)
	dir := t.TempDir()
	harness := testkit.NewRuntime(t, runtimeinfra.Config{
		Name: "stop-fail",
		Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.DataConfig{Dir: dir},
	})
	n := harness.Node
	testkit.ReplaceWorkloadForIntegrationTest(harness, workloadcontroller.NewWithExecutorInDir(dir, nodeStopFailExecutor{}))
	{
		err := n.Start(context.Background())
		require.NoErrorf(t, err, "start node: %v", err)
	}
	{

		err := testkit.Workloads(n).Register(context.Background(), workloadapi.SpecSnapshot{
			ID:      "work.stop.fail",
			Kind:    "service",
			Owner:   "node",
			Config:  testkit.HelperProcessConfig(t, "sleep"),
			Desired: "running",
		})
		require.NoErrorf(t, err, "register workload: %v", err)
	}

	err := n.Stop(context.Background())
	require.Error(t, err, "expected stop error")
	require.Truef(t, strings.Contains(err.Error(), "node stop failed"), "error = %v, want node stop failed", err)
	{

		got := n.Snapshot()
		require.Falsef(t, got.Node.State != "failed", "state = %q, want failed", got.Node.State)
	}

}

func TestNodeRejectsAuthoritativeMutationsWhenFailed(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "node",
		ScenarioID:  "NRI-001",
		Suite:       "integration",
		Tags:        []string{"integration", "node"},
		Speed:       "default",
		Environment: "local",
	})
	testkit.ConfigureLoopbackTransport(t)
	dir := t.TempDir()
	{
		err := os.WriteFile(filepath.Join(dir, "ardents.db"), []byte("{invalid"), 0o644)
		require.NoErrorf(t, err, "write corrupt state: %v", err)
	}

	harness := testkit.NewRuntime(t, runtimeinfra.Config{
		Name: "failed-mutations",
		Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.DataConfig{Dir: dir},
	})
	n := harness.Node
	{
		err := n.Start(context.Background())
		require.Error(t, err, "expected failed startup")
	}
	{

		_, err := n.ImportRecord(discoveryapi.CatalogRecordSnapshot{ID: "x"})
		require.Falsef(t, err == nil || !strings.
			Contains(err.Error(),
				"node is failed",
			), "import error = %v, want failed-node rejection", err)
	}
	{

		err := testkit.Workloads(n).Register(context.Background(), workloadapi.SpecSnapshot{ID: "work.echo", Kind: "service", Owner: "node"})
		require.Falsef(t, err == nil || !strings.
			Contains(err.Error(),
				"node is failed",
			), "register error = %v, want failed-node rejection", err)
	}
	{

		_, err := n.PublishObject(appdata.Object{Type: "doc"})
		require.Falsef(t, err == nil || !strings.
			Contains(err.Error(),
				"node is failed",
			), "publish object error = %v, want failed-node rejection", err)
	}

}

func TestNodeStartRollsBackRuntimeWhenBlobExchangeStartupFails(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "node",
		ScenarioID:  "NRI-001",
		Suite:       "integration",
		Tags:        []string{"integration", "node"},
		Speed:       "default",
		Environment: "local",
	})
	testkit.ConfigureLoopbackTransport(t)
	harness := testkit.NewRuntime(t, runtimeinfra.Config{
		Name: "data-plane-rollback",
		Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.DataConfig{Dir: t.TempDir()},
	})
	n := harness.Node
	testkit.SetBlobExchangeStarterForIntegrationTest(harness, func(context.Context) error {
		return errors.New("boom")
	})

	err := n.Start(context.Background())
	require.Error(t, err, "expected startup error")
	require.Truef(t, strings.Contains(err.Error(), "start data-plane exchange"), "error = %v, want data-plane startup failure", err)
	require.Falsef(t, testkit.TransportStateForIntegrationTest(harness) != "stopped", "transport state = %q, want stopped after rollback", testkit.TransportStateForIntegrationTest(harness))
	require.True(t, testkit.NetworkSideEffectsClearedForIntegrationTest(harness), "expected network side effects to be cleared after startup rollback")
	{

		got := n.Snapshot()
		require.Falsef(t, got.Node.State != "stopped", "node state = %q, want stopped after rolled-back startup failure", got.Node.State)
	}

}

func TestNodeStartRollsBackRuntimeWhenCallerContextCancelsDuringBlobExchangeStartup(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "node",
		ScenarioID:  "NRI-001",
		Suite:       "integration",
		Tags:        []string{"integration", "node"},
		Speed:       "default",
		Environment: "local",
	})
	testkit.ConfigureLoopbackTransport(t)
	harness := testkit.NewRuntime(t, runtimeinfra.Config{
		Name: "data-plane-cancel",
		Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.DataConfig{Dir: t.TempDir()},
	})
	n := harness.Node
	ctx, cancel := context.WithCancel(context.Background())
	testkit.SetBlobExchangeStarterForIntegrationTest(harness, func(networkCtx context.Context) error {
		cancel()
		<-networkCtx.Done()
		return networkCtx.Err()
	})

	err := n.Start(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "start data-plane exchange")
	require.True(t, testkit.NetworkSideEffectsClearedForIntegrationTest(harness), "startup rollback must clear node network handles")

	got := n.Snapshot()
	require.NotEqual(t, "ready", got.Node.State)
	require.NotEqual(t, "degraded", got.Node.State)
	require.NotEqual(t, "ready", got.Node.Lifecycle.Current)
	require.NotEqual(t, "degraded", got.Node.Lifecycle.Current)
}

func TestNodeStartupPhasesPersistAsCompletedOperations(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "node",
		ScenarioID:  "NRI-001",
		Suite:       "integration",
		Tags:        []string{"integration", "node"},
		Speed:       "default",
		Environment: "local",
	})
	testkit.ConfigureLoopbackTransport(t)
	dir := t.TempDir()
	harness := testkit.NewRuntime(t, runtimeinfra.Config{
		Name: "phases",
		Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.DataConfig{Dir: dir},
	})
	n := harness.Node
	{
		err := n.Start(context.Background())
		require.NoErrorf(t, err, "start node: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(dir, "operations.json"))
	require.NoErrorf(t, err, "read operations ledger: %v", err)

	var ledger struct {
		Operations []struct {
			Kind  string `json:"kind"`
			State string `json:"state"`
		} `json:"operations"`
	}
	{
		err := json.Unmarshal(raw, &ledger)
		require.NoErrorf(t, err, "decode operations ledger: %v", err)
	}

	want := map[string]bool{
		runtimeinfra.StartupPhaseStateLoad: false,
		runtimeinfra.StartupPhaseIdentity:  false,
		runtimeinfra.StartupPhaseDiscovery: false,
		runtimeinfra.StartupPhaseWorkloads: false,
	}
	for _, item := range ledger.Operations {
		if _, ok := want[item.Kind]; !ok {
			continue
		}
		require.Falsef(t, item.State != "completed", "phase %s state = %q, want completed", item.Kind, item.State)

		want[item.Kind] = true
	}
	for kind, found := range want {
		require.Truef(t, found, "missing startup phase %s in operations ledger", kind)

	}
}

func TestNodeShutdownPhasePersistsAsCompletedOperation(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "node",
		ScenarioID:  "NRI-001",
		Suite:       "integration",
		Tags:        []string{"integration", "node"},
		Speed:       "default",
		Environment: "local",
	})
	testkit.ConfigureLoopbackTransport(t)
	dir := t.TempDir()
	harness := testkit.NewRuntime(t, runtimeinfra.Config{
		Name: "shutdown",
		Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.DataConfig{Dir: dir},
	})
	n := harness.Node
	{
		err := n.Start(context.Background())
		require.NoErrorf(t, err, "start node: %v", err)
	}
	{

		err := n.Stop(context.Background())
		require.NoErrorf(t, err, "stop node: %v", err)
	}

	got := n.Snapshot()
	require.Falsef(t, got.Node.State != "stopped", "state = %q, want stopped", got.Node.State)
	require.Falsef(t, got.Node.Lifecycle.Current !=
		"stopped", "lifecycle current = %q, want stopped", got.Node.Lifecycle.Current)

	raw, err := os.ReadFile(filepath.Join(dir, "operations.json"))
	require.NoErrorf(t, err, "read operations ledger: %v", err)

	var ledger struct {
		Operations []struct {
			Kind  string `json:"kind"`
			State string `json:"state"`
		} `json:"operations"`
	}
	{
		err := json.Unmarshal(raw, &ledger)
		require.NoErrorf(t, err, "decode operations ledger: %v", err)
	}

	found := false
	for _, item := range ledger.Operations {
		if item.Kind != runtimeinfra.ShutdownPhaseNode {
			continue
		}
		found = true
		require.Falsef(t, item.State != "completed", "shutdown phase state = %q, want completed", item.State)

	}
	require.Truef(t, found, "missing shutdown phase %s in operations ledger", runtimeinfra.ShutdownPhaseNode)

}

type nodeStopFailExecutor struct{}

func (nodeStopFailExecutor) Prepare(context.Context, workloadcontroller.Request) (workloadcontroller.PreparedWorkload, error) {
	return workloadcontroller.PreparedWorkload{
		WorkloadID: "work.stop.fail",
		Generation: 1,
	}, nil
}

func (nodeStopFailExecutor) Start(context.Context, workloadcontroller.PreparedWorkload) (workloadcontroller.Instance, error) {
	return workloadcontroller.Instance{
		WorkloadID: "work.stop.fail",
		Generation: 1,
		Running:    true,
	}, nil
}

func (nodeStopFailExecutor) Stop(context.Context, workloadcontroller.Instance) error {
	return errors.New("executor stop boom")
}

func (nodeStopFailExecutor) Inspect(context.Context, string) (workloadcontroller.Instance, error) {
	return workloadcontroller.Instance{
		WorkloadID: "work.stop.fail",
		Running:    true,
	}, nil
}
