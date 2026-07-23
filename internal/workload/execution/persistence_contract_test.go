package execution_test

import (
	"path/filepath"
	"testing"

	persistence "ardents/internal/storage"
	workloadcontroller "ardents/internal/workload/execution"
	workloadregistry "ardents/internal/workload/registry"

	"github.com/stretchr/testify/require"
)

func TestWorkloadStateUsesStrictVersionedRequirementsAndRestarts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ardents.db")
	service := workloadcontroller.New(path, workloadcontroller.NewLocalExecutor())
	require.NoError(t, service.Load())
	require.NoError(t, service.Register(workloadregistry.Spec{
		ID: "work.gpu", Kind: "service", Owner: "node",
		Requirements: []workloadregistry.WorkloadRequirement{"gpu"},
	}))

	restarted := workloadcontroller.New(path, workloadcontroller.NewLocalExecutor())
	require.NoError(t, restarted.Load())
	item, found := restarted.Get("work.gpu")
	require.True(t, found)
	require.Equal(t, []workloadregistry.WorkloadRequirement{"gpu"}, item.Spec.Requirements)
}

func TestWorkloadStateRejectsObsoleteAndMalformedRecordsWithoutReplacingLiveState(t *testing.T) {
	tests := []struct {
		name  string
		state map[string]any
	}{
		{"missing version", workloadStateFixture()},
		{"unknown version", withWorkloadStateVersion(workloadStateFixture(), 99)},
		{"missing items", map[string]any{"version": 1}},
		{"legacy capabilities", withWorkloadSpecField(workloadStateFixture(), "capabilities", []string{"gpu"})},
		{"malformed requirement", withWorkloadSpecField(workloadStateFixture(), "requirements", []string{" GPU "})},
		{"mismatched key", withWorkloadSpecField(workloadStateFixture(), "id", "work.other")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "ardents.db")
			service := workloadcontroller.New(path, workloadcontroller.NewLocalExecutor())
			require.NoError(t, service.Load())
			require.NoError(t, service.Register(workloadregistry.Spec{ID: "work.live", Kind: "service", Owner: "node"}))

			require.NoError(t, persistence.SaveJSON(path, "workload", "snapshot", test.state))
			require.Error(t, service.Load())

			item, found := service.Get("work.live")
			require.True(t, found)
			require.Equal(t, "work.live", item.Spec.ID)
			_, replaced := service.Get("work.echo")
			require.False(t, replaced)
		})
	}
}

func workloadStateFixture() map[string]any {
	return map[string]any{
		"items": map[string]any{
			"work.echo": map[string]any{
				"spec": map[string]any{
					"id": "work.echo", "kind": "service", "owner": "node",
					"desired": "present", "requirements": []string{"gpu"},
				},
				"observed":              "accepted",
				"last_transition_at":    "2026-01-01T00:00:00Z",
				"needs_operator_action": false,
				"restart_count":         0,
				"instance": map[string]any{
					"workload_id": "", "generation": 0, "running": false,
					"started_at": "0001-01-01T00:00:00Z", "finished_at": "0001-01-01T00:00:00Z",
					"restarts": 0, "memory_limit_bytes": 0, "nano_cpus": 0, "pids_limit": 0,
				},
			},
		},
	}
}

func withWorkloadStateVersion(state map[string]any, version int) map[string]any {
	state["version"] = version
	return state
}

func withWorkloadSpecField(state map[string]any, name string, value any) map[string]any {
	state["version"] = 1
	items := state["items"].(map[string]any)
	status := items["work.echo"].(map[string]any)
	spec := status["spec"].(map[string]any)
	spec[name] = value
	return state
}
