package evaluation

import (
	"testing"

	identityapi "ardents/internal/identity/api"

	"github.com/stretchr/testify/require"
)

func TestCheckCapabilityUseEnforcesDisableAndScopeRules(t *testing.T) {
	use := identityapi.CapabilityUse{Scope: identityapi.CapabilityRealmDiscovery}
	require.True(t, CheckCapabilityUse(CapabilityConfig{}, use).Allowed)
	require.False(t, CheckCapabilityUse(CapabilityConfig{DisablePrivateCapabilityUse: true}, use).Allowed)
	require.False(t, CheckCapabilityUse(CapabilityConfig{
		DeniedCapabilityScopes: []string{"REALM.DISCOVERY"},
	}, use).Allowed)
}
