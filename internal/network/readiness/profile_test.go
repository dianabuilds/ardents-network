package readiness

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLookupProfileReturnsTCPWSSDefinition(t *testing.T) {
	definition := LookupProfile(ProfileTCPWSS)
	require.Equal(t, ProfileTCPWSS, definition.Profile)
	require.True(t, definition.Implemented)
	require.Equal(t, StartupVariantTCPWSS, definition.StartupVariant)
	require.Equal(t, []Family{FamilyTCP, FamilyWSS}, definition.ActiveFamilies)
}

func TestLookupProfileReturnsUnimplementedTCPQUICDefinition(t *testing.T) {
	definition := LookupProfile(ProfileTCPQUIC)
	require.Equal(t, ProfileTCPQUIC, definition.Profile)
	require.False(t, definition.Implemented)
	require.Equal(t, StartupVariantTCPQUIC, definition.StartupVariant)
}

func TestDefinitionRuntimeShapeClonesFamilies(t *testing.T) {
	definition := LookupProfile(ProfileTCPWSS)
	shape := definition.RuntimeShape()
	shape.ActiveFamilies[0] = FamilyQUIC
	shape.SuppressedFamilies[0] = FamilyTCP

	require.Equal(t, FamilyTCP, definition.ActiveFamilies[0])
	require.Equal(t, FamilyQUIC, definition.SuppressedFamilies[0])
}

func TestNormalizeProfileDefaultsToTCPOnly(t *testing.T) {
	require.Equal(t, ProfileTCPOnly, NormalizeProfile(""))
}

func TestRuntimeShapeForProfileTCPWSS(t *testing.T) {
	shape, err := RuntimeShapeForProfile(ProfileTCPWSS)
	require.NoError(t, err)
	require.Equal(t, []Family{FamilyTCP, FamilyWSS}, shape.ActiveFamilies)
}

func TestRuntimeShapeForProfileRejectsUnimplementedProfile(t *testing.T) {
	_, err := RuntimeShapeForProfile(ProfileTCPQUIC)
	require.Error(t, err)
}

func TestImplementedProfilesSuppressDTLSBearingTransportFamilies(t *testing.T) {
	for _, profile := range []Profile{ProfileTCPOnly, ProfileTCPWSS} {
		definition, err := ResolveProfile(profile)
		require.NoError(t, err)
		require.NotContains(t, definition.ActiveFamilies, FamilyQUIC)
		require.NotContains(t, definition.ActiveFamilies, FamilyWebTransport)
		require.NotContains(t, definition.ActiveFamilies, FamilyWebRTC)
		require.Contains(t, definition.SuppressedFamilies, FamilyQUIC)
		require.Contains(t, definition.SuppressedFamilies, FamilyWebTransport)
		require.Contains(t, definition.SuppressedFamilies, FamilyWebRTC)
	}
}

func TestClassifyBootstrapStatusRequiresOperationalRelayPath(t *testing.T) {
	ready := ClassifyBootstrapStatus(1, 1, 0, "")
	require.True(t, ready.Joined)
	require.Equal(t, "ready", ready.State)

	idle := ClassifyBootstrapStatus(0, 0, 0, "")
	require.Equal(t, "idle", idle.State)

	degraded := ClassifyBootstrapStatus(0, 1, 1, "dial timeout")
	require.False(t, degraded.Joined)
	require.Equal(t, "degraded", degraded.State)
	require.Equal(t, "bootstrap peer dial failed", degraded.Reason)

	relayFailed := ClassifyBootstrapStatus(0, 2, 1, "")
	require.Equal(t, "bootstrap relay readiness failed", relayFailed.Reason)
}
