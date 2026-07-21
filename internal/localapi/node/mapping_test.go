package node

import (
	"testing"
	"time"

	daemonruntime "ardents/internal/daemon"
	diagapi "ardents/internal/diagnostics"

	"github.com/stretchr/testify/require"
)

func TestSurfaceMappersPreserveNewSnapshotFields(t *testing.T) {
	now := time.Now().UTC()
	runtime := toNodeRuntimeSnapshot(daemonruntime.RuntimeSnapshot{
		Health: diagapi.HealthSnapshot{
			State:                  "degraded",
			PrimaryReason:          &diagapi.ReasonSnapshot{Code: "node.transport.degraded", Summary: "transport degraded"},
			Subsystems:             []diagapi.SubsystemHealthSnapshot{{Domain: "transport", State: "degraded"}},
			OperatorActionRequired: true,
			UpdatedAt:              now,
		},
	})
	health := runtime.GetHealth()
	require.Equal(t, "degraded", health.GetState())
	require.True(t, health.GetOperatorActionRequired())
	require.NotNil(t, health.GetPrimaryReason())
	require.Equal(t, now.Unix(), health.GetUpdatedAt().AsTime().Unix())
	require.Len(t, health.GetSubsystems(), 1)
	require.Equal(t, "transport", health.GetSubsystems()[0].GetDomain())

}
