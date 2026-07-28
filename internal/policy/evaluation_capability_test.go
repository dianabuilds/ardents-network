package policy

import (
	"testing"

	identityapi "ardents/internal/identity"

	"github.com/stretchr/testify/require"
)

func TestCheckChannelGrantUseEnforcesDisableAndScopeRules(t *testing.T) {
	use := identityapi.CapabilityUse{Scope: identityapi.CapabilityRealmDiscovery}
	require.True(t, CheckChannelGrantUse(ChannelGrantPolicyConfig{}, use).Allowed)
	require.False(t, CheckChannelGrantUse(ChannelGrantPolicyConfig{DisablePrivateChannelGrantUse: true}, use).Allowed)
	require.False(t, CheckChannelGrantUse(ChannelGrantPolicyConfig{
		DeniedChannelGrantScopes: []string{"REALM.DISCOVERY"},
	}, use).Allowed)
}

func TestRealmAuthorityChannelClassAdmissionUsesExactScopePolicy(t *testing.T) {
	service := New(Config{
		DeniedChannelGrantScopes: []string{string(identityapi.CapabilityApplication)},
	})
	require.NoError(t, service.AllowRealmChannelClass(identityapi.CapabilityRealmDiscovery))
	require.Error(t, service.AllowRealmChannelClass(identityapi.CapabilityApplication))
}
