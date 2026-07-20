package connectrpc

import (
	"testing"
	"time"

	diagapi "ardents/internal/diagnostics/api"
	hostingapi "ardents/internal/hosting/api"
	nodeapi "ardents/internal/node/api"

	"github.com/stretchr/testify/require"
)

func TestSurfaceMappersPreserveNewSnapshotFields(t *testing.T) {
	now := time.Now().UTC()
	runtime := toNodeRuntimeSnapshot(nodeapi.NodeRuntimeSnapshot{
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

	service := toHostedServiceSnapshot(hostingapi.HostedServiceSnapshot{
		ID:                 "svc.echo",
		Type:               "echo",
		Owner:              "node",
		WorkloadID:         "work.echo",
		DesiredPublication: "running",
		RuntimeBacking:     "active",
		Readiness:          "ready",
		Ready:              true,
		ExposureEligible:   true,
		Generation:         71,
		LastProbeAt:        now,
		Endpoints: []hostingapi.ServiceEndpointSnapshot{{
			Kind:      "tcp",
			Address:   "tcp://127.0.0.1:9000",
			Protocol:  "tcp",
			Exposure:  "network",
			Reachable: true,
		}},
	})
	require.Equal(t, "svc.echo", service.GetId())
	require.Equal(t, "work.echo", service.GetWorkloadId())
	require.Len(t, service.GetEndpoints(), 1)
	require.True(t, service.GetEndpoints()[0].GetReachable())
	require.Equal(t, "ready", service.GetReadiness())
	require.True(t, service.GetReady())
	require.True(t, service.GetExposureEligible())
	require.Equal(t, int64(71), service.GetGeneration())
	require.Equal(t, now.Unix(), service.GetLastProbeAt().AsTime().Unix())

	status := toHostedServiceStatusSnapshot(hostingapi.HostedServiceStatusSnapshot{ServiceID: "svc.echo", State: "ready",
		Ready: true, ExposureEligible: true, Generation: 71, LastProbeAt: now})
	require.True(t, status.GetReady())
	require.True(t, status.GetExposureEligible())
	require.Equal(t, int64(71), status.GetGeneration())
}
