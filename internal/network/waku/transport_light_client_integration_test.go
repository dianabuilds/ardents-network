//go:build integration

package waku

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"ardents/internal/network"

	"github.com/stretchr/testify/require"
)

func TestConstrainedClientUsesFilterLightpushAndStoreRecovery(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	provider := startMessagingRoleNode(t, network.NodeProfileServiceNode)
	client := New(network.Config{
		NodeProfile: network.NodeProfileConstrainedClient,
		StorePath:   filepath.Join(t.TempDir(), "client-store.db"), PrivateKeyPath: filepath.Join(t.TempDir(), "client-key.json"),
		BindAddress: "127.0.0.1", Profile: network.ProfileTCPOnly,
		ReachabilityMode: network.ReachabilityOutboundOnly,
	})
	client.SetBootstrapNodes(provider.Endpoints())
	require.NoError(t, client.Start(ctx))
	t.Cleanup(func() { require.NoError(t, client.Stop(context.Background())) })
	require.Equal(t, "ready", client.State())
	require.ElementsMatch(t, []network.TransportFeature{network.TransportFeatureFilterClient, network.TransportFeatureLightpushClient, network.TransportFeatureStoreClient}, client.ProfileSnapshot().ActiveFeatures)

	topic := "ardents/1/private/filter-lightpush"
	received, err := client.SubscribeFilterEnvelopes(ctx, provider.Endpoints(), topic)
	require.NoError(t, err)
	time.Sleep(100 * time.Millisecond)
	envelope := network.Envelope{PubsubTopic: network.DefaultPubsubTopic, ContentTopic: topic, Payload: []byte("ciphertext")}
	require.NoError(t, client.PublishLightpushEnvelope(ctx, provider.Endpoints()[0], envelope))
	select {
	case item := <-received:
		require.Equal(t, envelope.Payload, item.Payload)
	case <-ctx.Done():
		t.Fatal("Filter did not deliver the Lightpush message")
	}

	require.Eventually(t, func() bool {
		items, fetchErr := client.FetchEnvelopes(ctx, provider.Endpoints(), topic)
		return fetchErr == nil && len(items) > 0 && string(items[0].Payload) == "ciphertext"
	}, 5*time.Second, 100*time.Millisecond)
}

func TestNetworkAbuseLimitsBoundOversizedAndLightpushFlood(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	provider := startMessagingRoleNode(t, network.NodeProfileServiceNode)
	client := startConstrainedClient(t, provider, network.Limits{
		MaxMessageBytes: 1024, OperationRate: 1, OperationBurst: 1,
	})
	envelope := network.Envelope{
		PubsubTopic: network.DefaultPubsubTopic, ContentTopic: "ardents/1/private/abuse",
		Payload: make([]byte, 1025),
	}

	err := client.PublishLightpushEnvelope(ctx, provider.Endpoints()[0], envelope)
	require.ErrorContains(t, err, "exceeds 1024 byte limit")
	require.Equal(t, uint64(1), client.AbuseSnapshot().OversizedMessages)

	envelope.Payload = []byte("bounded")
	require.NoError(t, client.PublishLightpushEnvelope(ctx, provider.Endpoints()[0], envelope))
	var rejected int
	for range 5 {
		if err := client.PublishLightpushEnvelope(ctx, provider.Endpoints()[0], envelope); err != nil {
			rejected++
		}
	}
	require.Greater(t, rejected, 0)
	require.Greater(t, client.AbuseSnapshot().RateLimitedOperations, uint64(0))
}

func TestStoreRecoveryCannotAmplifyPastConfiguredResultLimit(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	provider := startMessagingRoleNode(t, network.NodeProfileServiceNode)
	client := startConstrainedClient(t, provider, network.Limits{MaxStoreResults: 3})
	topic := "ardents/1/private/store-limit"

	for i := range 6 {
		envelope := network.Envelope{
			PubsubTopic: network.DefaultPubsubTopic, ContentTopic: topic,
			Payload: []byte{byte(i)},
		}
		require.NoError(t, client.PublishLightpushEnvelope(ctx, provider.Endpoints()[0], envelope))
	}

	require.Eventually(t, func() bool {
		items, err := client.FetchEnvelopes(ctx, provider.Endpoints(), topic)
		return err == nil && len(items) == 3
	}, 7*time.Second, 150*time.Millisecond)
}

