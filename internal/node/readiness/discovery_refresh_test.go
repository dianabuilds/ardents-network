package readiness_test

import (
	"errors"
	"testing"

	"ardents/internal/diagnostics"
	nodelifecycle "ardents/internal/node/lifecycle"
	nodereadiness "ardents/internal/node/readiness"

	"github.com/stretchr/testify/require"
)

func TestRecordDiscoveryRefreshFailureMarksDiscoveryAndDegradesLifecycle(t *testing.T) {
	diag := diagnostics.NewInDir(t.TempDir())
	life := nodelifecycle.NewMachine()
	require.NoError(t, life.Move(nodelifecycle.Starting))
	require.NoError(t, life.Move(nodelifecycle.Initializing))
	called := ""

	nodereadiness.RecordDiscoveryRefreshFailure(
		diag,
		"node-a",
		errors.New("refresh failed"),
		func(reason string) { called = reason },
		func(domain string, state string, reason *diagnostics.Reason) {
			diag.SetPrimary(state, reason)
		},
		func(state string) { _ = life.Move(state) },
		func(string, map[string]any) {},
	)

	require.Equal(t, "refresh failed", called)
	require.Equal(t, nodelifecycle.Degraded, life.State())
	require.Equal(t, nodereadiness.DiscoveryRefreshFailedCode, nodereadiness.SubsystemReasonCode(diag.Health(), "discovery"))
}

func TestClearDiscoveryRefreshFailureClearsRefreshReason(t *testing.T) {
	diag := diagnostics.NewInDir(t.TempDir())
	diag.SetSubsystem("discovery", diagnostics.HealthDegraded, &diagnostics.Reason{
		Code:    nodereadiness.DiscoveryRefreshFailedCode,
		Domain:  "discovery",
		Summary: "refresh failed",
	})
	life := nodelifecycle.NewMachine()
	require.NoError(t, life.Move(nodelifecycle.Starting))
	require.NoError(t, life.Move(nodelifecycle.Initializing))
	readyCalled := false

	nodereadiness.ClearDiscoveryRefreshFailure(
		diag,
		func() { readyCalled = true },
		func(domain string) { nodereadiness.RestorePrimaryReason(diag, domain) },
		func(state string) { _ = life.Move(state) },
	)

	require.True(t, readyCalled)
	require.Equal(t, nodelifecycle.Ready, life.State())
}
