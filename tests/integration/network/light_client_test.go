//go:build integration

package network_test

import (
	"testing"
	"time"

	runtimeprocess "ardents/internal/daemon"
	networkprivacy "ardents/internal/messaging"
	networkapi "ardents/internal/network"
	networkwaku "ardents/internal/network/waku"
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
	fixture := testkit.NewDiscoveryPrivacyFixture(t, now)
	senderChannel, receiverChannel := fixture.Sender, fixture.Receiver
	var provider, client networkapi.Service
	var received <-chan networkapi.Envelope
	var sealed networkprivacy.SealedEnvelope
	payload := []byte("private-light-client-payload")

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
		require.ElementsMatch(t, []networkapi.TransportFeature{networkapi.TransportFeatureFilterClient, networkapi.TransportFeatureLightpushClient, networkapi.TransportFeatureStoreClient}, client.ProfileSnapshot().ActiveFeatures)
	})

	scenario.Step("subscribe with an opaque capability-derived Filter selector", func(t *testing.T) {
		contentTopic, err := receiverChannel.ContentTopic()
		require.NoError(t, err)
		received, err = client.SubscribeFilterEnvelopes(ctx, provider.Endpoints(), contentTopic)
		require.NoError(t, err)
	})

	scenario.Step("encrypt and publish through Lightpush without claiming end delivery from its acknowledgement", func(t *testing.T) {
		var err error
		sealed, err = senderChannel.Seal(networkprivacy.MessageClassDiscoveryRecord, 1, payload)
		require.NoError(t, err)
		require.NoError(t, client.PublishLightpushEnvelope(ctx, provider.Endpoints()[0], carrierEnvelope(sealed)))
	})

	scenario.Assert("Filter delivers ciphertext and the authorized client opens it", func(t *testing.T) {
		select {
		case item, ok := <-received:
			require.True(t, ok)
			require.NotContains(t, item.Payload, payload)
			opened, err := receiverChannel.Open(sealedEnvelope(item))
			require.NoError(t, err)
			require.Equal(t, payload, opened.Payload)
		case <-ctx.Done():
			require.FailNow(t, "timed out waiting for Filter delivery")
		}
	})

	scenario.Assert("Store fallback can recover the retained opaque envelope", func(t *testing.T) {
		testkit.WaitForCondition(t, 5*time.Second, "Store recovery after Filter delivery", func() (bool, string) {
			items, err := client.FetchEnvelopes(ctx, provider.Endpoints(), sealed.ContentTopic)
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
		require.ElementsMatch(t, []networkapi.TransportFeature{networkapi.TransportFeatureFilterClient, networkapi.TransportFeatureLightpushClient, networkapi.TransportFeatureStoreClient}, status.ActiveFeatures)
		require.ElementsMatch(t, status.ActiveFeatures, client.Snapshot().Transport.ActiveFeatures)
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
		client = networkwaku.New(networkapi.Config{
			NodeProfile: networkapi.NodeProfileConstrainedClient,
			Profile:     networkapi.ProfileTCPOnly, ReachabilityMode: networkapi.ReachabilityOutboundOnly,
			BindAddress: "127.0.0.1",
		})
		client.SetBootstrapNodes(incomplete.Endpoints())
		require.NoError(t, client.Start(ctx))
		t.Cleanup(func() { require.NoError(t, client.Stop(ctx)) })
		require.False(t, client.BootstrapStatus().Joined)
		require.Equal(t, "degraded", client.State())
		require.Contains(t, client.Reason(), "do not provide required Filter, Lightpush, and Store protocols")
		require.NotContains(t, client.ProfileSnapshot().ActiveFeatures, networkapi.TransportFeatureFilterClient)
		require.NotContains(t, client.ProfileSnapshot().ActiveFeatures, networkapi.TransportFeatureStoreClient)
	})
}