func TestFilterServerRejectsSubscriberExhaustion(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	provider := startMessagingRoleNodeWithLimits(t, network.NodeProfileServiceNode, network.Limits{MaxFilterSubscribers: 1})
	first := startConstrainedClient(t, provider, network.Limits{})
	second := startConstrainedClient(t, provider, network.Limits{})
	topic := "ardents/1/private/filter-limit"

	firstCtx, stopFirst := context.WithCancel(ctx)
	defer stopFirst()
	_, err := first.SubscribeFilterEnvelopes(firstCtx, provider.Endpoints(), topic)
	require.NoError(t, err)

	_, err = second.SubscribeFilterEnvelopes(ctx, provider.Endpoints(), topic)
	require.Error(t, err)
}

func TestConnectionChurnStaysWithinPerIPLimit(t *testing.T) {
	provider := startMessagingRoleNodeWithLimits(t, network.NodeProfileServiceNode, network.Limits{
		MaxPeerConnections: 4, MaxConnectionsPerIP: 2,
	})
	for range 5 {
		_ = startConstrainedClient(t, provider, network.Limits{})
	}

	require.Eventually(t, func() bool {
		return provider.PeerCount() <= 2
	}, 5*time.Second, 100*time.Millisecond)
}

func TestRestrictedDefenseRestartRemovesAndRestoresProviderServices(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	provider := startMessagingRoleNode(t, network.NodeProfileServiceNode)

	require.NoError(t, provider.SetModeForIntegration(ctx, network.ModeRestrictedDefense))
	require.Equal(t, []network.TransportFeature{network.TransportFeatureRelay}, provider.ProfileSnapshot().ActiveFeatures)
	require.Contains(t, provider.ProfileSnapshot().ReducedFeatures, network.TransportFeatureStore)
	require.Contains(t, provider.ProfileSnapshot().ReducedFeatures, network.TransportFeatureFilterService)
	require.Contains(t, provider.ProfileSnapshot().ReducedFeatures, network.TransportFeatureLightpushService)
	restrictedClient := startConstrainedClient(t, provider, network.Limits{})
	require.Equal(t, "degraded", restrictedClient.State())

	require.NoError(t, provider.SetModeForIntegration(ctx, network.ModeSteady))
	require.ElementsMatch(t,
		[]string{"relay", "store", "filter_service", "lightpush_service"},
		provider.ProfileSnapshot().ActiveFeatures,
	)
	recoveredClient := startConstrainedClient(t, provider, network.Limits{})
	require.Equal(t, "ready", recoveredClient.State())
}

func startMessagingRoleNode(t *testing.T, profile network.NodeProfile) *Service {
	return startMessagingRoleNodeWithLimits(t, profile, network.Limits{})
}

func startMessagingRoleNodeWithLimits(t *testing.T, profile network.NodeProfile, limits network.Limits) *Service {
	t.Helper()
	dir := t.TempDir()
	svc := New(network.Config{
		NodeProfile: profile, StorePath: filepath.Join(dir, "waku-store.db"), PrivateKeyPath: filepath.Join(dir, "waku-key.json"),
		BindAddress: "127.0.0.1", Profile: network.ProfileTCPOnly,
		ReachabilityMode: network.ReachabilityLocalOnly, Limits: limits,
	})
	require.NoError(t, svc.Start(context.Background()))
	t.Cleanup(func() { require.NoError(t, svc.Stop(context.Background())) })
	return svc
}

func startConstrainedClient(t *testing.T, provider *Service, limits network.Limits) *Service {
	t.Helper()
	dir := t.TempDir()
	client := New(network.Config{
		NodeProfile: network.NodeProfileConstrainedClient,
		StorePath:   filepath.Join(dir, "waku-store.db"), PrivateKeyPath: filepath.Join(dir, "waku-key.json"),
		BindAddress: "127.0.0.1", Profile: network.ProfileTCPOnly,
		ReachabilityMode: network.ReachabilityOutboundOnly, Limits: limits,
	})
	client.SetBootstrapNodes(provider.Endpoints())
	require.NoError(t, client.Start(context.Background()))
	t.Cleanup(func() { require.NoError(t, client.Stop(context.Background())) })
	return client
}
