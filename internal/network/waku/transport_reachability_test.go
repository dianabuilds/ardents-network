package waku

import (
	"ardents/internal/network"
	"context"
	"strings"
	"testing"
	"time"

	libp2pnetwork "github.com/libp2p/go-libp2p/core/network"
	"github.com/stretchr/testify/require"
)

func TestReachabilityConfigRequiresPublicCompatibleAddresses(t *testing.T) {
	base := network.Config{
		NodeProfile: network.NodeProfileServiceNode,
		Profile:     network.ProfileTCPOnly, ReachabilityMode: network.ReachabilityPublicDirect,
	}
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
		NodeProfile:              network.NodeProfileServiceNode,
		Profile:                  network.ProfileTCPOnly,
		ReachabilityMode:         network.ReachabilityPrivateLAN,
		PrivateLANManifestDigest: strings.Repeat("a", 64),
		PrivateLANTargetSlot:     "node-a",
		PrivateLANSourceSlots:    []string{"node-b", "node-c"},
	}
	require.ErrorContains(t, validateReachabilityConfig(base), "requires exactly one")

	base.AdvertiseAddresses = []string{"/ip4/10.23.0.11/tcp/61000"}
	require.NoError(t, validateReachabilityConfig(base))

	for _, invalid := range [][]string{
		{"/ip4/10.23.0.11/tcp/61000", "/ip4/10.23.0.12/tcp/61000"},
		{"/ip4/127.0.0.1/tcp/61000"},
		{"/ip4/0.0.0.0/tcp/61000"},
		{"/ip4/203.0.113.10/tcp/61000"},
		{"/ip6/::ffff:a17:b0b/tcp/61000"},
		{"/ip6/fe80::1/tcp/61000"},
		{"/dns4/node-a.internal/tcp/61000"},
		{"/ip4/10.23.0.11/tcp/61000/p2p-circuit"},
		{"/ip4/10.23.0.11/udp/61000"},
	} {
		base.AdvertiseAddresses = invalid
		require.Error(t, validateReachabilityConfig(base), invalid)
	}
}

func TestReachabilityConfigRequiresExactPrivateLANProofScope(t *testing.T) {
	valid := network.Config{
		NodeProfile:              network.NodeProfileServiceNode,
		Profile:                  network.ProfileTCPOnly,
		ReachabilityMode:         network.ReachabilityPrivateLAN,
		AdvertiseAddresses:       []string{"/ip4/10.23.0.11/tcp/61000"},
		PrivateLANManifestDigest: strings.Repeat("a", 64),
		PrivateLANTargetSlot:     "node-a",
		PrivateLANSourceSlots:    []string{"node-b", "node-c"},
	}
	require.NoError(t, validateReachabilityConfig(valid))
	for _, mutate := range []func(*network.Config){
		func(cfg *network.Config) { cfg.PrivateLANManifestDigest = "" },
		func(cfg *network.Config) { cfg.PrivateLANManifestDigest = strings.Repeat("A", 64) },
		func(cfg *network.Config) { cfg.PrivateLANTargetSlot = "" },
		func(cfg *network.Config) { cfg.PrivateLANSourceSlots = []string{"node-b"} },
		func(cfg *network.Config) { cfg.PrivateLANSourceSlots = []string{"node-b", "node-b"} },
		func(cfg *network.Config) { cfg.PrivateLANSourceSlots = []string{"node-a", "node-b"} },
		func(cfg *network.Config) { cfg.PrivateLANSourceSlots = []string{"NODE-B", "node-c"} },
	} {
		cfg := valid
		cfg.PrivateLANSourceSlots = append([]string(nil), valid.PrivateLANSourceSlots...)
		mutate(&cfg)
		require.ErrorContains(t, validateReachabilityConfig(cfg), "admitted topology scope")
	}
}

func TestReachabilityConfigRejectsProfileModeMismatch(t *testing.T) {
	require.Error(t, validateReachabilityConfig(network.Config{
		NodeProfile: network.NodeProfileLocalDevelopment,
		Profile:     network.ProfileTCPOnly, ReachabilityMode: network.ReachabilityPrivateLAN,
		AdvertiseAddresses: []string{"/ip4/10.23.0.11/tcp/61000"},
	}))
	require.Error(t, validateReachabilityConfig(network.Config{
		NodeProfile: network.NodeProfileConstrainedClient,
		Profile:     network.ProfileTCPOnly, ReachabilityMode: network.ReachabilityPrivateLAN,
		AdvertiseAddresses: []string{"/ip4/10.23.0.11/tcp/61000"},
	}))
}

