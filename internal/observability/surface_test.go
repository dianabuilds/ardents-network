package observability

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	appdata "ardents/internal/content"
	daemonruntime "ardents/internal/daemon"
	diagapi "ardents/internal/diagnostics"
	"ardents/internal/discovery"
	"ardents/internal/hosting"
	"ardents/internal/network"
	"ardents/internal/transfer"
	workloadapi "ardents/internal/workload"

	"github.com/stretchr/testify/require"
)

func TestSurfaceProjectsCanonicalReadyMetricsWithoutResourceLabels(t *testing.T) {
	source := populatedSource()
	surface, err := NewSurface(testDependencies(source), "")
	require.NoError(t, err)

	ready := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	request.Header.Set(correlationHeader, "operator-check-1")
	surface.Handler().ServeHTTP(ready, request)
	require.Equal(t, http.StatusOK, ready.Code)
	require.Equal(t, "operator-check-1", ready.Header().Get(correlationHeader))
	require.JSONEq(t, `{"status":"ready","state":"ready","health":"ready"}`, ready.Body.String())

	metrics := scrape(t, surface, "")
	require.Contains(t, metrics, `ardents_node_ready 1`)
	require.Contains(t, metrics, `ardents_waku_protocol_active{protocol="relay"} 1`)
	require.Contains(t, metrics, `ardents_waku_store_messages 90`)
	require.Contains(t, metrics, `ardents_waku_store_capacity_messages 100`)
	require.Contains(t, metrics, `ardents_waku_store_capacity_bytes 8192`)
	require.Contains(t, metrics, `ardents_waku_store_file_bytes 4096`)
	require.Contains(t, metrics, `ardents_waku_store_usage_ratio 0.9`)
	require.Contains(t, metrics, `ardents_workload_resource_limits{resource="memory_bytes"} 512`)
	require.Contains(t, metrics, `ardents_policy_denials_window{action="route"} 1`)
	require.NotContains(t, metrics, "peer-secret-id")
	require.NotContains(t, metrics, "workload-secret-id")
	require.NotContains(t, metrics, "blob-secret-id")
	require.NotContains(t, metrics, "selector-secret")
}

func TestSurfaceCorrelatesDegradedReadinessAndFailureMetrics(t *testing.T) {
	source := populatedSource()
	source.runtime.Node.Ready = false
	source.runtime.Node.State = "degraded"
	source.runtime.Health.State = "degraded"
	surface, err := NewSurface(testDependencies(source), "scrape-secret")
	require.NoError(t, err)

	ready := httptest.NewRecorder()
	surface.Handler().ServeHTTP(ready, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	require.Equal(t, http.StatusServiceUnavailable, ready.Code)
	require.True(t, validCorrelationID.MatchString(ready.Header().Get(correlationHeader)))

	unauthorized := httptest.NewRecorder()
	surface.Handler().ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	require.Equal(t, http.StatusUnauthorized, unauthorized.Code)

	metrics := scrape(t, surface, "scrape-secret")
	require.Contains(t, metrics, `ardents_node_health{state="degraded"} 1`)
	require.Contains(t, metrics, `ardents_privacy_failures_window{category="other",domain="data"} 1`)
}

func TestRequestLogUsesNormalizedRouteAndSafeCorrelation(t *testing.T) {
	var output bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&output, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	surface, err := NewSurface(testDependencies(populatedSource()), "")
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/private/blob-secret-id?token=secret", nil)
	request.Header.Set(correlationHeader, "contains spaces")
	surface.Handler().ServeHTTP(recorder, request)

	logText := output.String()
	require.Contains(t, logText, `"route":"unknown"`)
	require.NotContains(t, logText, "blob-secret-id")
	require.NotContains(t, logText, "token=secret")
	require.NotContains(t, logText, "contains spaces")
}

func TestBoundedLabelsPreserveCanonicalLifecycleAndWorkloadVocabulary(t *testing.T) {
	require.Equal(t, "starting", lifecycleState("starting"))
	require.Equal(t, "accepted", workloadState("accepted"))
	require.Equal(t, "preparing", workloadState("preparing"))
	require.Equal(t, "removed", workloadState("removed"))
	require.Equal(t, "other", workloadState("resource-id-not-a-state"))
}

func scrape(t *testing.T, surface *Surface, token string) string {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	surface.Handler().ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)
	body, err := io.ReadAll(recorder.Result().Body)
	require.NoError(t, err)
	return string(body)
}

