package process_test

import (
	"context"
	"testing"
	"time"

	process "ardents/internal/runtime/process"
	workloadapi "ardents/internal/workload/api"
	"ardents/tests/testkit"

	"github.com/stretchr/testify/require"
)

func TestNodeWorkloadAPIReportsStatusAndRestarts(t *testing.T) {
	n := process.NewNode(process.Config{
		Name: "workload-api-restart",
		Boot: process.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: process.DataConfig{Dir: t.TempDir()},
	})
	require.NoError(t, n.Start(context.Background()))
	defer func() { _ = n.Stop(context.Background()) }()

	require.NoError(t, n.RegisterWorkloadContext(context.Background(), workloadapi.WorkloadSpecSnapshot{
		ID:      "work.echo",
		Kind:    "service",
		Owner:   "node",
		Config:  testkit.HelperProcessConfig(t, "sleep"),
		Desired: "running",
	}))

	started, err := n.GetWorkloadStatus("work.echo")
	require.NoError(t, err)
	require.Equal(t, "running", started.Observed)
	firstGeneration := started.Instance.Generation

	require.NoError(t, n.RestartWorkloadContext(context.Background(), "work.echo"))

	deadline := time.Now().Add(3 * time.Second)
	for {
		current, err := n.GetWorkloadStatus("work.echo")
		require.NoError(t, err)
		if current.Observed == "running" && current.Instance.Generation != firstGeneration {
			return
		}
		require.Falsef(t, time.Now().After(deadline), "restart did not produce a new running generation: %#v", current)
		time.Sleep(20 * time.Millisecond)
	}
}
