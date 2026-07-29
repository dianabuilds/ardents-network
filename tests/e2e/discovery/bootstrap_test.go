//go:build e2e

package discoverye2e_test

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
	networkprivacy "ardents/internal/messaging"
	networkapi "ardents/internal/network"
	"ardents/tests/testkit"

	"github.com/stretchr/testify/require"
)

func TestDiscoveryReadyHelper(t *testing.T) {
	if os.Getenv("ARDENTS_DISCOVERY_READY_HELPER") != "1" {
		return
	}
	generation := os.Getenv("ARDENTS_WORKLOAD_GENERATION")
	server := &http.Server{Addr: os.Getenv("ARDENTS_DISCOVERY_READY_ADDRESS"), Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Ardents-Generation", generation)
		w.WriteHeader(http.StatusNoContent)
	})}
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		os.Exit(2)
	}
}

func TestDiscoveryBootstrapsFromPeerTransport(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("private-interface service reachability is an e2e Linux acceptance scenario")
	}
	testkit.BeginScenario(t, testkit.Spec{
		Layer: testkit.LayerE2E, Domain: "discovery", ScenarioID: "DKE-001", Suite: "e2e",
		Tags: []string{"integration", "e2e", "discovery"}, Speed: "default", Environment: "linux-container",
	})
	privacy := testkit.NewDiscoveryPrivacyFixture(t, time.Now().UTC().Truncate(time.Second))
	remote := testkit.StartNode(t, discoveryServiceConfig(t, "remote", t.TempDir(), privacy.Sender, "work.remote.echo", "svc.remote.echo"))
	local := testkit.StartNode(t, discoveryClientConfig(t, "local", t.TempDir(), privacy.Receiver, testkit.BootstrapEndpoints(t, remote)))

	result := testkit.WaitForServiceMatchCount(t, 10*time.Second, local, "echo", 1)
	require.NotEmpty(t, result.Matches)
	serviceRecord := result.Matches[0].Record
	require.NotNil(t, serviceRecord.Service)
	require.Nil(t, serviceRecord.Node)
	require.Equal(t, "svc.remote.echo", serviceRecord.Service.ID)
	require.Equal(t, "work.remote.echo", serviceRecord.Service.WorkloadID)
	require.NotEmpty(t, serviceRecord.Service.Endpoints)
	requireServiceEndpoint(t, serviceRecord.Service.Endpoints[0])
	record, err := local.ResolveRecord(remote.Snapshot().Ident.Principal, "node")
	require.NoError(t, err)
	require.Equal(t, "usable", record.Route.Outcome)
	require.NotNil(t, record.Route.Selected)
	require.Equal(t, "multiaddr", record.Route.Selected.Scheme)
	boot := local.Snapshot().Boot
	require.True(t, boot.Joined)
	require.Equal(t, "ready", boot.State)
}

func TestDiscoveryBootstrapDropsWithdrawnRemoteService(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("private-interface service reachability is an e2e Linux acceptance scenario")
	}
	testkit.BeginScenario(t, testkit.Spec{
		Layer: testkit.LayerE2E, Domain: "discovery", ScenarioID: "DKE-001", Suite: "e2e",
		Tags: []string{"integration", "e2e", "discovery"}, Speed: "default", Environment: "linux-container",
	})
	privacy := testkit.NewDiscoveryPrivacyFixture(t, time.Now().UTC().Truncate(time.Second))
	remote := testkit.StartNode(t, discoveryServiceConfig(t, "remote-withdraw", t.TempDir(), privacy.Sender, "work.echo", "svc.work.echo"))
	bootstrap := testkit.BootstrapEndpoints(t, remote)
	first := testkit.StartNode(t, discoveryClientConfig(t, "first-client", t.TempDir(), privacy.Receiver, bootstrap))
	testkit.WaitForServiceMatchCount(t, 10*time.Second, first, "echo", 1)

	require.NoError(t, testkit.Workloads(remote).Stop(context.Background(), "work.echo"))
	second := testkit.StartNode(t, discoveryClientConfig(t, "second-client", t.TempDir(), privacy.Receiver, bootstrap))
	testkit.WaitForServiceMatchCount(t, 10*time.Second, second, "echo", 0)
}

//goland:noinspection ALL
func discoveryServiceConfig(t *testing.T, name, dir string, privacy *networkprivacy.Channel, workloadID, serviceID string) runtimeinfra.Config {
	t.Helper()
	port := reserveDiscoveryPort(t)
	host := privateDiscoveryIPv4(t)
	return runtimeinfra.Config{
		Name: name, NodeProfile: networkapi.NodeProfileServiceNode,
		Boot:      runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Transport: runtimeinfra.TransportConfig{BindAddress: "127.0.0.1", ReachabilityMode: networkapi.ReachabilityPrivateLAN},
		Data:      runtimeinfra.DataConfig{Dir: dir}, Privacy: privacy, DiscoveryRefreshInterval: 50 * time.Millisecond,
		Workload: []runtimeinfra.WorkloadConfig{{ID: workloadID, Kind: "service", Owner: "node",
			Config: discoveryReadyConfig(t, host, port), Desired: "running", Services: []runtimeinfra.ServiceConfig{{
				ID: serviceID, Type: "echo", Mode: "NetworkPublished",
				Endpoints:      []string{fmt.Sprintf("http://%s:%d/ready", host, port)},
				ProbeEndpoints: []string{fmt.Sprintf("http://%s:%d/ready", host, port)},
			}}}},
	}
}

func discoveryClientConfig(t *testing.T, name, dir string, privacy *networkprivacy.Channel, bootstrap []string) runtimeinfra.Config {
	t.Helper()
	return runtimeinfra.Config{Name: name, NodeProfile: networkapi.NodeProfileServiceNode,
		Boot:      runtimeinfra.BootConfig{Sources: bootstrap},
		Transport: runtimeinfra.TransportConfig{BindAddress: "127.0.0.1", ReachabilityMode: networkapi.ReachabilityPrivateLAN},
		Data:      runtimeinfra.DataConfig{Dir: dir}, Privacy: privacy, DiscoveryRefreshInterval: 50 * time.Millisecond}
}

func discoveryReadyConfig(t *testing.T, host string, port int) string {
	t.Helper()
	executable, err := os.Executable()
	require.NoError(t, err)
	raw, err := json.Marshal(map[string]any{"command": executable, "args": []string{"-test.run=TestDiscoveryReadyHelper"},
		"env": map[string]string{"ARDENTS_DISCOVERY_READY_HELPER": "1", "ARDENTS_DISCOVERY_READY_ADDRESS": fmt.Sprintf("%s:%d", host, port)}})
	require.NoError(t, err)
	return string(raw)
}

func reserveDiscoveryPort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { require.NoError(t, listener.Close()) }()
	return listener.Addr().(*net.TCPAddr).Port
}

func privateDiscoveryIPv4(t *testing.T) string {
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

func requireServiceEndpoint(t *testing.T, endpoint string) {
	t.Helper()
	client := &http.Client{Timeout: time.Second}
	response, err := client.Get(endpoint)
	require.NoError(t, err)
	defer func() { require.NoError(t, response.Body.Close()) }()
	require.Equal(t, http.StatusNoContent, response.StatusCode)
}
