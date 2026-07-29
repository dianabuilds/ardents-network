//go:build e2e

package workloade2e_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"runtime"
	"testing"
	"time"

	runtimeinfra "ardents/internal/daemon"
	networkapi "ardents/internal/network"
	workloadapi "ardents/internal/workload"
	"ardents/tests/testkit"

	"github.com/stretchr/testify/require"
)

func TestWorkloadReadyHelper(t *testing.T) {
	if os.Getenv("ARDENTS_E2E_READY_HELPER") != "1" {
		return
	}
	generation := os.Getenv("ARDENTS_WORKLOAD_GENERATION")
	server := &http.Server{Addr: os.Getenv("ARDENTS_E2E_READY_ADDRESS"), Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Ardents-Generation", generation)
		w.WriteHeader(http.StatusNoContent)
	})}
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		os.Exit(2)
	}
}

func TestWorkloadHostedServiceLifecycleAcrossNodeRestart(t *testing.T) {
	scenario := testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerE2E,
		Domain:      "workload",
		ScenarioID:  "WKE-001",
		Suite:       "e2e",
		Tags:        []string{"integration", "e2e", "workload"},
		Speed:       "default",
		Environment: "local",
	})
	ctx := context.Background()
	dir := t.TempDir()
	cfg := workloadLifecycleConfig(t, dir)
	var first *runtimeinfra.Node
	var second *runtimeinfra.Node

	scenario.Precondition("start node with workload-backed service", func(t *testing.T) {
		first = testkit.StartNode(t, cfg)
	})

	scenario.Step("assert running state and initial publication", func(t *testing.T) {
		assertReadyRunningPublished(t, first, "initial start")
	})

	scenario.Step("stop node and withdraw publication", func(t *testing.T) {
		stopAndAssertWithdrawal(t, ctx, first)
	})

	scenario.Step("restart node from persisted state", func(t *testing.T) {
		second = testkit.StartNode(t, cfg)
	})

	scenario.Assert("recovered runtime republishes without stale restart marker", func(t *testing.T) {
		recovered := assertReadyRunningPublished(t, second, "restart recovery")
		require.Equal(t, "running", recovered.Observed)
		require.NotContains(t, recovered.Reason, "restart reconciliation after node restart")
	})
}

func TestWorkloadHostedServiceObservedExitWithdrawsPublicationAndDegradesDiagnostics(t *testing.T) {
	scenario := testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerE2E,
		Domain:      "workload",
		ScenarioID:  "WKE-001",
		Suite:       "e2e",
		Tags:        []string{"integration", "e2e", "workload"},
		Speed:       "default",
		Environment: "local",
	})
	cfg := workloadLifecycleConfig(t, t.TempDir())
	var harness *testkit.RuntimeHarness
	var n *runtimeinfra.Node

	scenario.Precondition("start node with workload-backed service", func(t *testing.T) {
		harness = testkit.StartRuntime(t, cfg)
		n = harness.Node
	})

	scenario.Step("assert running state and publication before observed exit", func(t *testing.T) {
		assertReadyRunningPublished(t, n, "before observed exit")
	})

	scenario.Degraded("unexpected workload exit withdraws publication and surfaces degraded diagnostics", func(t *testing.T) {
		killWorkloadProcess(t, harness, "work.echo")

		item := waitForWorkloadObserved(t, n, "work.echo", "degraded")
		require.Contains(t, item.Reason, "restart reconciliation")
		require.Len(t, item.PublishedServices, 1)
		require.False(t, item.PublishedServices[0].Published)
		require.NotEmpty(t, item.PublishedServices[0].Reason)
		diag := testkit.Diagnostics(n).DiagnosticsSnapshot()
		current, currentErr := testkit.Workloads(n).Get("work.echo")
		require.Equalf(t, "degraded", diag.Health.State, "workload=%+v current=%+v current_err=%v diagnostics=%+v", item, current, currentErr, diag.Health)
		require.NotNil(t, diag.Health.PrimaryReason)
		require.Equal(t, "workload.hosted_service.degraded", diag.Health.PrimaryReason.Code)

		res, err := n.ResolveService("echo")
		require.NoError(t, err)
		require.Equal(t, "not_found", res.Outcome)
		require.Empty(t, res.Matches)
	})

	scenario.Assert("operator restart proves fresh readiness before republishing", func(t *testing.T) {
		require.NoError(t, testkit.Workloads(n).Restart(context.Background(), "work.echo"))
		assertReadyRunningPublished(t, n, "crash recovery")
	})
}

//goland:noinspection ALL
func workloadLifecycleConfig(t *testing.T, dir string) runtimeinfra.Config {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("private-interface service reachability is an e2e Linux acceptance scenario")
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := listener.Addr().(*net.TCPAddr).Port
	require.NoError(t, listener.Close())
	host := privateContainerIPv4(t)
	advertised := fmt.Sprintf("http://%s:%d/ready", host, port)
	privacy := testkit.NewDiscoveryPrivacyFixture(t, time.Now().UTC().Truncate(time.Second))
	return runtimeinfra.Config{
		Name: "workload-e2e", NodeProfile: networkapi.NodeProfileServiceNode,
		Boot:      runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Transport: runtimeinfra.TransportConfig{BindAddress: "127.0.0.1", ReachabilityMode: networkapi.ReachabilityPrivateLAN},
		Data:      runtimeinfra.DataConfig{Dir: dir}, Privacy: privacy.Sender,
		DiscoveryRefreshInterval: 50 * time.Millisecond,
		Workload: []runtimeinfra.WorkloadConfig{{
			ID:      "work.echo",
			Kind:    "service",
			Owner:   "node",
			Config:  readyHelperConfig(t, host, port),
			Desired: "running",
			Services: []runtimeinfra.ServiceConfig{{
				ID:        "svc.work.echo",
				Type:      "echo",
				Mode:      "NetworkPublished",
				Endpoints: []string{advertised}, ProbeEndpoints: []string{advertised},
			}},
		}},
	}
}

