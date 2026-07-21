//go:build integration

package hosting_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"sync/atomic"
	"testing"
	"time"

	runtimeprocess "ardents/internal/daemon"
	"ardents/internal/workload/readiness"
	"ardents/internal/workload/registry"
	hostingservice "ardents/internal/workload/registry"
	"ardents/tests/testkit"

	"github.com/stretchr/testify/require"
)

//goland:noinspection ALL
func TestHostedServicePublicTruthRequiresActualWorkloadListener(t *testing.T) {
	if os.Getenv("ARDENTS_READINESS_HELPER") == "1" {
		return
	}
	testkit.BeginScenario(t, testkit.Spec{Layer: testkit.LayerIntegration, Domain: "hosted-services", ScenarioID: "HSI-001",
		Suite: "integration", Tags: []string{"integration", "hosted-services", "readiness", "workload"}, Speed: "default", Environment: "local"})
	address := reserveAddress(t)
	runtime := testkit.StartRuntime(t, runtimeprocess.Config{Name: "hosted-readiness-runtime",
		Boot: runtimeprocess.BootConfig{Sources: []string{"local://bootstrap"}}, Data: runtimeprocess.DataConfig{Dir: t.TempDir()},
		Workload: []runtimeprocess.WorkloadConfig{{ID: "work.ready", Kind: "service", Owner: "node", Desired: "running",
			Config: readinessHelperConfig(t, address), Services: []runtimeprocess.ServiceConfig{{ID: "svc.ready", Type: "http",
				Mode: "NetworkPublished", Endpoints: []string{"http://" + address + "/ready"}}}}}})

	var serviceReady bool
	for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); {
		services, err := runtime.Hosting.ListHostedServices()
		require.NoError(t, err)
		if len(services) == 1 && services[0].Ready {
			serviceReady = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	require.True(t, serviceReady)
	status, err := runtime.Hosting.GetHostedService("svc.ready")
	require.NoError(t, err)
	require.True(t, status.Ready)
	require.True(t, status.ExposureEligible)
	require.NotZero(t, status.Generation)

	require.NoError(t, runtime.Workload.Stop(context.Background(), "work.ready"))
	stopped, err := runtime.Hosting.GetHostedService("svc.ready")
	require.NoError(t, err)
	require.Equal(t, readiness.StateInactive, stopped.State)
	require.False(t, stopped.Ready)
}

func TestReadinessWorkloadHelper(t *testing.T) {
	if os.Getenv("ARDENTS_READINESS_HELPER") != "1" {
		return
	}
	generation := os.Getenv("ARDENTS_WORKLOAD_GENERATION")
	server := &http.Server{Addr: os.Getenv("ARDENTS_READY_ADDRESS"), Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Ardents-Generation", generation)
		w.WriteHeader(http.StatusNoContent)
	})}
	if err := server.ListenAndServe(); err != nil {
		t.Fatal(err)
	}
}

func readinessHelperConfig(t *testing.T, address string) string {
	t.Helper()
	raw, err := json.Marshal(map[string]any{"command": os.Args[0], "args": []string{"-test.run=TestReadinessWorkloadHelper", "-test.count=1"},
		"env": map[string]string{"ARDENTS_READINESS_HELPER": "1", "ARDENTS_READY_ADDRESS": address}})
	require.NoError(t, err)
	return string(raw)
}

