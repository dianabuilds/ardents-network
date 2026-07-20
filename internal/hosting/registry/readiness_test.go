package registry_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"ardents/internal/hosting/readiness"
	"ardents/internal/hosting/registry"
	hostingservice "ardents/internal/hosting/service"
	"github.com/stretchr/testify/require"
)

func TestRegistryOwnsGenerationBoundReadinessTruth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Ardents-Generation", "31")
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)

	controller := readiness.NewController(readiness.Policy{Timeout: 100 * time.Millisecond, SuccessThreshold: 2, FailureThreshold: 2, StaleAfter: time.Second})
	services := registry.NewWithReadiness(nil, controller)
	now := time.Now().UTC()
	backing := registry.Backing{
		Spec:       hostingservice.Spec{ID: "svc.registry", Type: "http", Owner: "work.registry", Mode: "NetworkPublished", Endpoints: []string{server.URL}},
		WorkloadID: "work.registry", Generation: 31, Running: true, StartedAt: now,
	}

	require.NoError(t, services.Observe(context.Background(), []registry.Backing{backing}, now))
	first, ok := services.Readiness(backing.Spec.ID, now)
	require.True(t, ok)
	require.Equal(t, readiness.StateWarming, first.State)
	require.NoError(t, services.Observe(context.Background(), []registry.Backing{backing}, now.Add(time.Millisecond)))
	ready, ok := services.Readiness(backing.Spec.ID, now.Add(time.Millisecond))
	require.True(t, ok)
	require.True(t, ready.Ready)
	require.Equal(t, int64(31), ready.Generation)

	backing.Running = false
	require.NoError(t, services.Observe(context.Background(), []registry.Backing{backing}, now.Add(2*time.Millisecond)))
	inactive, _ := services.Readiness(backing.Spec.ID, now.Add(2*time.Millisecond))
	require.Equal(t, readiness.StateInactive, inactive.State)
	require.False(t, inactive.ExposureEligible)
}
