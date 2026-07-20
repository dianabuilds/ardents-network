//go:build integration

package workload_test

import (
	"context"
	"path/filepath"
	"testing"

	workloadcontroller "ardents/internal/workload/controller"
	"ardents/tests/testkit"

	"github.com/stretchr/testify/require"
)

func TestRuntimeRecoveryPersistsAndPublishes(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "workload",
		ScenarioID:  "WKI-002",
		Suite:       "integration",
		Tags:        []string{"integration", "workload"},
		Speed:       "default",
		Environment: "local",
	})
	path := filepath.Join(t.TempDir(), "ardents.db")
	svc := workloadcontroller.New(path, workloadcontroller.NewLocalExecutor())
	require.NoError(t, svc.Load())
	require.NoError(t, svc.Register(workloadcontroller.Spec{
		ID:      "work.echo",
		Kind:    "service",
		Owner:   "node",
		Config:  testkit.HelperProcessConfig(t, "sleep"),
		Desired: workloadcontroller.DesiredRunning,
		Services: []workloadcontroller.ServiceSpec{{
			ID:        "svc.work.echo",
			Type:      "echo",
			Mode:      "NetworkPublished",
			Endpoints: []string{"quic://echo:9000"},
		}},
	}))
	require.NoError(t, svc.Reconcile(context.Background()))
	require.Len(t, svc.Published(), 1)

	restored := workloadcontroller.New(path, workloadcontroller.NewLocalExecutor())
	require.NoError(t, restored.Load())
	items := restored.List()
	require.Len(t, items, 1)
	require.Equal(t, workloadcontroller.ObservedRunning, items[0].Observed)
	require.True(t, items[0].Instance.Running)
	require.NotZero(t, items[0].Instance.PID)
	require.Len(t, restored.Published(), 1)
	require.Empty(t, items[0].PublishedServices[0].Reason)
	require.NoError(t, restored.Reconcile(context.Background()))
	items = restored.List()
	require.Equal(t, workloadcontroller.ObservedRunning, items[0].Observed)
	require.True(t, items[0].Instance.Running)
	require.Len(t, items[0].PublishedServices, 1)
	require.True(t, items[0].PublishedServices[0].Published)
}

func TestRuntimeStoppedAndRemovedTransitions(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "workload",
		ScenarioID:  "WKI-002",
		Suite:       "integration",
		Tags:        []string{"integration", "workload"},
		Speed:       "default",
		Environment: "local",
	})
	svc := workloadcontroller.New(filepath.Join(t.TempDir(), "ardents.db"), workloadcontroller.NewLocalExecutor())
	require.NoError(t, svc.Register(workloadcontroller.Spec{
		ID:      "work.echo",
		Kind:    "service",
		Owner:   "node",
		Config:  testkit.HelperProcessConfig(t, "sleep"),
		Desired: workloadcontroller.DesiredRunning,
	}))
	require.NoError(t, svc.Reconcile(context.Background()))
	require.NoError(t, svc.SetDesired("work.echo", workloadcontroller.DesiredStopped))
	require.NoError(t, svc.Reconcile(context.Background()))
	item, ok := svc.Get("work.echo")
	require.True(t, ok)
	require.Equal(t, workloadcontroller.ObservedStopped, item.Observed)

	require.NoError(t, svc.SetDesired("work.echo", workloadcontroller.DesiredRemoved))
	require.NoError(t, svc.Reconcile(context.Background()))
	{
		_, ok := svc.Get("work.echo")
		require.False(t, ok, "expected workload removal")
	}

}

func TestRuntimeStopAllMarksStoppedAndUnpublished(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "workload",
		ScenarioID:  "WKI-002",
		Suite:       "integration",
		Tags:        []string{"integration", "workload"},
		Speed:       "default",
		Environment: "local",
	})
	path := filepath.Join(t.TempDir(), "ardents.db")
	svc := workloadcontroller.New(path, workloadcontroller.NewLocalExecutor())
	require.NoError(t, svc.Load())
	require.NoError(t, svc.Register(workloadcontroller.Spec{
		ID:      "work.echo",
		Kind:    "service",
		Owner:   "node",
		Config:  testkit.HelperProcessConfig(t, "sleep"),
		Desired: workloadcontroller.DesiredRunning,
		Services: []workloadcontroller.ServiceSpec{{
			ID:        "svc.work.echo",
			Type:      "echo",
			Mode:      "NetworkPublished",
			Endpoints: []string{"quic://echo:9000"},
		}},
	}))
	require.NoError(t, svc.Reconcile(context.Background()))

	require.NoError(t, svc.StopAll(context.Background()))

	item, ok := svc.Get("work.echo")
	require.True(t, ok)
	require.Equal(t, workloadcontroller.ObservedStopped, item.Observed)
	require.False(t, item.Instance.Running)
	require.Len(t, svc.Published(), 0)

	restored := workloadcontroller.New(path, workloadcontroller.NewLocalExecutor())
	require.NoError(t, restored.Load())
	loaded, ok := restored.Get("work.echo")
	require.True(t, ok)
	require.Equal(t, workloadcontroller.ObservedStopped, loaded.Observed)
	require.False(t, loaded.Instance.Running)
}
