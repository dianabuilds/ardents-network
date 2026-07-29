package network

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReachabilityModeForProfileUsesSafeConsistentDefaults(t *testing.T) {
	require.Equal(t, ReachabilityLocalOnly,
		ReachabilityModeForProfile("", NodeProfileLocalDevelopment))
	require.Equal(t, ReachabilityOutboundOnly,
		ReachabilityModeForProfile("", NodeProfileServiceNode))
	require.Equal(t, ReachabilityOutboundOnly,
		ReachabilityModeForProfile("", NodeProfileConstrainedClient))
	require.Equal(t, ReachabilityPrivateLAN,
		ReachabilityModeForProfile(ReachabilityPrivateLAN, NodeProfileServiceNode))
}
