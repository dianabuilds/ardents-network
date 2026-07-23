package network

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestStatusUsesObservedReachability(t *testing.T) {
	unknown := ProjectStatus(NodeProfileServiceNode, "ready", "", false,
		Snapshot{Profile: ProfileTCPOnly}, ReachabilitySnapshot{Mode: ReachabilityPublicDirect, State: "unknown"},
		AbuseSnapshot{}, time.Time{}, PrivateMessagingStatus{})
	require.False(t, unknown.Reachable)

	public := ProjectStatus(NodeProfileServiceNode, "ready", "", true,
		Snapshot{Profile: ProfileTCPOnly}, ReachabilitySnapshot{Mode: ReachabilityPublicDirect, State: "public", Reachable: true},
		AbuseSnapshot{}, time.Time{}, PrivateMessagingStatus{})
	require.True(t, public.Reachable)
	require.Equal(t, "public", public.ReachabilityState)
}

func TestStatusReportsObservedCapabilitiesAndAbuse(t *testing.T) {
	status := ProjectStatus(NodeProfileConstrainedClient, "degraded", "", false,
		Snapshot{Profile: ProfileTCPOnly, ActiveFeatures: []TransportFeature{TransportFeatureFilterClient}, ReducedFeatures: []TransportFeature{TransportFeatureStoreRecovery}},
		ReachabilitySnapshot{Mode: ReachabilityOutboundOnly},
		AbuseSnapshot{State: "degraded", RateLimitedOperations: 3, BackpressuredOperations: 2, OversizedMessages: 1, BannedProviders: 1},
		time.Time{}, PrivateMessagingStatus{})
	require.Equal(t, []TransportFeature{TransportFeatureFilterClient}, status.ActiveFeatures)
	require.Contains(t, status.ReducedFeatures, TransportFeatureStoreRecovery)
	require.Equal(t, uint64(3), status.RateLimitedOperations)
	require.Equal(t, 1, status.BannedProviders)
}
