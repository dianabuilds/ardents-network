package readiness_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"ardents/internal/workload/readiness"

	"github.com/stretchr/testify/require"
)

func TestControllerRejectsWrongGenerationAndRequiresConsecutiveRecovery(t *testing.T) {
	generation := "6"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Ardents-Generation", generation)
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)

	policy := readiness.Policy{
		Timeout:          100 * time.Millisecond,
		Warmup:           10 * time.Millisecond,
		SuccessThreshold: 2,
		FailureThreshold: 3,
		StaleAfter:       time.Second,
	}
	controller := readiness.NewController(policy)
	now := time.Now().UTC()
	observation := readiness.Observation{
		ServiceID:  "svc.echo",
		WorkloadID: "work.echo",
		Generation: 7,
		Running:    true,
		StartedAt:  now.Add(-policy.Warmup),
		Endpoints:  []string{server.URL},
	}

	wrong := controller.Observe(context.Background(), observation, now)
	require.Equal(t, readiness.StateNotReady, wrong.State)
	require.False(t, wrong.Ready)
	require.Equal(t, readiness.ReasonGenerationMismatch, wrong.Reason)

	generation = "7"
	first := controller.Observe(context.Background(), observation, now.Add(time.Millisecond))
	require.Equal(t, readiness.StateWarming, first.State)
	require.False(t, first.Ready)

	ready := controller.Observe(context.Background(), observation, now.Add(2*time.Millisecond))
	require.Equal(t, readiness.StateReady, ready.State)
	require.True(t, ready.Ready)
	require.True(t, ready.ExposureEligible)
}

func TestControllerNeverReturnsToAnOlderGeneration(t *testing.T) {
	generation := int64(2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Ardents-Generation", generationHeader(generation))
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)

	controller := readiness.NewController(readiness.Policy{Warmup: 0, SuccessThreshold: 1, FailureThreshold: 1})
	now := time.Now().UTC()
	current := readiness.Observation{ServiceID: "svc", WorkloadID: "work", Generation: 2, Running: true,
		StartedAt: now.Add(-time.Minute), Endpoints: []string{server.URL}}
	require.True(t, controller.Observe(context.Background(), current, now).Ready)

	generation = 1
	stale := current
	stale.Generation = 1
	snapshot := controller.Observe(context.Background(), stale, now.Add(time.Second))
	require.False(t, snapshot.Ready)
	require.False(t, snapshot.ExposureEligible)
	require.Equal(t, readiness.StateNotReady, snapshot.State)
	require.Equal(t, readiness.ReasonGenerationMismatch, snapshot.Reason)

	generation = 2
	snapshot = controller.Observe(context.Background(), current, now.Add(2*time.Second))
	require.True(t, snapshot.Ready)
	require.Equal(t, int64(2), snapshot.Generation)
}

func TestControllerReportsBoundedProbeTimeoutAfterWarmup(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.Header().Set("X-Ardents-Generation", "9")
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)

	policy := readiness.Policy{Timeout: 10 * time.Millisecond, Warmup: time.Millisecond, SuccessThreshold: 1, FailureThreshold: 1, StaleAfter: time.Second}
	controller := readiness.NewController(policy)
	now := time.Now().UTC()
	got := controller.Observe(context.Background(), readiness.Observation{
		ServiceID: "svc.slow", WorkloadID: "work.slow", Generation: 9,
		Running: true, StartedAt: now.Add(-policy.Warmup), Endpoints: []string{server.URL},
	}, now)

	require.Equal(t, readiness.StateNotReady, got.State)
	require.Equal(t, readiness.ReasonProbeTimeout, got.Reason)
}

func TestControllerHonorsCallerCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(250 * time.Millisecond)
		w.Header().Set("X-Ardents-Generation", "11")
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)

	controller := readiness.NewController(readiness.Policy{Timeout: time.Second, SuccessThreshold: 1, FailureThreshold: 1})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	started := time.Now()
	got := controller.Observe(ctx, readiness.Observation{
		ServiceID: "svc.cancel", WorkloadID: "work.cancel", Generation: 11,
		Running: true, StartedAt: started.Add(-time.Second), Endpoints: []string{server.URL},
	}, started)

	require.Less(t, time.Since(started), 100*time.Millisecond)
	require.Equal(t, readiness.ReasonProbeTimeout, got.Reason)
}

func TestControllerContainsFlappingAndResetsChangedIdentity(t *testing.T) {
	var generation atomic.Int64
	generation.Store(21)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Ardents-Generation", generationHeader(generation.Load()))
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)

	policy := readiness.Policy{Timeout: 100 * time.Millisecond, SuccessThreshold: 2, FailureThreshold: 3, StaleAfter: 20 * time.Millisecond}
	controller := readiness.NewController(policy)
	now := time.Now().UTC()
	observation := readiness.Observation{ServiceID: "svc.flap", WorkloadID: "work.flap", Generation: 21, Running: true, StartedAt: now, Endpoints: []string{server.URL}}

	require.False(t, controller.Observe(context.Background(), observation, now).Ready)
	require.True(t, controller.Observe(context.Background(), observation, now.Add(time.Millisecond)).Ready)
	generation.Store(20)
	firstFailure := controller.Observe(context.Background(), observation, now.Add(2*time.Millisecond))
	require.Equal(t, readiness.StateDegraded, firstFailure.State)
	require.True(t, firstFailure.ExposureEligible)
	require.True(t, controller.Observe(context.Background(), observation, now.Add(3*time.Millisecond)).Ready)
	thirdFailure := controller.Observe(context.Background(), observation, now.Add(4*time.Millisecond))
	require.Equal(t, readiness.StateNotReady, thirdFailure.State)
	require.False(t, thirdFailure.ExposureEligible)

	generation.Store(22)
	observation.Generation = 22
	reset := controller.Observe(context.Background(), observation, now.Add(5*time.Millisecond))
	require.Equal(t, readiness.StateWarming, reset.State)
	require.False(t, reset.Ready)
	ready := controller.Observe(context.Background(), observation, now.Add(6*time.Millisecond))
	require.True(t, ready.Ready)
	stale, ok := controller.Snapshot(observation.ServiceID, now.Add(time.Second))
	require.True(t, ok)
	require.Equal(t, readiness.StateStale, stale.State)
	require.False(t, stale.ExposureEligible)

	observation.Running = false
	inactive := controller.Observe(context.Background(), observation, now.Add(2*time.Second))
	require.Equal(t, readiness.StateInactive, inactive.State)
	require.Equal(t, readiness.ReasonRuntimeInactive, inactive.Reason)
}

func generationHeader(generation int64) string {
	return fmt.Sprintf("%d", generation)
}
