package waku

import (
	"ardents/internal/network"
	"context"
	"testing"
	"time"

	libp2pnetwork "github.com/libp2p/go-libp2p/core/network"
	"github.com/stretchr/testify/require"
)

func TestReachabilityConfigRequiresPublicCompatibleAddresses(t *testing.T) {
	base := network.Config{Profile: network.ProfileTCPOnly, ReachabilityMode: network.ReachabilityPublicDirect}
	require.ErrorContains(t, validateReachabilityConfig(base), "requires at least one")

	base.AdvertiseAddresses = []string{"/ip4/127.0.0.1/tcp/61000"}
	require.ErrorContains(t, validateReachabilityConfig(base), "public IP or DNS")

	base.AdvertiseAddresses = []string{"/dns4/node.example/tcp/443/tls/ws"}
	require.ErrorContains(t, validateReachabilityConfig(base), "incompatible")

	base.AdvertiseAddresses = []string{"/dns4/node.example/tcp/61000"}
	require.NoError(t, validateReachabilityConfig(base))

	base.AdvertiseAddresses = []string{"/dns4/relay.example/tcp/61000/p2p-circuit"}
	require.ErrorContains(t, validateReachabilityConfig(base), "without a peer ID")
}

func TestReachabilityConfigRequiresOnePrivateLiteralTranslatedHostAddress(t *testing.T) {
	base := network.Config{
		NodeProfile:      network.NodeProfileServiceNode,
		Profile:          network.ProfileTCPOnly,
		ReachabilityMode: network.ReachabilityPrivateLAN,
	}
	require.ErrorContains(t, validateReachabilityConfig(base), "requires exactly one")

	base.AdvertiseAddresses = []string{"/ip4/10.23.0.11/tcp/61000"}
	require.NoError(t, validateReachabilityConfig(base))

	for _, invalid := range [][]string{
		{"/ip4/10.23.0.11/tcp/61000", "/ip4/10.23.0.12/tcp/61000"},
		{"/ip4/127.0.0.1/tcp/61000"},
		{"/ip4/0.0.0.0/tcp/61000"},
		{"/ip4/203.0.113.10/tcp/61000"},
		{"/dns4/node-a.internal/tcp/61000"},
		{"/ip4/10.23.0.11/tcp/61000/p2p-circuit"},
		{"/ip4/10.23.0.11/udp/61000"},
	} {
		base.AdvertiseAddresses = invalid
		require.Error(t, validateReachabilityConfig(base), invalid)
	}
}

func TestPublicDirectWithholdsAndWithdrawsUnverifiedEndpoints(t *testing.T) {
	svc := New(network.Config{
		StorePath:          t.TempDir() + "/waku-store.db",
		PrivateKeyPath:     t.TempDir() + "/waku-key.json",
		BindAddress:        "127.0.0.1",
		Profile:            network.ProfileTCPOnly,
		ReachabilityMode:   network.ReachabilityPublicDirect,
		AdvertiseAddresses: []string{"/dns4/node.example/tcp/61000"},
	})
	require.NoError(t, svc.Start(context.Background()))
	t.Cleanup(func() { require.NoError(t, svc.Stop(context.Background())) })
	require.Empty(t, svc.Endpoints())

	svc.mu.Lock()
	svc.applyReachabilityEventLocked(libp2pnetwork.ReachabilityPublic, time.Now())
	svc.mu.Unlock()
	require.Equal(t, "public", svc.ReachabilitySnapshot().State)
	require.NotEmpty(t, svc.Endpoints())

	svc.mu.Lock()
	svc.applyReachabilityEventLocked(libp2pnetwork.ReachabilityPrivate, time.Now())
	svc.mu.Unlock()
	require.Equal(t, "nat_blocked", svc.ReachabilitySnapshot().State)
	require.Empty(t, svc.Endpoints())
}

func TestOutboundOnlyNeverPublishesBoundEndpoints(t *testing.T) {
	svc := New(network.Config{
		StorePath:        t.TempDir() + "/waku-store.db",
		PrivateKeyPath:   t.TempDir() + "/waku-key.json",
		BindAddress:      "127.0.0.1",
		Profile:          network.ProfileTCPOnly,
		ReachabilityMode: network.ReachabilityOutboundOnly,
	})
	require.NoError(t, svc.Start(context.Background()))
	t.Cleanup(func() { require.NoError(t, svc.Stop(context.Background())) })
	require.Empty(t, svc.Endpoints())
	require.Equal(t, "outbound_only", svc.ReachabilitySnapshot().State)
}

