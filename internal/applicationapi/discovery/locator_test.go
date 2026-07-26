package discovery_test

import (
	"errors"
	"fmt"
	"testing"

	applicationdiscovery "ardents/internal/applicationapi/discovery"
	discoverytruth "ardents/internal/discovery"
	discoveryrecord "ardents/internal/discovery/records"
	"ardents/internal/network"
	"ardents/internal/policy"
	"ardents/tests/testkit"

	"github.com/stretchr/testify/require"
)

func TestLocatorProjectsOnlyEligibleDirectEndpoints(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		schemes  []string
		want     bool
	}{
		{name: "https public IPv4", endpoint: "https://203.0.113.10:8443/api?version=1", schemes: []string{"https"}, want: true},
		{name: "http private IPv4", endpoint: "http://10.20.30.40:8080", schemes: []string{"http"}, want: true},
		{name: "tcp link-local IPv4", endpoint: "tcp://169.254.10.20:9000", schemes: []string{"tcp"}, want: true},
		{name: "https public IPv6", endpoint: "https://[2001:db8::10]:8443", schemes: []string{"https"}, want: true},
		{name: "tcp link-local IPv6", endpoint: "tcp://[fe80::10]:9000", schemes: []string{"tcp"}, want: true},
		{name: "tcp zoned link-local IPv6", endpoint: "tcp://[fe80::10%25eth0]:9000", schemes: []string{"tcp"}, want: true},
		{name: "caller scheme mismatch", endpoint: "http://10.20.30.40:8080", schemes: []string{"https"}},
		{name: "DNS name", endpoint: "https://service.internal:8443", schemes: []string{"https"}},
		{name: "Unix", endpoint: "unix:///run/service.sock", schemes: []string{"https"}},
		{name: "Waku", endpoint: "waku://10.20.30.40:9000", schemes: []string{"https"}},
		{name: "relay", endpoint: "relay://10.20.30.40:9000", schemes: []string{"https"}},
		{name: "multiaddr", endpoint: "/ip4/10.20.30.40/tcp/9000", schemes: []string{"https"}},
		{name: "QUIC", endpoint: "quic://10.20.30.40:9000", schemes: []string{"https"}},
		{name: "WebRTC", endpoint: "webrtc://10.20.30.40:9000", schemes: []string{"https"}},
		{name: "userinfo", endpoint: "https://user:secret@10.20.30.40:8443", schemes: []string{"https"}},
		{name: "fragment", endpoint: "https://10.20.30.40:8443/api#private", schemes: []string{"https"}},
		{name: "empty fragment", endpoint: "https://10.20.30.40:8443/api#", schemes: []string{"https"}},
		{name: "non-ASCII path", endpoint: "https://10.20.30.40:8443/café", schemes: []string{"https"}},
		{name: "missing port", endpoint: "https://10.20.30.40/api", schemes: []string{"https"}},
		{name: "zero port", endpoint: "https://10.20.30.40:0/api", schemes: []string{"https"}},
		{name: "overflow port", endpoint: "https://10.20.30.40:65536/api", schemes: []string{"https"}},
		{name: "malformed port", endpoint: "https://10.20.30.40:not-a-port/api", schemes: []string{"https"}},
		{name: "loopback IPv4", endpoint: "https://127.0.0.1:8443", schemes: []string{"https"}},
		{name: "loopback mapped IPv4", endpoint: "https://[::ffff:127.0.0.1]:8443", schemes: []string{"https"}},
		{name: "loopback IPv6", endpoint: "https://[::1]:8443", schemes: []string{"https"}},
		{name: "unspecified IPv4", endpoint: "https://0.0.0.0:8443", schemes: []string{"https"}},
		{name: "unspecified mapped IPv4", endpoint: "https://[::ffff:0.0.0.0]:8443", schemes: []string{"https"}},
		{name: "unspecified IPv6", endpoint: "https://[::]:8443", schemes: []string{"https"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			truth := &projectionTruth{entries: []discoverytruth.Entry{serviceEntry(
				"svc.echo", "echo", "NetworkPublished", test.endpoint,
			)}}
			locator, err := applicationdiscovery.NewLocator(truth)
			require.NoError(t, err)

			targets, err := locator.Resolve(applicationdiscovery.Query{
				ServiceType: "echo", AcceptedSchemes: test.schemes,
			})

			if test.want {
				require.NoError(t, err)
				require.Equal(t, []applicationdiscovery.Target{{
					ServiceID: "svc.echo", Endpoint: test.endpoint, Scheme: test.schemes[0],
				}}, targets)
				return
			}
			require.ErrorIs(t, err, applicationdiscovery.ErrNotFound)
			require.Nil(t, targets)
		})
	}
}

