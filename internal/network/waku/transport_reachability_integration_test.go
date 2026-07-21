//go:build integration

package waku

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"ardents/internal/network"

	libp2pevent "github.com/libp2p/go-libp2p/core/event"
	libp2pnetwork "github.com/libp2p/go-libp2p/core/network"
	"github.com/stretchr/testify/require"
)

func TestPublicIngressObservationWithdrawsAndRecoversRealWakuEndpoints(t *testing.T) {
	svc := startPublicReachabilityNode(t, t.TempDir(), "/dns4/node-a.example/tcp/61000")
	require.Empty(t, svc.Endpoints())

	emitter, err := svc.node.Host().EventBus().Emitter(new(libp2pevent.EvtLocalReachabilityChanged))
	require.NoError(t, err)
	t.Cleanup(func() { _ = emitter.Close() })
	require.NoError(t, emitter.Emit(libp2pevent.EvtLocalReachabilityChanged{Reachability: libp2pnetwork.ReachabilityPublic}))
	require.Eventually(t, func() bool {
		return svc.ReachabilitySnapshot().State == "public" && len(svc.Endpoints()) == 1
	}, 3*time.Second, 25*time.Millisecond)

	require.NoError(t, emitter.Emit(libp2pevent.EvtLocalReachabilityChanged{Reachability: libp2pnetwork.ReachabilityPrivate}))
	require.Eventually(t, func() bool {
		return svc.ReachabilitySnapshot().State == "nat_blocked" && len(svc.Endpoints()) == 0
	}, 3*time.Second, 25*time.Millisecond)

	require.NoError(t, emitter.Emit(libp2pevent.EvtLocalReachabilityChanged{Reachability: libp2pnetwork.ReachabilityPublic}))
	require.Eventually(t, func() bool { return len(svc.Endpoints()) == 1 }, 3*time.Second, 25*time.Millisecond)
}

func TestPublicAddressChangeRequiresRestartAndFreshObservation(t *testing.T) {
	dir := t.TempDir()
	first := startPublicReachabilityNode(t, dir, "/dns4/node-a.example/tcp/61000")
	first.mu.Lock()
	first.applyReachabilityEventLocked(libp2pnetwork.ReachabilityPublic, time.Now())
	first.mu.Unlock()
	require.Contains(t, first.Endpoints()[0], "node-a.example")
	require.NoError(t, first.Stop(context.Background()))

	second := startPublicReachabilityNode(t, dir, "/dns4/node-b.example/tcp/62000")
	require.Empty(t, second.Endpoints(), "a changed address must not inherit prior reachability")
	second.mu.Lock()
	second.applyReachabilityEventLocked(libp2pnetwork.ReachabilityPublic, time.Now())
	second.mu.Unlock()
	require.Contains(t, second.Endpoints()[0], "node-b.example")
	require.NotContains(t, second.Endpoints()[0], "node-a.example")
}

func startPublicReachabilityNode(t *testing.T, dir, address string) *Service {
	t.Helper()
	svc := New(network.Config{
		StorePath:          filepath.Join(dir, "waku-store.db"),
		PrivateKeyPath:     filepath.Join(dir, "waku-key.json"),
		BindAddress:        "127.0.0.1",
		Profile:            network.ProfileTCPOnly,
		ReachabilityMode:   network.ReachabilityPublicDirect,
		AdvertiseAddresses: []string{address},
	})
	require.NoError(t, svc.Start(context.Background()))
	t.Cleanup(func() {
		if svc.State() != "stopped" {
			require.NoError(t, svc.Stop(context.Background()))
		}
	})
	return svc
}
