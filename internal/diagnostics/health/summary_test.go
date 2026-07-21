package health

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestComposeUsesFailedSubsystemAsPrimaryReason(t *testing.T) {
	now := time.Date(2026, 3, 22, 18, 0, 0, 0, time.UTC)
	summary := Compose(now, "", false, nil, map[string]SubsystemStatus{
		"transport": {
			Domain: "transport",
			State:  Failed,
			Reason: &Reason{Code: "transport.failed", Domain: "transport", Summary: "transport failed"},
		},
	})
	require.Falsef(t, summary.State != Failed, "state = %q, want failed", summary.State)
	require.Falsef(t, summary.PrimaryReason == nil || summary.PrimaryReason.Code != "transport.failed", "primary reason = %#v, want transport.failed", summary.PrimaryReason)
}

func TestCloneSummaryCopiesReasonAndSubsystems(t *testing.T) {
	now := time.Date(2026, 3, 22, 17, 0, 0, 0, time.UTC)
	current := Summary{
		State:         Degraded,
		UpdatedAt:     now,
		PrimaryReason: &Reason{Code: "boot.degraded", Domain: "boot", Summary: "boot degraded"},
		Subsystems: []SubsystemStatus{{
			Domain:    "boot",
			State:     Degraded,
			Reason:    &Reason{Code: "boot.degraded", Domain: "boot", Summary: "boot degraded"},
			UpdatedAt: now,
		}},
	}
	out := CloneSummary(current)
	require.Falsef(t, out.PrimaryReason == nil || out.PrimaryReason.Code != "boot.degraded", "primary reason = %#v, want boot.degraded", out.PrimaryReason)
	require.Falsef(t, len(out.Subsystems) != 1 || out.Subsystems[0].Domain != "boot", "subsystems = %#v, want cloned boot subsystem", out.Subsystems)
}
