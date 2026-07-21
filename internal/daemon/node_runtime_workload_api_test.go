package daemon_test

import (
	"context"
	"testing"
	"time"

	process "ardents/internal/daemon"
	workloadapi "ardents/internal/workload"
	"ardents/internal/workload/execution"
	"ardents/tests/testkit"

	"github.com/stretchr/testify/require"
)

func TestNodeWorkloadAPIReportsStatusAndRestarts(t *testing.T) {
	n := process.NewNode(process.Config{
		Name:             "workload-api-restart",
		Boot:             process.BootConfig{Sources: []string{"local://bootstrap"}},
		Data:             process.DataConfig{Dir: t.TempDir()},
		WorkloadExecutor: execution.NewLocalExecutor(),
	})
	require.NoError(t, n.Start(context.Background()))
	defer func() { require.NoError(t, n.Stop(context.Background())) }()

	workloads := testkit.Workloads(n)
	require.NoError(t, workloads.Register(context.Background(), workloadapi.SpecSnapshot{
		ID:      "work.echo",
		Kind:    "service",
		Owner:   "node",
		Config:  testkit.HelperProcessConfig(t, "sleep"),
		Desired: "running",
	}))

	started, err := workloads.Get("work.echo")
	require.NoError(t, err)
	require.Equal(t, "running", started.Observed)
	firstGeneration := started.Instance.Generation

	require.NoError(t, workloads.Restart(context.Background(), "work.echo"))

	deadline := time.Now().Add(3 * time.Second)
	for {
		current, err := workloads.Get("work.echo")
		require.NoError(t, err)
		if current.Observed == "running" && current.Instance.Generation != firstGeneration {
			return
		}
		require.Falsef(t, time.Now().After(deadline), "restart did not produce a new running generation: %#v", current)
		time.Sleep(20 * time.Millisecond)
	}
}