func TestLocatorOrdersDeduplicatesAndCapsTargets(t *testing.T) {
	truth := &projectionTruth{entries: []discoverytruth.Entry{
		serviceEntry("svc.z", "echo", "NetworkPublished", "https://10.0.0.2:8443"),
		serviceEntry("svc.a", "echo", "NetworkPublished", "http://10.0.0.2:8002", "http://10.0.0.1:8001"),
		serviceEntry("svc.a", "echo", "NetworkPublished", "http://10.0.0.1:8001"),
		serviceEntry("svc.aa", "echo", "NetworkPublished", "https://10.0.0.4:8443"),
		serviceEntry("svc.ab", "echo", "NetworkPublished", "http://10.0.0.1:8001"),
		serviceEntry("svc.c", "echo", "NetworkPublished", "http://10.0.0.5:8000"),
		serviceEntry("svc.d", "echo", "NetworkPublished", "http://10.0.0.6:8000"),
		serviceEntry("svc.e", "echo", "NetworkPublished", "http://10.0.0.7:8000"),
		serviceEntry("svc.f", "echo", "NetworkPublished", "http://10.0.0.8:8000"),
		serviceEntry("svc.g", "echo", "NetworkPublished", "http://10.0.0.9:8000"),
		serviceEntry("svc.h", "echo", "NetworkPublished", "http://10.0.0.10:8000"),
		serviceEntry("svc.b", "echo", "NetworkPublished", "tcp://10.0.0.3:9000"),
	}}
	locator, err := applicationdiscovery.NewLocator(truth)
	require.NoError(t, err)

	targets, err := locator.Resolve(applicationdiscovery.Query{
		ServiceType: "echo", AcceptedSchemes: []string{"tcp", "https", "http"},
	})

	require.NoError(t, err)
	require.Equal(t, []applicationdiscovery.Target{
		{ServiceID: "svc.b", Endpoint: "tcp://10.0.0.3:9000", Scheme: "tcp"},
		{ServiceID: "svc.aa", Endpoint: "https://10.0.0.4:8443", Scheme: "https"},
		{ServiceID: "svc.z", Endpoint: "https://10.0.0.2:8443", Scheme: "https"},
		{ServiceID: "svc.a", Endpoint: "http://10.0.0.1:8001", Scheme: "http"},
		{ServiceID: "svc.a", Endpoint: "http://10.0.0.2:8002", Scheme: "http"},
		{ServiceID: "svc.ab", Endpoint: "http://10.0.0.1:8001", Scheme: "http"},
		{ServiceID: "svc.c", Endpoint: "http://10.0.0.5:8000", Scheme: "http"},
		{ServiceID: "svc.d", Endpoint: "http://10.0.0.6:8000", Scheme: "http"},
	}, targets)
}

