package workload

import (
	"testing"
	"time"

	"ardents/internal/hosting"

	"github.com/stretchr/testify/require"
)

func TestHostingMappingPreservesReadiness(t *testing.T) {
	now := time.Now().UTC()
	service := toHostedServiceSnapshot(hosting.ServiceSnapshot{
		ID: "svc.echo", WorkloadID: "work.echo", Readiness: "ready", Ready: true,
		ExposureEligible: true, Generation: 71, LastProbeAt: now,
		Endpoints: []hosting.EndpointSnapshot{{Address: "tcp://127.0.0.1:9000", Reachable: true}},
	})
	require.Equal(t, "work.echo", service.GetWorkloadId())
	require.True(t, service.GetEndpoints()[0].GetReachable())
	require.True(t, service.GetExposureEligible())
	require.Equal(t, int64(71), service.GetGeneration())

	status := toHostedServiceStatusSnapshot(hosting.ServiceStatusSnapshot{
		ServiceID: "svc.echo", State: "ready", Ready: true, ExposureEligible: true,
		Generation: 71, LastProbeAt: now,
	})
	require.True(t, status.GetReady())
	require.Equal(t, int64(71), status.GetGeneration())
}
