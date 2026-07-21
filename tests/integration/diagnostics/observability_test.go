//go:build integration

package diagnostics_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	runtimeprocess "ardents/internal/daemon"
	"ardents/internal/diagnostics"
	diagapi "ardents/internal/diagnostics"
	"ardents/internal/observability"
	"ardents/tests/testkit"

	"github.com/stretchr/testify/require"
)

func TestProductionObservabilityCorrelatesCanonicalSuccessAndFailure(t *testing.T) {
	scenario := testkit.BeginScenario(t, testkit.Spec{
		Layer: testkit.LayerIntegration, Domain: "diagnostics", ScenarioID: "OBS-001",
		Suite: "integration", Tags: []string{"integration", "observability", "security", "degraded"},
		Speed: "fast", Environment: "linux-container",
	})
	node := testkit.StartNode(t, runtimeprocess.Config{
		Name: "observability-node", Boot: runtimeprocess.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeprocess.DataConfig{Dir: t.TempDir()},
	})
	owners, ok := runtimeprocess.OwnersFor(node)
	require.True(t, ok)
	surface, err := observability.NewSurface(observability.Dependencies{
		Runtime: node, Diagnostics: testkit.Diagnostics(node), Workloads: owners.Workloads, Hosting: owners.Hosting,
		Data: testkit.Content(node), Transfers: testkit.Transfers(node),
	}, "")
	require.NoError(t, err)
	server := httptest.NewServer(surface.Handler())
	t.Cleanup(server.Close)

	scenario.Step("observe canonical healthy readiness and metrics", func(t *testing.T) {
		status, body := getObservability(t, server.URL+"/readyz")
		require.Equal(t, http.StatusOK, status)
		require.Contains(t, body, `"status":"ready"`)
		_, metrics := getObservability(t, server.URL+"/metrics")
		require.Contains(t, metrics, "ardents_node_ready 1")
		require.Contains(t, metrics, `ardents_node_health{state="ready"} 1`)
	})

	scenario.Degraded("inject Diagnostics-owned degradation and a sensitive denial event", func(t *testing.T) {
		reason := &diagnostics.Reason{Code: "observability.injected", Domain: "diagnostics", Summary: "injected degradation"}
		node.DiagnosticsRecorder().SetSubsystem("diagnostics", diagnostics.HealthDegraded, reason)
		node.DiagnosticsRecorder().SetPrimary(diagnostics.HealthDegraded, reason)
		node.DiagnosticsRecorder().RecordEventCommand(diagapi.RecordEventCommand{
			Domain: "policy", Type: "denied", Resource: "blob-sensitive-id",
			Payload: map[string]any{"action": "route.use", "token": "scrape-secret-value"},
		})
	})

	scenario.Assert("readiness and redacted metrics expose the same degraded truth", func(t *testing.T) {
		status, body := getObservability(t, server.URL+"/readyz")
		require.Equal(t, http.StatusServiceUnavailable, status)
		require.Contains(t, body, `"health":"degraded"`)
		_, metrics := getObservability(t, server.URL+"/metrics")
		require.Contains(t, metrics, `ardents_node_health{state="degraded"} 1`)
		require.Contains(t, metrics, `ardents_policy_denials_window{action="route"} 1`)
		require.NotContains(t, metrics, "blob-sensitive-id")
		require.NotContains(t, metrics, "scrape-secret-value")
	})
}

func getObservability(t *testing.T, url string) (int, string) {
	t.Helper()
	response, err := http.Get(url)
	require.NoError(t, err)
	defer func() { require.NoError(t, response.Body.Close()) }()
	body, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	require.NoError(t, err)
	return response.StatusCode, strings.TrimSpace(string(body))
}
