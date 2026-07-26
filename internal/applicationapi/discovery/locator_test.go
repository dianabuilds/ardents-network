package discovery_test

import (
	"testing"

	applicationdiscovery "ardents/internal/applicationapi/discovery"
	discoverytruth "ardents/internal/discovery"
	discoveryrecord "ardents/internal/discovery/records"
	"ardents/tests/testkit"

	"github.com/stretchr/testify/require"
)

func TestLocatorResolvesTrustedCurrentNetworkPublishedEndpoint(t *testing.T) {
	record, registry := testkit.TrustedNetworkPublishedService(t, "svc.echo", "echo", "https://10.20.30.40:8443")
	trust := discoverytruth.NewTrustEvaluator(registry)
	store := discoverytruth.NewWithTrust("", trust)
	_, err := store.Import(record, discoveryrecord.Imported)
	require.NoError(t, err)
	truth, err := applicationdiscovery.NewMaintainedTruth(store, trust)
	require.NoError(t, err)
	locator, err := applicationdiscovery.NewLocator(truth)
	require.NoError(t, err)

	targets, err := locator.Resolve(applicationdiscovery.Query{
		ServiceType: "echo", AcceptedSchemes: []string{"https"},
	})

	require.NoError(t, err)
	require.Equal(t, []applicationdiscovery.Target{{
		ServiceID: "svc.echo", Endpoint: "https://10.20.30.40:8443", Scheme: "https",
	}}, targets)
}

func TestLocatorReportsMaintainedTruthBeforeReadinessAsUnavailable(t *testing.T) {
	store := discoverytruth.New("")
	truth, err := applicationdiscovery.NewMaintainedTruth(store, discoverytruth.NewTrustEvaluator(nil))
	require.NoError(t, err)
	locator, err := applicationdiscovery.NewLocator(truth)
	require.NoError(t, err)

	_, err = locator.Resolve(applicationdiscovery.Query{
		ServiceType: "echo", AcceptedSchemes: []string{"https"},
	})

	require.ErrorIs(t, err, applicationdiscovery.ErrUnavailable)
}

func TestLocatorPropagatesFallibleTruthSeamAsUnavailable(t *testing.T) {
	locator, err := applicationdiscovery.NewLocator(unavailableTruth{})
	require.NoError(t, err)

	_, err = locator.Resolve(applicationdiscovery.Query{
		ServiceType: "echo", AcceptedSchemes: []string{"https"},
	})

	require.ErrorIs(t, err, applicationdiscovery.ErrUnavailable)
}

type unavailableTruth struct{}

func (unavailableTruth) FindService(string) ([]discoverytruth.Entry, error) {
	return nil, applicationdiscovery.ErrUnavailable
}

func (unavailableTruth) Evaluate(discoverytruth.Record) (discoverytruth.TrustResult, error) {
	return discoverytruth.TrustResult{}, applicationdiscovery.ErrUnavailable
}
