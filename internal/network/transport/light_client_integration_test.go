//go:build integration

package transport

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	networkprivacy "ardents/internal/network/privacy"
	networkreadiness "ardents/internal/network/readiness"

	"github.com/stretchr/testify/require"
)

func TestConstrainedClientUsesFilterLightpushAndStoreRecovery(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	provider := startMessagingRoleNode(t, networkreadiness.NodeProfileServiceNode)
	client := New(Config{
		NodeProfile: networkreadiness.NodeProfileConstrainedClient,
		StorePath:   filepath.Join(t.TempDir(), "client-store.db"), PrivateKeyPath: filepath.Join(t.TempDir(), "client-key.json"),
		BindAddress: "127.0.0.1", Profile: networkreadiness.ProfileTCPOnly,
		ReachabilityMode: networkreadiness.ReachabilityOutboundOnly,
	})
	client.SetBootstrapNodes(provider.Endpoints())
	require.NoError(t, client.Start(ctx))
	t.Cleanup(func() { require.NoError(t, client.Stop(context.Background())) })
	require.Equal(t, "ready", client.State())
	require.ElementsMatch(t, []string{"filter_client", "lightpush_client", "store_client"}, client.ProfileSnapshot().ActiveCapabilities)

	topic := "ardents/1/private/filter-lightpush"
	received, err := client.SubscribePrivateFilter(ctx, provider.Endpoints(), topic)
	require.NoError(t, err)
	time.Sleep(100 * time.Millisecond)
	envelope := networkprivacy.SealedEnvelope{PubsubTopic: networkreadiness.DefaultPubsubTopic(), ContentTopic: topic, Payload: []byte("ciphertext")}
	require.NoError(t, client.PublishPrivateLightpush(ctx, provider.Endpoints()[0], envelope))
	select {
	case item := <-received:
		require.Equal(t, envelope.Payload, item.Payload)
	case <-ctx.Done():
		t.Fatal("Filter did not deliver the Lightpush message")
	}

	require.Eventually(t, func() bool {
		items, fetchErr := client.FetchPrivateEnvelopes(ctx, provider.Endpoints(), topic)
		return fetchErr == nil && len(items) > 0 && string(items[0].Payload) == "ciphertext"
	}, 5*time.Second, 100*time.Millisecond)
}

func TestNetworkAbuseLimitsBoundOversizedAndLightpushFlood(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	provider := startMessagingRoleNode(t, networkreadiness.NodeProfileServiceNode)
	client := startConstrainedClient(t, provider, Limits{
		MaxMessageBytes: 1024, OperationRate: 1, OperationBurst: 1,
	})
	envelope := networkprivacy.SealedEnvelope{
		PubsubTopic: networkreadiness.DefaultPubsubTopic(), ContentTopic: "ardents/1/private/abuse",
		Payload: make([]byte, 1025),
	}

	err := client.PublishPrivateLightpush(ctx, provider.Endpoints()[0], envelope)
	require.ErrorContains(t, err, "exceeds 1024 byte limit")
	require.Equal(t, uint64(1), client.AbuseSnapshot().OversizedMessages)

	envelope.Payload = []byte("bounded")
	require.NoError(t, client.PublishPrivateLightpush(ctx, provider.Endpoints()[0], envelope))
	var rejected int
	for range 5 {
		if err := client.PublishPrivateLightpush(ctx, provider.Endpoints()[0], envelope); err != nil {
			rejected++
		}
	}
	require.Greater(t, rejected, 0)
	require.Greater(t, client.AbuseSnapshot().RateLimitedOperations, uint64(0))
}

func TestStoreRecoveryCannotAmplifyPastConfiguredResultLimit(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	provider := startMessagingRoleNode(t, networkreadiness.NodeProfileServiceNode)
	client := startConstrainedClient(t, provider, Limits{MaxStoreResults: 3})
	topic := "ardents/1/private/store-limit"

	for i := range 6 {
		envelope := networkprivacy.SealedEnvelope{
			PubsubTopic: networkreadiness.DefaultPubsubTopic(), ContentTopic: topic,
			Payload: []byte{byte(i)},
		}
		require.NoError(t, client.PublishPrivateLightpush(ctx, provider.Endpoints()[0], envelope))
	}

	require.Eventually(t, func() bool {
		items, err := client.FetchPrivateEnvelopes(ctx, provider.Endpoints(), topic)
		return err == nil && len(items) == 3
	}, 7*time.Second, 150*time.Millisecond)
}