func TestPublicDirectWithholdsAndWithdrawsUnverifiedEndpoints(t *testing.T) {
	svc := New(network.Config{
		NodeProfile:        network.NodeProfileServiceNode,
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
		NodeProfile:      network.NodeProfileConstrainedClient,
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

	digest := strings.Repeat("a", 64)
	svc := New(network.Config{
		NodeProfile:              network.NodeProfileServiceNode,
		StorePath:                t.TempDir() + "/waku-store.db",
		PrivateKeyPath:           t.TempDir() + "/waku-key.json",
		BindAddress:              "127.0.0.1",
		Profile:                  network.ProfileTCPOnly,
		ReachabilityMode:         network.ReachabilityPrivateLAN,
		AdvertiseAddresses:       []string{"/ip4/10.23.0.11/tcp/61000"},
		PrivateLANManifestDigest: digest,
		PrivateLANTargetSlot:     "node-a",
		PrivateLANSourceSlots:    []string{"node-b", "node-c"},
	})
	require.NoError(t, svc.Start(context.Background()))
	t.Cleanup(func() { require.NoError(t, svc.Stop(context.Background())) })

	snapshot := svc.ReachabilitySnapshot()
	require.False(t, snapshot.Reachable)
	require.Equal(t, "unknown", snapshot.State)
	require.Empty(t, svc.Endpoints())

	address := "/ip4/10.23.0.11/tcp/61000"
	scopedProbe := func(source, target, advertised string, at time.Time, success bool) network.PrivateLANProbe {
		return network.PrivateLANProbe{
			ManifestDigest: digest,
			SourceSlot:     source, TargetSlot: target, Address: advertised,
			ObservedAt: at, Success: success,
		}
	}
	require.Error(t, svc.ApplyPrivateLANProbe(
		scopedProbe("node-a", "node-a", address, now, true),
	))
	require.Error(t, svc.ApplyPrivateLANProbe(
		scopedProbe("NODE-B", "node-a", address, now, true),
	))
	require.Error(t, svc.ApplyPrivateLANProbe(
		scopedProbe("node-x", "node-a", address, now, true),
	))
	require.Error(t, svc.ApplyPrivateLANProbe(
		scopedProbe("node-b", "node-c", address, now, true),
	))
	wrongDigest := scopedProbe("node-b", "node-a", address, now, true)
	wrongDigest.ManifestDigest = strings.Repeat("b", 64)
	require.Error(t, svc.ApplyPrivateLANProbe(wrongDigest))
	require.Error(t, svc.ApplyPrivateLANProbe(
		scopedProbe("node-b", "node-a", "/ip4/10.23.0.99/tcp/61000", now, true),
	))
	require.Error(t, svc.ApplyPrivateLANProbe(scopedProbe(
		"node-b", "node-a", address,
		now.Add(-network.PrivateLANProbeMaxAge-time.Second), true,
	)))
	require.Error(t, svc.ApplyPrivateLANProbe(scopedProbe(
		"node-b", "node-a", address,
		now.Add(network.PrivateLANProbeFutureSkew+time.Second), true,
	)))

	require.NoError(t, svc.ApplyPrivateLANProbe(
		scopedProbe("node-b", "node-a", address, now, true),
	))
	snapshot = svc.ReachabilitySnapshot()
	require.True(t, snapshot.Reachable)
	require.Equal(t, "lan", snapshot.State)
	require.Contains(t, snapshot.Reason, "public ingress is not claimed")
	require.Len(t, svc.Endpoints(), 1)
	require.Contains(t, svc.Endpoints()[0], address)

	require.NoError(t, svc.ApplyPrivateLANProbe(
		scopedProbe("node-b", "node-a", address, now.Add(time.Second), false),
	))
	require.False(t, svc.ReachabilitySnapshot().Reachable)
	require.Empty(t, svc.Endpoints())
	require.Error(t, svc.ApplyPrivateLANProbe(
		scopedProbe("node-b", "node-a", address, now, true),
	))
	require.Error(t, svc.ApplyPrivateLANProbe(
		scopedProbe("node-b", "node-a", address, now.Add(time.Second), true),
	))
	require.Empty(t, svc.Endpoints(), "failure dominates equal-time success")

	require.NoError(t, svc.ApplyPrivateLANProbe(
		scopedProbe("node-c", "node-a", address, now.Add(2*time.Second), true),
	))
	expired := make(chan struct{}, 1)
	svc.SetReachabilityObserver(func() { expired <- struct{}{} })
	now = now.Add(2*time.Second + network.PrivateLANProbeMaxAge + time.Second)
	require.Empty(t, svc.Endpoints())
	select {
	case <-expired:
	case <-time.After(time.Second):
		t.Fatal("private LAN expiry did not notify the reachability observer")
	}
	require.Equal(t, "unknown", svc.ReachabilitySnapshot().State)
}

func TestPublicWSSAdvertisementMustMatchCertificateHostAndPort(t *testing.T) {
	cfg := network.Config{
		NodeProfile:         network.NodeProfileServiceNode,
		Profile:             network.ProfileTCPWSS,
		ReachabilityMode:    network.ReachabilityPublicDirect,
		WSSPort:             443,
		WSSAdvertiseAddress: "node.example",
		AdvertiseAddresses:  []string{"/dns4/other.example/tcp/443/tls/ws"},
	}
	require.ErrorContains(t, validateReachabilityConfig(cfg), "match the configured certificate host and port")
}
