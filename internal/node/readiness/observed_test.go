package readiness_test

import (
	"testing"

	"ardents/internal/diagnostics"
	transport "ardents/internal/network/api"
	nodelifecycle "ardents/internal/node/lifecycle"
	nodereadiness "ardents/internal/node/readiness"
	noderecovery "ardents/internal/node/recovery"

	"github.com/stretchr/testify/require"
)

func TestSyncBootHealthProjectsDegradedBootstrap(t *testing.T) {
	diag := diagnostics.NewInDir(t.TempDir())
	boot := noderecovery.NewBootStatus(noderecovery.BootConfig{})

	nodereadiness.SyncBootHealth(diag, boot, transport.BootstrapStatus{
		Joined: true,
		State:  "degraded",
		Reason: "bootstrap incomplete",
	})

	require.Equal(t, noderecovery.BootDegraded, boot.Result().State)
	health := diag.Health()
	require.Equal(t, diagnostics.HealthDegraded, health.State)
	require.Equal(t, "boot.join.degraded", nodereadiness.SubsystemReasonCode(health, "boot"))
}

func TestSyncPrimaryReasonPrefersObservedSubsystem(t *testing.T) {
	diag := diagnostics.NewInDir(t.TempDir())
	diag.SetSubsystem("discovery", diagnostics.HealthDegraded, &diagnostics.Reason{
		Code:    "discovery.refresh_failed",
		Domain:  "discovery",
		Summary: "refresh failed",
	})

	nodereadiness.SyncPrimaryReason(diag)

	health := diag.Health()
	require.NotNil(t, health.PrimaryReason)
	require.Equal(t, "discovery", health.PrimaryReason.Domain)
}

func TestSyncLifecycleStateMovesFromDiagnosticsHealth(t *testing.T) {
	diag := diagnostics.NewInDir(t.TempDir())
	life := nodelifecycle.NewMachine()
	require.NoError(t, life.Move(nodelifecycle.Starting))
	require.NoError(t, life.Move(nodelifecycle.Initializing))
	diag.SetSubsystem("discovery", diagnostics.HealthDegraded, &diagnostics.Reason{
		Code:    "discovery.refresh_failed",
		Domain:  "discovery",
		Summary: "refresh failed",
	})

	nodereadiness.SyncLifecycleState(life, diag, func(next string) {
		_ = life.Move(next)
	})

	require.Equal(t, nodelifecycle.Degraded, life.State())
}