//goland:noinspection ALL
func TestHostedServiceReadinessTracksRealGenerationBoundListener(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{Layer: testkit.LayerIntegration, Domain: "hosted-services", ScenarioID: "HSI-001",
		Suite: "integration", Tags: []string{"integration", "hosted-services", "readiness"}, Speed: "default", Environment: "local"})

	address := reserveAddress(t)
	endpoint := "http://" + address + "/ready"
	policy := readiness.Policy{Timeout: 100 * time.Millisecond, Warmup: 10 * time.Millisecond, SuccessThreshold: 2, FailureThreshold: 2, StaleAfter: time.Second}
	now := time.Now().UTC()
	backing := registry.Backing{Spec: hostingservice.ServiceSpec{ID: "svc.integration", Type: "http", Owner: "work.integration",
		Mode: "NetworkPublished", Endpoints: []string{endpoint}}, WorkloadID: "work.integration", Generation: 41, Running: true, StartedAt: now}
	services := registry.NewWithReadiness(nil, readiness.NewController(policy))

	require.NoError(t, services.Observe(context.Background(), []registry.Backing{backing}, now))
	warming, _ := services.Readiness(backing.Spec.ID, now)
	require.Equal(t, readiness.StateWarming, warming.State)
	require.NoError(t, services.Observe(context.Background(), []registry.Backing{backing}, now.Add(policy.Warmup)))
	missing, _ := services.Readiness(backing.Spec.ID, now.Add(policy.Warmup))
	require.Equal(t, readiness.StateNotReady, missing.State)

	var servedGeneration atomic.Int64
	servedGeneration.Store(41)
	stop := startGenerationServer(t, address, &servedGeneration)
	defer stop()
	require.NoError(t, services.Observe(context.Background(), []registry.Backing{backing}, now.Add(20*time.Millisecond)))
	require.NoError(t, services.Observe(context.Background(), []registry.Backing{backing}, now.Add(21*time.Millisecond)))
	ready, _ := services.Readiness(backing.Spec.ID, now.Add(21*time.Millisecond))
	require.True(t, ready.Ready)
	require.True(t, ready.ExposureEligible)

	servedGeneration.Store(40)
	require.NoError(t, services.Observe(context.Background(), []registry.Backing{backing}, now.Add(22*time.Millisecond)))
	degraded, _ := services.Readiness(backing.Spec.ID, now.Add(22*time.Millisecond))
	require.Equal(t, readiness.StateDegraded, degraded.State)
	require.NoError(t, services.Observe(context.Background(), []registry.Backing{backing}, now.Add(23*time.Millisecond)))
	wrong, _ := services.Readiness(backing.Spec.ID, now.Add(23*time.Millisecond))
	require.Equal(t, readiness.ReasonGenerationMismatch, wrong.Reason)
	require.False(t, wrong.Ready)

	servedGeneration.Store(41)
	recovered := registry.NewWithReadiness(nil, readiness.NewController(policy))
	require.NoError(t, recovered.Observe(context.Background(), []registry.Backing{backing}, now.Add(24*time.Millisecond)))
	firstRecovery, _ := recovered.Readiness(backing.Spec.ID, now.Add(24*time.Millisecond))
	require.False(t, firstRecovery.Ready)
	require.NoError(t, recovered.Observe(context.Background(), []registry.Backing{backing}, now.Add(25*time.Millisecond)))
	secondRecovery, _ := recovered.Readiness(backing.Spec.ID, now.Add(25*time.Millisecond))
	require.True(t, secondRecovery.Ready)

	backing.Spec.Endpoints = []string{endpoint + "?revision=2"}
	require.NoError(t, recovered.Observe(context.Background(), []registry.Backing{backing}, now.Add(26*time.Millisecond)))
	changed, _ := recovered.Readiness(backing.Spec.ID, now.Add(26*time.Millisecond))
	require.Equal(t, readiness.StateWarming, changed.State)
	require.False(t, changed.Ready)
	require.NoError(t, recovered.Observe(context.Background(), []registry.Backing{backing}, now.Add(27*time.Millisecond)))
	changedReady, _ := recovered.Readiness(backing.Spec.ID, now.Add(27*time.Millisecond))
	require.True(t, changedReady.Ready)
}

func reserveAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	address := listener.Addr().String()
	require.NoError(t, listener.Close())
	return address
}

func startGenerationServer(t *testing.T, address string, generation *atomic.Int64) func() {
	t.Helper()
	listener, err := net.Listen("tcp", address)
	require.NoError(t, err)
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Ardents-Generation", fmt.Sprintf("%d", generation.Load()))
		w.WriteHeader(http.StatusNoContent)
	})}
	go func() { _ = server.Serve(listener) }()
	return func() { _ = server.Close() }
}