func TestLocatorRequiresCurrentLifecycleModeTrustAndRoutePolicy(t *testing.T) {
	tests := []struct {
		name        string
		entries     []discoverytruth.Entry
		trust       discoverytruth.TrustResult
		policyErr   error
		want        bool
		policyCalls int
	}{
		{name: "absent"},
		{name: "expired", entries: []discoverytruth.Entry{
			serviceEntry("svc.echo", "echo", "NetworkPublished", "https://10.20.30.40:8443"),
		}, trust: discoverytruth.TrustResult{Outcome: "expired"}},
		{name: "withdrawn", entries: []discoverytruth.Entry{
			serviceEntry("svc.echo", "echo", "NetworkPublished"),
		}, trust: discoverytruth.TrustResult{Valid: true, Trusted: true, Usable: true}},
		{name: "wrong mode", entries: []discoverytruth.Entry{
			serviceEntry("svc.echo", "echo", "LocalOnly", "https://10.20.30.40:8443"),
		}, trust: discoverytruth.TrustResult{Valid: true, Trusted: true, Usable: true}},
		{name: "untrusted", entries: []discoverytruth.Entry{
			serviceEntry("svc.echo", "echo", "NetworkPublished", "https://10.20.30.40:8443"),
		}, trust: discoverytruth.TrustResult{Valid: true}},
		{name: "inconsistent usable but untrusted", entries: []discoverytruth.Entry{
			serviceEntry("svc.echo", "echo", "NetworkPublished", "https://10.20.30.40:8443"),
		}, trust: discoverytruth.TrustResult{Valid: true, Usable: true}},
		{name: "unsafe endpoint before policy", entries: []discoverytruth.Entry{
			serviceEntry("svc.echo", "echo", "NetworkPublished", "https://127.0.0.1:8443"),
		}, trust: discoverytruth.TrustResult{Valid: true, Trusted: true, Usable: true}},
		{name: "policy denied", entries: []discoverytruth.Entry{
			serviceEntry("svc.echo", "echo", "NetworkPublished", "https://10.20.30.40:8443"),
		}, trust: discoverytruth.TrustResult{Valid: true, Trusted: true, Usable: true},
			policyErr: errors.New("private policy reason"), policyCalls: 1},
		{name: "eligible", entries: []discoverytruth.Entry{
			serviceEntry("svc.echo", "echo", "NetworkPublished", "https://10.20.30.40:8443"),
		}, trust: discoverytruth.TrustResult{Valid: true, Trusted: true, Usable: true},
			want: true, policyCalls: 1},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			truth := &projectionTruth{entries: test.entries, trust: test.trust, policyErr: test.policyErr}
			locator, err := applicationdiscovery.NewLocator(truth)
			require.NoError(t, err)

			targets, err := locator.Resolve(applicationdiscovery.Query{
				ServiceType: "echo", AcceptedSchemes: []string{"https"},
			})

			if test.want {
				require.NoError(t, err)
				require.Len(t, targets, 1)
			} else {
				require.ErrorIs(t, err, applicationdiscovery.ErrNotFound)
				require.Nil(t, targets)
			}
			require.Len(t, truth.policyCalls, test.policyCalls)
			if test.policyCalls == 1 {
				require.Equal(t, network.Candidate{
					Subject: "svc.echo", Service: "echo", Endpoint: "https://10.20.30.40:8443",
					Scheme: "https", Mode: "NetworkPublished", Trusted: true, Usable: true,
				}, truth.policyCalls[0])
			}
		})
	}
}

func TestLocatorClassifiesMaintainedTruthFailuresWithoutPrivateDetail(t *testing.T) {
	tests := []struct {
		name  string
		truth *projectionTruth
		want  error
	}{
		{
			name:  "record store unavailable",
			truth: &projectionTruth{findErr: applicationdiscovery.ErrUnavailable},
			want:  applicationdiscovery.ErrUnavailable,
		},
		{
			name: "trust unavailable",
			truth: &projectionTruth{
				entries:     []discoverytruth.Entry{serviceEntry("svc.echo", "echo", "NetworkPublished", "https://10.20.30.40:8443")},
				evaluateErr: applicationdiscovery.ErrUnavailable,
			},
			want: applicationdiscovery.ErrUnavailable,
		},
		{
			name: "route policy unavailable",
			truth: &projectionTruth{
				entries:   []discoverytruth.Entry{serviceEntry("svc.echo", "echo", "NetworkPublished", "https://10.20.30.40:8443")},
				policyErr: applicationdiscovery.ErrUnavailable,
			},
			want: applicationdiscovery.ErrUnavailable,
		},
		{
			name: "unexpected truth invariant",
			truth: &projectionTruth{
				entries:     []discoverytruth.Entry{serviceEntry("svc.echo", "echo", "NetworkPublished", "https://10.20.30.40:8443")},
				evaluateErr: errors.New("private trust invariant"),
			},
			want: applicationdiscovery.ErrInternal,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			locator, err := applicationdiscovery.NewLocator(test.truth)
			require.NoError(t, err)

			_, err = locator.Resolve(applicationdiscovery.Query{
				ServiceType: "echo", AcceptedSchemes: []string{"https"},
			})

			require.ErrorIs(t, err, test.want)
			require.NotContains(t, err.Error(), "private trust invariant")
		})
	}
}