func TestPrivateLANReportsScopedListenerWithoutPublicClaim(t *testing.T) {
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	previous := timeNowUTC
	timeNowUTC = func() time.Time { return now }
	t.Cleanup(func() { timeNowUTC = previous })

	svc := New(network.Config{
		StorePath:          t.TempDir() + "/waku-store.db",
		PrivateKeyPath:     t.TempDir() + "/waku-key.json",
		BindAddress:        "127.0.0.1",
		Profile:            network.ProfileTCPOnly,
		ReachabilityMode:   network.ReachabilityPrivateLAN,
		AdvertiseAddresses: []string{"/ip4/10.23.0.11/tcp/61000"},
	})
	require.NoError(t, svc.Start(context.Background()))
	t.Cleanup(func() { require.NoError(t, svc.Stop(context.Background())) })

	snapshot := svc.ReachabilitySnapshot()
	require.False(t, snapshot.Reachable)
	require.Equal(t, "unknown", snapshot.State)
	require.Empty(t, svc.Endpoints())

	address := "/ip4/10.23.0.11/tcp/61000"
	require.Error(t, svc.ApplyPrivateLANProbe(network.PrivateLANProbe{
		SourceSlot: "node-a", TargetSlot: "node-a", Address: address,
		ObservedAt: now, Success: true,
	}))
	require.Error(t, svc.ApplyPrivateLANProbe(network.PrivateLANProbe{
		SourceSlot: "NODE-B", TargetSlot: "node-a", Address: address,
		ObservedAt: now, Success: true,
	}))
	require.Error(t, svc.ApplyPrivateLANProbe(network.PrivateLANProbe{
		SourceSlot: "node-b", TargetSlot: "node-a",
		Address: "/ip4/10.23.0.99/tcp/61000", ObservedAt: now, Success: true,
	}))
	require.Error(t, svc.ApplyPrivateLANProbe(network.PrivateLANProbe{
		SourceSlot: "node-b", TargetSlot: "node-a", Address: address,
		ObservedAt: now.Add(-network.PrivateLANProbeMaxAge - time.Second), Success: true,
	}))
	require.Error(t, svc.ApplyPrivateLANProbe(network.PrivateLANProbe{
		SourceSlot: "node-b", TargetSlot: "node-a", Address: address,
		ObservedAt: now.Add(network.PrivateLANProbeFutureSkew + time.Second), Success: true,
	}))

	require.NoError(t, svc.ApplyPrivateLANProbe(network.PrivateLANProbe{
		SourceSlot: "node-b", TargetSlot: "node-a", Address: address,
		ObservedAt: now, Success: true,
	}))
	snapshot = svc.ReachabilitySnapshot()
	require.True(t, snapshot.Reachable)
	require.Equal(t, "lan", snapshot.State)
	require.Contains(t, snapshot.Reason, "public ingress is not claimed")
	require.Len(t, svc.Endpoints(), 1)
	require.Contains(t, svc.Endpoints()[0], address)

	require.NoError(t, svc.ApplyPrivateLANProbe(network.PrivateLANProbe{
		SourceSlot: "node-b", TargetSlot: "node-a", Address: address,
		ObservedAt: now.Add(time.Second), Success: false,
	}))
	require.False(t, svc.ReachabilitySnapshot().Reachable)
	require.Empty(t, svc.Endpoints())

	require.NoError(t, svc.ApplyPrivateLANProbe(network.PrivateLANProbe{
		SourceSlot: "node-c", TargetSlot: "node-a", Address: address,
		ObservedAt: now.Add(2 * time.Second), Success: true,
	}))
	now = now.Add(2*time.Second + network.PrivateLANProbeMaxAge + time.Second)
	require.Equal(t, "unknown", svc.ReachabilitySnapshot().State)
	require.Empty(t, svc.Endpoints())
}

func TestPublicWSSAdvertisementMustMatchCertificateHostAndPort(t *testing.T) {
	cfg := network.Config{
		Profile:             network.ProfileTCPWSS,
		ReachabilityMode:    network.ReachabilityPublicDirect,
		WSSPort:             443,
		WSSAdvertiseAddress: "node.example",
		AdvertiseAddresses:  []string{"/dns4/other.example/tcp/443/tls/ws"},
	}
	require.ErrorContains(t, validateReachabilityConfig(cfg), "match the configured certificate host and port")
}
