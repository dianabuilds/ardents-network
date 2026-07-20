package status

import (
	"testing"
	"time"

	networkapi "ardents/internal/network/api"
	networkprivacy "ardents/internal/network/privacy"

	"github.com/stretchr/testify/require"
)

func TestNetworkStatusDoesNotTreatLocalBindingAsReachability(t *testing.T) {
	status := NetworkStatusSnapshot(
		networkapi.NodeProfileServiceNode, "ready", "", false,
		networkapi.Snapshot{Profile: networkapi.ProfileTCPOnly, ActiveFamilies: []networkapi.Family{"tcp"}},
		networkapi.ReachabilitySnapshot{Mode: networkapi.ReachabilityPublicDirect, State: "unknown"},
		networkapi.AbuseSnapshot{},
		time.Time{}, networkprivacy.StatusSnapshot{},
	)

	require.False(t, status.Reachable)
}

func TestNetworkStatusReportsVerifiedIngressAsReachable(t *testing.T) {
	status := NetworkStatusSnapshot(
		networkapi.NodeProfileServiceNode, "ready", "", true,
		networkapi.Snapshot{Profile: networkapi.ProfileTCPOnly, ActiveFamilies: []networkapi.Family{"tcp"}},
		networkapi.ReachabilitySnapshot{Mode: networkapi.ReachabilityPublicDirect, State: "public", Reachable: true},
		networkapi.AbuseSnapshot{},
		time.Time{}, networkprivacy.StatusSnapshot{},
	)

	require.True(t, status.Reachable)
	require.Equal(t, "public", status.ReachabilityState)
}

func TestNetworkStatusReportsOnlyObservedLightClientCapabilities(t *testing.T) {
	status := NetworkStatusSnapshot(
		networkapi.NodeProfileConstrainedClient, "degraded", "Store provider missing", false,
		networkapi.Snapshot{
			NodeProfile: networkapi.NodeProfileConstrainedClient, Profile: networkapi.ProfileTCPOnly,
			ActiveCapabilities: []string{"filter_client", "lightpush_client"}, ReducedCapabilities: []string{"store_recovery"},
		},
		networkapi.ReachabilitySnapshot{Mode: networkapi.ReachabilityOutboundOnly, State: "outbound_only"},
		networkapi.AbuseSnapshot{},
		time.Time{}, networkprivacy.StatusSnapshot{},
	)

	require.ElementsMatch(t, []string{"filter_client", "lightpush_client"}, status.ActiveCapabilities)
	require.Contains(t, status.ReducedCapabilities, "store_recovery")
}

func TestNetworkStatusReportsAbuseStateWithoutProviderIdentity(t *testing.T) {
	status := NetworkStatusSnapshot(
		networkapi.NodeProfileServiceNode, "degraded", "", true,
		networkapi.Snapshot{Profile: networkapi.ProfileTCPOnly},
		networkapi.ReachabilitySnapshot{Mode: networkapi.ReachabilityPrivateLAN, State: "lan"},
		networkapi.AbuseSnapshot{
			State: "degraded", Reason: "one or more network providers are temporarily banned after repeated failures",
			RateLimitedOperations: 3, BackpressuredOperations: 2, OversizedMessages: 1, BannedProviders: 1,
		},
		time.Time{}, networkprivacy.StatusSnapshot{},
	)

	require.Equal(t, "degraded", status.AbuseState)
	require.Equal(t, uint64(3), status.RateLimitedOperations)
	require.Equal(t, uint64(2), status.BackpressuredOperations)
	require.Equal(t, uint64(1), status.OversizedMessages)
	require.Equal(t, 1, status.BannedProviders)
}
