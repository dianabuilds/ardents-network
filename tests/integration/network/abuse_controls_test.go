//go:build integration

package network_test

import (
	"context"
	"testing"
	"time"

	networkprivacy "ardents/internal/messaging"
	networkapi "ardents/internal/network"
	"ardents/tests/testkit"

	"github.com/stretchr/testify/require"
)

func TestNetworkAbuseControlsBoundOversizedAndLightpushFlood(t *testing.T) {
	scenario := testkit.BeginScenario(t, testkit.Spec{
		Layer: testkit.LayerIntegration, Domain: "network-foundation", ScenarioID: "NFI-006",
		Suite: "integration", Tags: []string{"integration", "network", "abuse", "lightpush"},
		Speed: "default", Environment: "local",
	})
	provider := testkit.StartTransport(t)
	client := startLimitedClient(t, provider, networkapi.Limits{
		MaxMessageBytes: 1024, OperationRate: 1, OperationBurst: 1,
	})
	envelope := networkprivacy.SealedEnvelope{
		PubsubTopic: networkapi.DefaultPubsubTopic, ContentTopic: "ardents/1/private/abuse",
		Payload: make([]byte, 1025),
	}

	scenario.Step("reject an oversized message before Waku publication", func(t *testing.T) {
		err := client.PublishLightpushEnvelope(t.Context(), provider.Endpoints()[0], carrierEnvelope(envelope))
		require.ErrorContains(t, err, "exceeds 1024 byte limit")
	})

	scenario.Degraded("bound a Lightpush burst and expose rejection counters", func(t *testing.T) {
		envelope.Payload = []byte("bounded")
		require.NoError(t, client.PublishLightpushEnvelope(t.Context(), provider.Endpoints()[0], carrierEnvelope(envelope)))
		for range 5 {
			_ = client.PublishLightpushEnvelope(t.Context(), provider.Endpoints()[0], carrierEnvelope(envelope))
		}
		snapshot := client.AbuseSnapshot()
		require.Equal(t, uint64(1), snapshot.OversizedMessages)
		require.Greater(t, snapshot.RateLimitedOperations, uint64(0))
	})
}

func TestNetworkAbuseControlsBoundStoreAndFilterResources(t *testing.T) {
	scenario := testkit.BeginScenario(t, testkit.Spec{
		Layer: testkit.LayerIntegration, Domain: "network-foundation", ScenarioID: "NFI-006",
		Suite: "integration", Tags: []string{"integration", "network", "store", "filter", "abuse"},
		Speed: "default", Environment: "local",
	})
	provider := testkit.StartTransportWithConfig(t, networkapi.Config{
		NodeProfile: networkapi.NodeProfileServiceNode, Profile: networkapi.ProfileTCPOnly,
		ReachabilityMode: networkapi.ReachabilityLocalOnly,
		Limits:           networkapi.Limits{MaxFilterSubscribers: 1},
	})
	first := startLimitedClient(t, provider, networkapi.Limits{MaxStoreResults: 3})
	second := startLimitedClient(t, provider, networkapi.Limits{})
	topic := "ardents/1/private/resource-limits"

	scenario.Step("retain more messages than one Store query may return", func(t *testing.T) {
		for i := range 6 {
			envelope := networkprivacy.SealedEnvelope{
				PubsubTopic: networkapi.DefaultPubsubTopic, ContentTopic: topic, Payload: []byte{byte(i)},
			}
			require.NoError(t, first.PublishLightpushEnvelope(t.Context(), provider.Endpoints()[0], carrierEnvelope(envelope)))
		}
	})

	scenario.Assert("Store returns no more than the configured result bound", func(t *testing.T) {
		testkit.WaitForCondition(t, 7*time.Second, "bounded Store result", func() (bool, string) {
			items, err := first.FetchEnvelopes(t.Context(), provider.Endpoints(), topic)
			return err == nil && len(items) == 3, "Store result has not reached the bounded size"
		})
	})

	scenario.Degraded("Filter rejects a subscriber beyond provider capacity", func(t *testing.T) {
		_, err := first.SubscribeFilterEnvelopes(t.Context(), provider.Endpoints(), topic)
		require.NoError(t, err)
		_, err = second.SubscribeFilterEnvelopes(t.Context(), provider.Endpoints(), topic)
		require.Error(t, err)
	})
}

