package transport

import (
	"context"
	"testing"
	"time"

	networkreadiness "ardents/internal/network/readiness"

	libp2pnetwork "github.com/libp2p/go-libp2p/core/network"
	"github.com/stretchr/testify/require"
)

func TestReachabilityConfigRequiresPublicCompatibleAddresses(t *testing.T) {
	base := Config{Profile: networkreadiness.ProfileTCPOnly, ReachabilityMode: networkreadiness.ReachabilityPublicDirect}
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

func TestPublicDirectWithholdsAndWithdrawsUnverifiedEndpoints(t *testing.T) {
	svc := New(Config{
		StorePath:          t.TempDir() + "/waku-store.db",
		PrivateKeyPath:     t.TempDir() + "/waku-key.json",
		BindAddress:        "127.0.0.1",
		Profile:            networkreadiness.ProfileTCPOnly,
		ReachabilityMode:   networkreadiness.ReachabilityPublicDirect,
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
	svc := New(Config{
		StorePath:        t.TempDir() + "/waku-store.db",
		PrivateKeyPath:   t.TempDir() + "/waku-key.json",
		BindAddress:      "127.0.0.1",
		Profile:          networkreadiness.ProfileTCPOnly,
		ReachabilityMode: networkreadiness.ReachabilityOutboundOnly,
	})
	require.NoError(t, svc.Start(context.Background()))
	t.Cleanup(func() { require.NoError(t, svc.Stop(context.Background())) })
	require.Empty(t, svc.Endpoints())
	require.Equal(t, "outbound_only", svc.ReachabilitySnapshot().State)
}

func TestPrivateLANReportsScopedListenerWithoutPublicClaim(t *testing.T) {
	svc := New(Config{
		StorePath:        t.TempDir() + "/waku-store.db",
		PrivateKeyPath:   t.TempDir() + "/waku-key.json",
		BindAddress:      "127.0.0.1",
		Profile:          networkreadiness.ProfileTCPOnly,
		ReachabilityMode: networkreadiness.ReachabilityPrivateLAN,
	})
	require.NoError(t, svc.Start(context.Background()))
	t.Cleanup(func() { require.NoError(t, svc.Stop(context.Background())) })
	snapshot := svc.ReachabilitySnapshot()
	require.True(t, snapshot.Reachable)
	require.Equal(t, "lan", snapshot.State)
	require.Contains(t, snapshot.Reason, "public ingress is not claimed")
}

func TestPublicWSSAdvertisementMustMatchCertificateHostAndPort(t *testing.T) {
	cfg := Config{
		Profile:             networkreadiness.ProfileTCPWSS,
		ReachabilityMode:    networkreadiness.ReachabilityPublicDirect,
		WSSPort:             443,
		WSSAdvertiseAddress: "node.example",
		AdvertiseAddresses:  []string{"/dns4/other.example/tcp/443/tls/ws"},
	}
	require.ErrorContains(t, validateReachabilityConfig(cfg), "match the configured certificate host and port")
}
