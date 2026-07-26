package discovery_test

import (
	"testing"

	applicationadmission "ardents/internal/applicationapi/admission"
	applicationcontent "ardents/internal/applicationapi/content"
	applicationdiscovery "ardents/internal/applicationapi/discovery"
	applicationv1connect "ardents/sdk/go/protocol/applicationv1/applicationv1connect"

	"github.com/stretchr/testify/require"
)

func TestDiscoveryProcedureComposesWithContentThroughClosedRegistry(t *testing.T) {
	contentContracts, contentRules, err := applicationcontent.ProtectedProcedureSet()
	require.NoError(t, err)
	discoveryContracts, discoveryRules, err := applicationdiscovery.ProtectedProcedureSet()
	require.NoError(t, err)
	require.Equal(t, []string{applicationv1connect.DiscoveryServiceResolveProcedure}, discoveryContracts)
	require.Len(t, discoveryRules, 1)
	require.Equal(t, applicationdiscovery.ActionResolve, discoveryRules[0].Action)
	require.Equal(t, "service-type", string(discoveryRules[0].ResourceKind))
	require.False(t, discoveryRules[0].OwnerRequired)
	require.False(t, discoveryRules[0].Mutating)

	contracts := append(append([]string(nil), contentContracts...), discoveryContracts...)
	rules := append(append([]applicationadmission.ProcedureRule(nil), contentRules...), discoveryRules...)
	registry, err := applicationadmission.NewRegistry(contracts, rules)
	require.NoError(t, err)
	_, ok := registry.Lookup(applicationv1connect.ContentServiceGetProcedure)
	require.True(t, ok)
	_, ok = registry.Lookup(applicationv1connect.DiscoveryServiceResolveProcedure)
	require.True(t, ok)

	_, err = applicationadmission.NewRegistry(contracts, contentRules)
	require.Error(t, err)
	_, err = applicationadmission.NewRegistry(contracts, append(rules, discoveryRules[0]))
	require.Error(t, err)
}