func TestFilterServerRejectsSubscriberExhaustion(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	provider := startMessagingRoleNodeWithLimits(t, networkreadiness.NodeProfileServiceNode, Limits{MaxFilterSubscribers: 1})
	first := startConstrainedClient(t, provider, Limits{})
	second := startConstrainedClient(t, provider, Limits{})
	topic := "ardents/1/private/filter-limit"

	firstCtx, stopFirst := context.WithCancel(ctx)
	defer stopFirst()
	_, err := first.SubscribePrivateFilter(firstCtx, provider.Endpoints(), topic)
	require.NoError(t, err)

	_, err = second.SubscribePrivateFilter(ctx, provider.Endpoints(), topic)
	require.Error(t, err)
}

func TestConnectionChurnStaysWithinPerIPLimit(t *testing.T) {
	provider := startMessagingRoleNodeWithLimits(t, networkreadiness.NodeProfileServiceNode, Limits{
		MaxPeerConnections: 4, MaxConnectionsPerIP: 2,
	})
	for range 5 {
		_ = startConstrainedClient(t, provider, Limits{})
	}

	require.Eventually(t, func() bool {
		return provider.PeerCount() <= 2
	}, 5*time.Second, 100*time.Millisecond)
}

func TestRestrictedDefenseRestartRemovesAndRestoresProviderServices(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	provider := startMessagingRoleNode(t, networkreadiness.NodeProfileServiceNode)

	require.NoError(t, provider.SetModeForIntegration(ctx, networkreadiness.ModeRestrictedDefense))
	require.Equal(t, []string{"relay"}, provider.ProfileSnapshot().ActiveCapabilities)
	require.Contains(t, provider.ProfileSnapshot().ReducedCapabilities, "store")
	require.Contains(t, provider.ProfileSnapshot().ReducedCapabilities, "filter_service")
	require.Contains(t, provider.ProfileSnapshot().ReducedCapabilities, "lightpush_service")
	restrictedClient := startConstrainedClient(t, provider, Limits{})
	require.Equal(t, "degraded", restrictedClient.State())

	require.NoError(t, provider.SetModeForIntegration(ctx, networkreadiness.ModeSteady))
	require.ElementsMatch(t,
		[]string{"relay", "store", "filter_service", "lightpush_service"},
		provider.ProfileSnapshot().ActiveCapabilities,
	)
	recoveredClient := startConstrainedClient(t, provider, Limits{})
	require.Equal(t, "ready", recoveredClient.State())
}

func startMessagingRoleNode(t *testing.T, profile networkreadiness.NodeProfile) *Service {
	return startMessagingRoleNodeWithLimits(t, profile, Limits{})
}

func startMessagingRoleNodeWithLimits(t *testing.T, profile networkreadiness.NodeProfile, limits Limits) *Service {
	t.Helper()
	dir := t.TempDir()
	svc := New(Config{
		NodeProfile: profile, StorePath: filepath.Join(dir, "waku-store.db"), PrivateKeyPath: filepath.Join(dir, "waku-key.json"),
		BindAddress: "127.0.0.1", Profile: networkreadiness.ProfileTCPOnly,
		ReachabilityMode: networkreadiness.ReachabilityLocalOnly, Limits: limits,
	})
	require.NoError(t, svc.Start(context.Background()))
	t.Cleanup(func() { require.NoError(t, svc.Stop(context.Background())) })
	return svc
}

func startConstrainedClient(t *testing.T, provider *Service, limits Limits) *Service {
	t.Helper()
	dir := t.TempDir()
	client := New(Config{
		NodeProfile: networkreadiness.NodeProfileConstrainedClient,
		StorePath:   filepath.Join(dir, "waku-store.db"), PrivateKeyPath: filepath.Join(dir, "waku-key.json"),
		BindAddress: "127.0.0.1", Profile: networkreadiness.ProfileTCPOnly,
		ReachabilityMode: networkreadiness.ReachabilityOutboundOnly, Limits: limits,
	})
	client.SetBootstrapNodes(provider.Endpoints())
	require.NoError(t, client.Start(context.Background()))
	t.Cleanup(func() { require.NoError(t, client.Stop(context.Background())) })
	return client
}