func TestLocatorPerformsNoObservationRefreshProbeFetchOrDial(t *testing.T) {
	truth := &sideEffectTruth{projectionTruth: projectionTruth{
		entries: []discoverytruth.Entry{
			serviceEntry("svc.echo", "echo", "NetworkPublished", "https://10.20.30.40:8443"),
		},
	}}
	locator, err := applicationdiscovery.NewLocator(truth)
	require.NoError(t, err)

	_, err = locator.Resolve(applicationdiscovery.Query{
		ServiceType: "echo", AcceptedSchemes: []string{"https"},
	})

	require.NoError(t, err)
	require.Zero(t, truth.observationCalls)
	require.Zero(t, truth.refreshCalls)
	require.Zero(t, truth.probeCalls)
	require.Zero(t, truth.fetchCalls)
	require.Zero(t, truth.dialCalls)
	require.Zero(t, truth.networkCalls)
}

func TestLocatorResolvesTrustedCurrentNetworkPublishedEndpoint(t *testing.T) {
	record, registry := testkit.TrustedNetworkPublishedService(t, "svc.echo", "echo", "https://10.20.30.40:8443")
	trust := discoverytruth.NewTrustEvaluator(registry)
	store := discoverytruth.NewWithTrust("", trust)
	_, err := store.Import(record, discoveryrecord.Imported)
	require.NoError(t, err)
	truth, err := applicationdiscovery.NewMaintainedTruth(store, trust, allowRoutePolicy{})
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

func TestMaintainedTruthUsesCurrentTrustAndRoutePolicyOnEveryResolve(t *testing.T) {
	record, registry := testkit.TrustedNetworkPublishedService(t, "svc.echo", "echo", "https://10.20.30.40:8443")
	trust := discoverytruth.NewTrustEvaluator(registry)
	store := discoverytruth.NewWithTrust("", trust)
	_, err := store.Import(record, discoveryrecord.Imported)
	require.NoError(t, err)
	routePolicy := policy.New(policy.Config{})
	truth, err := applicationdiscovery.NewMaintainedTruth(store, trust, routePolicy)
	require.NoError(t, err)
	locator, err := applicationdiscovery.NewLocator(truth)
	require.NoError(t, err)
	query := applicationdiscovery.Query{ServiceType: "echo", AcceptedSchemes: []string{"https"}}

	_, err = locator.Resolve(query)
	require.NoError(t, err)

	trust.ReplaceRegistry(nil)
	_, err = locator.Resolve(query)
	require.ErrorIs(t, err, applicationdiscovery.ErrNotFound)

	trust.ReplaceRegistry(registry)
	routePolicy.Reconfigure(policy.Config{DeniedRouteSchemes: []string{"https"}})
	_, err = locator.Resolve(query)
	require.ErrorIs(t, err, applicationdiscovery.ErrNotFound)

	routePolicy.Reconfigure(policy.Config{})
	_, err = locator.Resolve(query)
	require.NoError(t, err)
}

func TestLocatorRejectsProjectionWorkBeyondItsFixedBudget(t *testing.T) {
	const (
		recordBudget   = 64
		endpointBudget = 256
	)

	t.Run("record scan", func(t *testing.T) {
		entries := make([]discoverytruth.Entry, 0, recordBudget+1)
		for index := 0; index <= recordBudget; index++ {
			entries = append(entries, serviceEntry(
				fmt.Sprintf("service-%03d", index), "echo", "NetworkPublished",
				fmt.Sprintf("https://192.0.2.%d:443", index%250+1),
			))
		}
		truth := &projectionTruth{entries: entries}
		locator, err := applicationdiscovery.NewLocator(truth)
		require.NoError(t, err)

		_, err = locator.Resolve(applicationdiscovery.Query{
			ServiceType: "echo", AcceptedSchemes: []string{"https"},
		})

		require.ErrorIs(t, err, applicationdiscovery.ErrInternal)
		require.Zero(t, truth.evaluateCalls)
		require.Empty(t, truth.policyCalls)
	})

	t.Run("endpoint scan", func(t *testing.T) {
		endpoints := make([]string, 0, endpointBudget+1)
		for index := 0; index <= endpointBudget; index++ {
			endpoints = append(endpoints, fmt.Sprintf(
				"https://192.0.2.%d:%d", index%250+1, 1024+index,
			))
		}
		truth := &projectionTruth{entries: []discoverytruth.Entry{
			serviceEntry("service-a", "echo", "NetworkPublished", endpoints...),
		}}
		locator, err := applicationdiscovery.NewLocator(truth)
		require.NoError(t, err)

		_, err = locator.Resolve(applicationdiscovery.Query{
			ServiceType: "echo", AcceptedSchemes: []string{"https"},
		})

		require.ErrorIs(t, err, applicationdiscovery.ErrInternal)
		require.Zero(t, truth.evaluateCalls)
		require.Empty(t, truth.policyCalls)
	})
}

func TestLocatorReportsMaintainedTruthBeforeReadinessAsUnavailable(t *testing.T) {
	store := discoverytruth.New("")
	truth, err := applicationdiscovery.NewMaintainedTruth(
		store, discoverytruth.NewTrustEvaluator(nil), allowRoutePolicy{},
	)
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

func (unavailableTruth) AllowRouteUse(network.Candidate) error {
	return applicationdiscovery.ErrUnavailable
}

type allowRoutePolicy struct{}

func (allowRoutePolicy) AllowRouteUse(network.Candidate) error {
	return nil
}

type projectionTruth struct {
	entries       []discoverytruth.Entry
	trust         discoverytruth.TrustResult
	findErr       error
	evaluateErr   error
	evaluateCalls int
	policyErr     error
	policyCalls   []network.Candidate
}

func (t *projectionTruth) FindService(string) ([]discoverytruth.Entry, error) {
	if t.findErr != nil {
		return nil, t.findErr
	}
	return append([]discoverytruth.Entry(nil), t.entries...), nil
}

func (t *projectionTruth) Evaluate(discoverytruth.Record) (discoverytruth.TrustResult, error) {
	t.evaluateCalls++
	if t.evaluateErr != nil {
		return discoverytruth.TrustResult{}, t.evaluateErr
	}
	if t.trust == (discoverytruth.TrustResult{}) {
		return discoverytruth.TrustResult{Valid: true, Trusted: true, Usable: true}, nil
	}
	return t.trust, nil
}

func (t *projectionTruth) AllowRouteUse(candidate network.Candidate) error {
	t.policyCalls = append(t.policyCalls, candidate)
	return t.policyErr
}

type sideEffectTruth struct {
	projectionTruth
	observationCalls int
	refreshCalls     int
	probeCalls       int
	fetchCalls       int
	dialCalls        int
	networkCalls     int
}

func (t *sideEffectTruth) ObserveWorkloads() {
	t.observationCalls++
}

func (t *sideEffectTruth) RefreshDiscovery() {
	t.refreshCalls++
}

func (t *sideEffectTruth) ProbeEndpoint(string) {
	t.probeCalls++
}

func (t *sideEffectTruth) FetchRemoteRecords() {
	t.fetchCalls++
}

func (t *sideEffectTruth) DialEndpoint(string) {
	t.dialCalls++
}

func (t *sideEffectTruth) PerformNetworkWork() {
	t.networkCalls++
}

func serviceEntry(serviceID, serviceType, mode string, endpoints ...string) discoverytruth.Entry {
	return discoverytruth.Entry{Record: discoverytruth.Record{Service: &discoveryrecord.ServiceFacts{
		ID: discoveryrecord.ServiceID(serviceID), Type: serviceType, Mode: mode,
		Endpoints: append([]string(nil), endpoints...),
	}}}
}
