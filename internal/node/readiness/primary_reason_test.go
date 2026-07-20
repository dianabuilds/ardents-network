package readiness_test

import (
	"testing"

	"ardents/internal/diagnostics"
	nodereadiness "ardents/internal/node/readiness"

	"github.com/stretchr/testify/require"
)

func TestCurrentPrimaryReasonSummaryUsesActivePrimaryReason(t *testing.T) {
	diag := diagnostics.NewInDir(t.TempDir())
	diag.SetPrimary(diagnostics.HealthDegraded, &diagnostics.Reason{
		Code:    "discovery.refresh_failed",
		Domain:  "discovery",
		Summary: "refresh failed",
	})

	require.Equal(t, "refresh failed", nodereadiness.CurrentPrimaryReasonSummary(diag))
}

func TestCurrentPrimaryReasonCodeFallsBackToEmpty(t *testing.T) {
	diag := diagnostics.NewInDir(t.TempDir())

	require.Empty(t, nodereadiness.CurrentPrimaryReasonCode(diag))
}