func testDependencies(source *fakeSource) Dependencies {
	return Dependencies{Runtime: source, Diagnostics: source, Workloads: fakeWorkloads{source}, Hosting: source, Data: source, Transfers: source}
}

type fakeSource struct {
	runtime     daemonruntime.RuntimeSnapshot
	network     network.StatusSnapshot
	peers       []discovery.PeerSnapshot
	diagnostics diagapi.DiagSnapshot
	workloads   []workloadapi.StatusSnapshot
	services    []hosting.ServiceSnapshot
	inventory   appdata.InventorySnapshot
	transfers   []transfer.Record
}

func populatedSource() *fakeSource {
	return &fakeSource{
		runtime: daemonruntime.RuntimeSnapshot{Node: daemonruntime.NodeSnapshot{State: "ready", Ready: true}, Health: diagapi.HealthSnapshot{State: "ready"}},
		network: network.StatusSnapshot{
			ActiveFeatures:        []network.TransportFeature{network.TransportFeatureRelay, network.TransportFeatureStore},
			RateLimitedOperations: 2, StoreEnabled: true, StoreMessages: 90,
			StoreCapacityMessages: 100, StoreFileBytes: 4096, StoreUsageRatio: 0.9,
			StoreCapacityBytes: 8192,
		},
		peers: []discovery.PeerSnapshot{{NodeID: "peer-secret-id", State: "connected", Trust: discovery.TrustSnapshot{Valid: true, Trusted: true, Usable: true}}},
		diagnostics: diagapi.DiagSnapshot{RecentEvents: []diagapi.EventEnvelope{
			{Domain: "policy", Type: "denied", Resource: "blob-secret-id", Payload: map[string]any{"action": "route.use", "selector": "selector-secret"}},
			{Domain: "data", Type: "privacy_degraded", Resource: "blob-secret-id"},
			{Domain: "data", Type: "replica_repaired", Resource: "blob-secret-id"},
		}},
		workloads: []workloadapi.StatusSnapshot{{Spec: workloadapi.SpecSnapshot{ID: "workload-secret-id"}, Observed: "running", Instance: workloadapi.InstanceSnapshot{MemoryLimitBytes: 512, NanoCPUs: 100, PIDsLimit: 10, Restarts: 2}}},
		services:  []hosting.ServiceSnapshot{{ID: "service-secret-id", Readiness: "ready", Ready: true}},
		inventory: appdata.InventorySnapshot{Blobs: 1, LocalBytes: 512},
		transfers: []transfer.Record{{ID: "blob-secret-id", State: "running", Direction: "inbound", UpdatedAt: time.Now()}},
	}
}

func (s *fakeSource) GetNodeRuntime() daemonruntime.RuntimeSnapshot { return s.runtime }
func (s *fakeSource) GetNetworkStatus() network.StatusSnapshot      { return s.network }
func (s *fakeSource) ListPeers() []discovery.PeerSnapshot           { return s.peers }
func (s *fakeSource) DiagnosticsSnapshot() diagapi.DiagSnapshot     { return s.diagnostics }

type fakeWorkloads struct{ source *fakeSource }

func (s fakeWorkloads) List() ([]workloadapi.StatusSnapshot, error) {
	return s.source.workloads, nil
}
func (s *fakeSource) ListHostedServices() ([]hosting.ServiceSnapshot, error) {
	return s.services, nil
}
func (s *fakeSource) InventorySnapshot() appdata.InventorySnapshot { return s.inventory }
func (s *fakeSource) List() []transfer.Record                      { return s.transfers }
