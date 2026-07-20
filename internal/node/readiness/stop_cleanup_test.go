package readiness_test

import (
	"testing"

	"ardents/internal/diagnostics"
	nodereadiness "ardents/internal/node/readiness"
	noderecovery "ardents/internal/node/recovery"

	"github.com/stretchr/testify/require"
)

func TestClearRuntimeHealthForStopResetsBootAndSubsystems(t *testing.T) {
	diag := diagnostics.NewInDir(t.TempDir())
	boot := noderecovery.NewBootStatus(noderecovery.BootConfig{})
	boot.SetResult(noderecovery.BootResult{Joined: true, State: noderecovery.BootDegraded, Reason: "degraded"})
	diag.SetSubsystem("transport", diagnostics.HealthDegraded, &diagnostics.Reason{
		Code:    "transport.bootstrap.degraded",
		Domain:  "transport",
		Summary: "transport degraded",
	})

	nodereadiness.ClearRuntimeHealthForStop(diag, boot)

	require.Equal(t, noderecovery.BootIdle, boot.Result().State)
	require.Empty(t, nodereadiness.SubsystemReasonCode(diag.Health(), "transport"))
}

func TestClearRuntimeHealthForStopClearsObservedPrimary(t *testing.T) {
	diag := diagnostics.NewInDir(t.TempDir())
	boot := noderecovery.NewBootStatus(noderecovery.BootConfig{})
	diag.SetSubsystem("transport", diagnostics.HealthDegraded, &diagnostics.Reason{
		Code:    "transport.bootstrap.degraded",
		Domain:  "transport",
		Summary: "transport degraded",
	})
	diag.SetPrimary(diagnostics.HealthDegraded, &diagnostics.Reason{
		Code:    "discovery.refresh_failed",
		Domain:  "discovery",
		Summary: "refresh failed",
	})

	nodereadiness.ClearRuntimeHealthForStop(diag, boot)

	health := diag.Health()
	require.Nil(t, health.PrimaryReason)
}
