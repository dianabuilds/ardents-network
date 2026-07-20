//go:build integration

package networkfoundation_test

import (
	"testing"
	"time"

	networkapi "ardents/internal/network/api"
	networkprivacy "ardents/internal/network/privacy"
	runtimeprocess "ardents/internal/runtime/process"
	"ardents/tests/testkit"

	"github.com/stretchr/testify/require"
)

func TestConstrainedClientFilterLightpushAndOfflineRecovery(t *testing.T) {
	scenario := testkit.BeginScenario(t, testkit.Spec{
		Layer: testkit.LayerIntegration, Domain: "network-foundation", ScenarioID: "NFI-005",
		Suite: "integration", Tags: []string{"integration", "network", "filter", "lightpush", "store", "privacy"},
		Speed: "default", Environment: "local",
	})
	ctx := t.Context()
	now := time.Now().UTC().Truncate(time.Second)
	fixture := newRelayPrivacyFixture(t, now)
	senderChannel, receiverChannel := fixture.channels(t, now)
	var provider, client networkapi.Service
	var received <-chan networkprivacy.SealedEnvelope
	var sealed networkprivacy.SealedEnvelope

	scenario.Precondition("start a full Waku provider and a Relay-free constrained client", func(t *testing.T) {
		provider = testkit.StartTransport(t)
		client = testkit.StartTransportWithConfig(t, networkapi.Config{
			NodeProfile: networkapi.NodeProfileConstrainedClient,
			Profile:     networkapi.ProfileTCPOnly, ReachabilityMode: networkapi.ReachabilityOutboundOnly,
		})
		client.SetBootstrapNodes(provider.Endpoints())
		require.NoError(t, client.Stop(ctx))
		require.NoError(t, client.Start(ctx))
		require.Equal(t, "ready", client.State())
		require.Zero(t, client.RelayPeerCount(networkapi.DefaultPubsubTopic))
		require.ElementsMatch(t, []string{"filter_client", "lightpush_client", "store_client"}, client.ProfileSnapshot().ActiveCapabilities)
	})

	scenario.Step("subscribe with an opaque capability-derived Filter selector", func(t *testing.T) {
		contentTopic, err := receiverChannel.ContentTopic()
		require.NoError(t, err)
		received, err = client.SubscribePrivateFilter(ctx, provider.Endpoints(), contentTopic)
		require.NoError(t, err)
	})

	scenario.Step("encrypt and publish through Lightpush without claiming end delivery from its acknowledgement", func(t *testing.T) {
		var err error
		sealed, err = senderChannel.Seal(networkprivacy.MessageClassDiscoveryRecord, 1, fixture.plaintext)
		require.NoError(t, err)
		require.NoError(t, client.PublishPrivateLightpush(ctx, provider.Endpoints()[0], sealed))
	})

	scenario.Assert("Filter delivers ciphertext and the authorized client opens it", func(t *testing.T) {
		select {
		case item, ok := <-received:
			require.True(t, ok)
			require.NotContains(t, item.Payload, fixture.plaintext)
			opened, err := receiverChannel.Open(item)
			require.NoError(t, err)
			require.Equal(t, fixture.plaintext, opened.Payload)
		case <-ctx.Done():
			require.FailNow(t, "timed out waiting for Filter delivery")
		}
	})

	scenario.Assert("Store fallback can recover the retained opaque envelope", func(t *testing.T) {
		testkit.WaitForCondition(t, 5*time.Second, "Store recovery after Filter delivery", func() (bool, string) {
			items, err := client.FetchPrivateEnvelopes(ctx, provider.Endpoints(), sealed.ContentTopic)
			if err != nil || len(items) == 0 {
				if err != nil {
					return false, err.Error()
				}
				return false, "no retained envelope"
			}
			for _, item := range items {
				if string(item.Payload) == string(sealed.Payload) {
					return true, ""
				}
			}
			return false, "retained ciphertext not found"
		})
	})
}

func TestConstrainedClientCapabilitiesReachCanonicalStatus(t *testing.T) {
	scenario := testkit.BeginScenario(t, testkit.Spec{
		Layer: testkit.LayerIntegration, Domain: "network-foundation", ScenarioID: "NFI-005",
		Suite: "integration", Tags: []string{"integration", "network", "light-client", "status"},
		Speed: "default", Environment: "local",
	})
	provider := testkit.StartTransport(t)
	client := runtimeprocess.NewNode(runtimeprocess.Config{
		Name: "light-client-status", NodeProfile: networkapi.NodeProfileConstrainedClient,
		Boot: runtimeprocess.BootConfig{Sources: provider.Endpoints()},
		Data: runtimeprocess.DataConfig{Dir: t.TempDir()},
		Transport: runtimeprocess.TransportConfig{
			Profile: networkapi.ProfileTCPOnly, ReachabilityMode: networkapi.ReachabilityOutboundOnly,
		},
	})

	scenario.Step("start the constrained product runtime against a full provider", func(t *testing.T) {
		require.NoError(t, client.Start(t.Context()))
		t.Cleanup(func() { require.NoError(t, client.Stop(t.Context())) })
	})

	scenario.Assert("canonical status reports observed client capabilities and no inbound claim", func(t *testing.T) {
		status := client.GetNetworkStatus()
		require.True(t, status.Joined)
		require.False(t, status.Reachable)
		require.ElementsMatch(t, []string{"filter_client", "lightpush_client", "store_client"}, status.ActiveCapabilities)
		require.ElementsMatch(t, status.ActiveCapabilities, client.Snapshot().Transport.ActiveCapabilities)
	})
}

func TestConstrainedClientRejectsPeerWithoutRequiredProviderProtocols(t *testing.T) {
	scenario := testkit.BeginScenario(t, testkit.Spec{
		Layer: testkit.LayerIntegration, Domain: "network-foundation", ScenarioID: "NFI-005",
		Suite: "integration", Tags: []string{"integration", "network", "filter", "lightpush", "store", "degraded"},
		Speed: "default", Environment: "local",
	})
	ctx := t.Context()
	var incomplete, client networkapi.Service

	scenario.Precondition("start a peer that does not expose Filter server or Store", func(t *testing.T) {
		incomplete = testkit.StartTransportWithConfig(t, networkapi.Config{
			NodeProfile: networkapi.NodeProfileConstrainedClient,
			Profile:     networkapi.ProfileTCPOnly, ReachabilityMode: networkapi.ReachabilityLocalOnly,
		})
		require.NotEmpty(t, incomplete.Endpoints())
	})

	scenario.Degraded("connect the constrained client but keep required provider protocols absent", func(t *testing.T) {
		client = networkapi.New(networkapi.Config{
			NodeProfile: networkapi.NodeProfileConstrainedClient,
			Profile:     networkapi.ProfileTCPOnly, ReachabilityMode: networkapi.ReachabilityOutboundOnly,
		})
		client.SetBootstrapNodes(incomplete.Endpoints())
		require.NoError(t, client.Start(ctx))
		t.Cleanup(func() { require.NoError(t, client.Stop(ctx)) })
		require.False(t, client.BootstrapStatus().Joined)
		require.Equal(t, "degraded", client.State())
		require.Contains(t, client.Reason(), "do not provide required Filter, Lightpush, and Store protocols")
		require.NotContains(t, client.ProfileSnapshot().ActiveCapabilities, "filter_client")
		require.NotContains(t, client.ProfileSnapshot().ActiveCapabilities, "store_client")
	})
}
