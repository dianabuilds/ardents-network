//go:build integration

package network_test

import (
	"testing"
	"time"

	runtimeprocess "ardents/internal/daemon"
	networkapi "ardents/internal/network"
	"ardents/tests/testkit"

	"github.com/stretchr/testify/require"
)

func TestPublicReachabilityGatesAndWithdrawsNodeAdvertisement(t *testing.T) {
	scenario := testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "network-foundation",
		ScenarioID:  "NFI-004",
		Suite:       "integration",
		Tags:        []string{"integration", "network", "reachability", "autonat"},
		Speed:       "default",
		Environment: "local",
	})

	var runtime *testkit.RuntimeHarness
	var node *runtimeprocess.Node
	scenario.Precondition("start a public-direct node with an unverified advertised address", func(t *testing.T) {
		runtime = testkit.NewRuntime(t, runtimeprocess.Config{
			Name:        "reachability-gate",
			NodeProfile: networkapi.NodeProfileServiceNode,
			Data:        runtimeprocess.DataConfig{Dir: t.TempDir()},
			Transport: runtimeprocess.TransportConfig{
				BindAddress:        "0.0.0.0",
				ReachabilityMode:   networkapi.ReachabilityPublicDirect,
				AdvertiseAddresses: []string{"/dns4/node.example/tcp/61000"},
			},
		})
		require.NoError(t, runtime.Runtime.Start(t.Context()))
		t.Cleanup(func() { _ = runtime.Runtime.Stop(t.Context()) })
		node = runtime.Runtime
		require.False(t, runtime.Runtime.GetNetworkStatus().Reachable)
		require.Empty(t, localNodeEndpoints(t, runtime.Runtime))
	})

	scenario.Step("peer dialback verification publishes the direct address", func(t *testing.T) {
		require.NoError(t, runtimeprocess.SetReachabilityForIntegrationTest(node, "public"))
		require.Eventually(t, func() bool {
			status := runtime.Runtime.GetNetworkStatus()
			return status.Reachable && status.ReachabilityState == "public" &&
				len(localNodeEndpoints(t, runtime.Runtime)) == 1
		}, 5*time.Second, 25*time.Millisecond)
	})

	scenario.Degraded("private NAT observation withdraws the address and explains degradation", func(t *testing.T) {
		require.NoError(t, runtimeprocess.SetReachabilityForIntegrationTest(node, "private"))
		require.Eventually(t, func() bool {
			status := runtime.Runtime.GetNetworkStatus()
			return !status.Reachable && status.ReachabilityState == "nat_blocked" &&
				len(localNodeEndpoints(t, runtime.Runtime)) == 0
		}, 5*time.Second, 25*time.Millisecond)
	})
}

func localNodeEndpoints(t *testing.T, runtime *runtimeprocess.Node) []string {
	t.Helper()
	records, err := runtime.ListRecords()
	require.NoError(t, err)
	for _, record := range records {
		if record.Kind() == "node" && record.Source == "local" {
			return record.EndpointList()
		}
	}
	return nil
}