func readyHelperConfig(t *testing.T, host string, port int) string {
	t.Helper()
	executable, err := os.Executable()
	require.NoError(t, err)
	raw, err := json.Marshal(map[string]any{
		"command": executable,
		"args":    []string{"-test.run=TestWorkloadReadyHelper"},
		"env": map[string]string{"ARDENTS_E2E_READY_HELPER": "1",
			"ARDENTS_E2E_READY_ADDRESS": fmt.Sprintf("%s:%d", host, port)},
	})
	require.NoError(t, err)
	return string(raw)
}

func privateContainerIPv4(t *testing.T) string {
	t.Helper()
	addresses, err := net.InterfaceAddrs()
	require.NoError(t, err)
	for _, address := range addresses {
		ip, _, parseErr := net.ParseCIDR(address.String())
		if parseErr == nil && ip.To4() != nil && ip.IsPrivate() && !ip.IsLoopback() {
			return ip.String()
		}
	}
	t.Fatal("Linux test container has no private IPv4 address")
	return ""
}

func waitForRunningAndPublished(t *testing.T, n *runtimeinfra.Node) workloadapi.StatusSnapshot {
	t.Helper()
	var last workloadapi.StatusSnapshot
	var lastService string
	testkit.WaitForCondition(t, 10*time.Second, "running workload and published service", func() (bool, string) {
		items, err := testkit.Workloads(n).List()
		if err != nil {
			lastService = err.Error()
			return false, err.Error()
		}
		if len(items) != 1 {
			lastService = fmt.Sprintf("workloads=%d", len(items))
			return false, lastService
		}
		last = items[0]
		res, err := n.ResolveService("echo")
		if err != nil {
			lastService = err.Error()
			return false, err.Error()
		}
		lastService = fmt.Sprintf("outcome=%q matches=%d", res.Outcome, len(res.Matches))
		if last.Observed != "running" {
			return false, fmt.Sprintf("observed=%q", last.Observed)
		}
		if len(last.PublishedServices) != 1 || !last.PublishedServices[0].Published {
			return false, fmt.Sprintf("published_services=%d", len(last.PublishedServices))
		}
		if res.Outcome != "usable" && len(res.Matches) == 0 {
			return false, lastService
		}
		return len(res.Matches) == 1, lastService
	})
	require.Equalf(t, "running", last.Observed, "service=%s snapshot=%#v", lastService, last)
	return last
}

func assertReadyRunningPublished(t *testing.T, n *runtimeinfra.Node, stage string) workloadapi.StatusSnapshot {
	t.Helper()
	item := waitForRunningAndPublished(t, n)
	require.Len(t, item.PublishedServices[0].Endpoints, 1)
	client := &http.Client{Timeout: time.Second}
	response, err := client.Get(item.PublishedServices[0].Endpoints[0])
	require.NoError(t, err, "%s", stage)
	require.Equal(t, http.StatusNoContent, response.StatusCode, "%s", stage)
	require.NoError(t, response.Body.Close())
	diag := testkit.Diagnostics(n).DiagnosticsSnapshot()
	require.Equal(t, "ready", diag.Health.State, "%s", stage)
	return item
}

func stopAndAssertWithdrawal(t *testing.T, ctx context.Context, n *runtimeinfra.Node) {
	t.Helper()
	require.NoError(t, n.Stop(ctx))
	testkit.WaitForCondition(t, 10*time.Second, "stopped workload after node stop", func() (bool, string) {
		items, err := testkit.Workloads(n).List()
		if err != nil {
			return false, err.Error()
		}
		if len(items) != 1 {
			return false, fmt.Sprintf("workloads=%d", len(items))
		}
		return items[0].Observed == "stopped", fmt.Sprintf("observed=%q", items[0].Observed)
	})
	testkit.WaitForCondition(t, 10*time.Second, "service withdrawal after node stop", func() (bool, string) {
		res, err := n.ResolveService("echo")
		if err != nil {
			return false, err.Error()
		}
		if res.Outcome == "not_found" && len(res.Matches) == 0 {
			return true, ""
		}
		return false, fmt.Sprintf("outcome=%q matches=%d", res.Outcome, len(res.Matches))
	})
}

func waitForWorkloadObserved(t *testing.T, n *runtimeinfra.Node, workloadID, observed string) workloadapi.StatusSnapshot {
	t.Helper()

	var item workloadapi.StatusSnapshot
	testkit.WaitForCondition(t, 10*time.Second, fmt.Sprintf("%s workload observed=%s", workloadID, observed), func() (bool, string) {
		current, err := testkit.Workloads(n).Get(workloadID)
		if err != nil {
			return false, err.Error()
		}
		item = current
		return current.Observed == observed, fmt.Sprintf("observed=%q reason=%q", current.Observed, current.Reason)
	})
	return item
}

func killWorkloadProcess(t *testing.T, harness *testkit.RuntimeHarness, workloadID string) {
	t.Helper()

	status, ok := testkit.WorkloadStatusForIntegrationTest(harness, workloadID)
	require.Truef(t, ok, "expected internal status for workload %q", workloadID)
	pid := status.Instance.PID
	require.NotZero(t, pid)

	proc, err := os.FindProcess(pid)
	require.NoError(t, err)
	require.NoError(t, proc.Kill())
}