func TestNetworkAbuseControlsBoundConnectionChurn(t *testing.T) {
	scenario := testkit.BeginScenario(t, testkit.Spec{
		Layer: testkit.LayerIntegration, Domain: "network-foundation", ScenarioID: "NFI-006",
		Suite: "integration", Tags: []string{"integration", "network", "connections", "churn"},
		Speed: "default", Environment: "local",
	})
	provider := testkit.StartTransportWithConfig(t, networkapi.Config{
		NodeProfile: networkapi.NodeProfileServiceNode, Profile: networkapi.ProfileTCPOnly,
		ReachabilityMode: networkapi.ReachabilityLocalOnly,
		Limits:           networkapi.Limits{MaxPeerConnections: 4, MaxConnectionsPerIP: 2},
	})

	scenario.Step("start five clients behind one source IP", func(t *testing.T) {
		for range 5 {
			_ = startLimitedClient(t, provider, networkapi.Limits{})
		}
	})

	scenario.Assert("the provider remains within its per-IP connection bound", func(t *testing.T) {
		require.LessOrEqual(t, provider.PeerCount(), 2)
	})
}

func TestRestrictedDefenseRemovesAndRestoresProviderServices(t *testing.T) {
	scenario := testkit.BeginScenario(t, testkit.Spec{
		Layer: testkit.LayerIntegration, Domain: "network-foundation", ScenarioID: "NFI-006",
		Suite: "integration", Tags: []string{"integration", "network", "restricted-defense", "recovery"},
		Speed: "default", Environment: "local",
	})
	provider := testkit.StartTransport(t)
	modeControl := provider.(interface {
		SetModeForIntegration(context.Context, networkapi.Mode) error
	})

	scenario.Degraded("restart into Relay-only restricted defense", func(t *testing.T) {
		require.NoError(t, modeControl.SetModeForIntegration(t.Context(), networkapi.ModeRestrictedDefense))
		require.Equal(t, []networkapi.TransportFeature{networkapi.TransportFeatureRelay}, provider.ProfileSnapshot().ActiveFeatures)
		require.Contains(t, provider.ProfileSnapshot().ReducedFeatures, networkapi.TransportFeatureStore)
		restrictedClient := startLimitedClient(t, provider, networkapi.Limits{})
		require.Equal(t, "degraded", restrictedClient.State())
	})

	scenario.Assert("restart back to steady restores provider services", func(t *testing.T) {
		require.NoError(t, modeControl.SetModeForIntegration(t.Context(), networkapi.ModeSteady))
		require.ElementsMatch(t,
			[]networkapi.TransportFeature{
				networkapi.TransportFeatureRelay,
				networkapi.TransportFeatureStore,
				networkapi.TransportFeatureFilterService,
				networkapi.TransportFeatureLightpushService,
			},
			provider.ProfileSnapshot().ActiveFeatures,
		)
		recoveredClient := startLimitedClient(t, provider, networkapi.Limits{})
		require.Equal(t, "ready", recoveredClient.State())
	})
}

func startLimitedClient(t *testing.T, provider networkapi.Service, limits networkapi.Limits) networkapi.Service {
	t.Helper()
	client := testkit.StartTransportWithConfig(t, networkapi.Config{
		NodeProfile: networkapi.NodeProfileConstrainedClient, Profile: networkapi.ProfileTCPOnly,
		ReachabilityMode: networkapi.ReachabilityOutboundOnly, Limits: limits,
	})
	client.SetBootstrapNodes(provider.Endpoints())
	require.NoError(t, client.Stop(t.Context()))
	require.NoError(t, client.Start(t.Context()))
	return client
}
