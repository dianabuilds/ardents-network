package readiness_test

import (
	"testing"

	"ardents/internal/diagnostics"
	transport "ardents/internal/network/api"
	nodereadiness "ardents/internal/node/readiness"

	"github.com/stretchr/testify/require"
)

func TestApplyTransportHealthMarksReadyWithoutReason(t *testing.T) {
	diag := diagnostics.NewInDir(t.TempDir())

	nodereadiness.ApplyTransportHealth(diag, "ready", "", transport.Snapshot{
		Profile: transport.ProfileTCPWSS,
		Mode:    transport.ModeSteady,
	})

	health := diag.Health()
	require.Empty(t, nodereadiness.SubsystemReasonCode(health, "transport"))
}

func TestApplyTransportHealthPrefixesReasonWithProfileAndMode(t *testing.T) {
	diag := diagnostics.NewInDir(t.TempDir())

	nodereadiness.ApplyTransportHealth(diag, "degraded", "bootstrap unreachable", transport.Snapshot{
		Profile: transport.ProfileTCPWSS,
		Mode:    transport.ModeSteady,
	})

	health := diag.Health()
	require.Equal(t, "transport.bootstrap.degraded", nodereadiness.SubsystemReasonCode(health, "transport"))
	require.Contains(t, health.Subsystems[0].Reason.Detail, "profile tcp_wss, mode steady: bootstrap unreachable")
}

func TestApplyTransportHealthProjectsRestrictedDefenseCooldown(t *testing.T) {
	diag := diagnostics.NewInDir(t.TempDir())

	nodereadiness.ApplyTransportHealth(diag, "degraded", "restricted defense mode is active", transport.Snapshot{
		Profile: transport.ProfileTCPWSS,
		Mode:    transport.ModeRestrictedDefense,
	})

	health := diag.Health()
	require.Equal(t, "transport.mode.restricted_defense", nodereadiness.SubsystemReasonCode(health, "transport"))
	require.Equal(t, "transport is in restricted defense recovery cooldown", health.Subsystems[0].Reason.Summary)
	require.Equal(t, "automatic", health.Subsystems[0].Reason.Recovery)
	require.False(t, health.Subsystems[0].Reason.OperatorActionRequired)
	require.Contains(t, health.Subsystems[0].Reason.Detail, "profile tcp_wss, mode restricted_defense: restricted defense mode is active")
}
