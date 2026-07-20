package readiness_test

import (
	"testing"

	"ardents/internal/diagnostics"
	nodereadiness "ardents/internal/node/readiness"

	"github.com/stretchr/testify/require"
)

func TestAdoptPrimaryReasonUsesObservedOwnershipRule(t *testing.T) {
	diag := diagnostics.NewInDir(t.TempDir())
	diag.SetPrimary(diagnostics.HealthDegraded, &diagnostics.Reason{
		Code:    "policy.denied",
		Domain:  "policy",
		Summary: "policy denied",
	})

	nodereadiness.AdoptPrimaryReason(diag, "discovery", diagnostics.HealthDegraded, &diagnostics.Reason{
		Code:    "discovery.refresh_failed",
		Domain:  "discovery",
		Summary: "refresh failed",
	})

	require.Equal(t, "policy", diag.Health().PrimaryReason.Domain)
}

func TestRestorePrimaryReasonPromotesSubsystemFallback(t *testing.T) {
	diag := diagnostics.NewInDir(t.TempDir())
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

	nodereadiness.RestorePrimaryReason(diag, "discovery")

	health := diag.Health()
	require.NotNil(t, health.PrimaryReason)
	require.Equal(t, "transport", health.PrimaryReason.Domain)
}
