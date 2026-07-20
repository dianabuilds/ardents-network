package readiness

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestServiceNodeAllowsImplementedTransportProfiles(t *testing.T) {
	for _, profile := range []Profile{ProfileTCPOnly, ProfileTCPWSS} {
		require.NoError(t, ValidateNodeProfileTransport(NodeProfileServiceNode, profile))
	}
}

func TestLocalDevelopmentRejectsWSSCombination(t *testing.T) {
	err := ValidateNodeProfileTransport(NodeProfileLocalDevelopment, ProfileTCPWSS)

	require.ErrorContains(t, err, "does not support")
}

func TestConstrainedClientAllowsOnlyTCPTransport(t *testing.T) {
	require.NoError(t, ValidateNodeProfileTransport(NodeProfileConstrainedClient, ProfileTCPOnly))
	require.ErrorContains(t,
		ValidateNodeProfileTransport(NodeProfileConstrainedClient, ProfileTCPWSS), "does not support")
}

func TestRestrictedDefenseCannotBeSelectedAsStartupProfile(t *testing.T) {
	_, err := ResolveNodeProfile(NodeProfileRestrictedDefense)

	require.ErrorContains(t, err, "automatic runtime mode")
}

func TestUnknownNodeProfileFailsClosed(t *testing.T) {
	_, err := ResolveNodeProfile("unknown")

	require.ErrorContains(t, err, "unknown")
}

func TestNodeProfileDefaultsToLocalDevelopment(t *testing.T) {
	require.Equal(t, NodeProfileLocalDevelopment, NormalizeNodeProfile(""))
}
