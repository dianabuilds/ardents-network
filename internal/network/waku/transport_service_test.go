package waku

import (
	"ardents/internal/network"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestBuildCandidates(t *testing.T) {
	svc := New()
	record := network.RouteRecord{
		Subject:   "svc.local.echo",
		Service:   "echo",
		Mode:      "NetworkPublished",
		Endpoints: []string{"quic://node:9000", "relay://node/echo"},
	}

	candidates := svc.BuildCandidates(record, true)
	require.Len(t, candidates, 2)
	require.Equal(t, "quic", candidates[0].Scheme)
	require.True(t, candidates[0].Trusted)
	require.False(t, candidates[0].Usable)
}

func TestBuildCandidatesUsesObservedMultiaddrReachability(t *testing.T) {
	endpoint := "/ip4/127.0.0.1/tcp/9000/p2p/test"
	svc := &Service{observed: map[string]endpointObservation{endpoint: {usable: true}}}

	candidates := svc.BuildCandidates(network.RouteRecord{
		Subject:   "node.local",
		Endpoints: []string{endpoint},
	}, true)
	require.Len(t, candidates, 1)
	require.True(t, candidates[0].Usable)
}

func TestHealthSignalsCountBootstrapPeersAndUsableEndpoints(t *testing.T) {
	svc := New()
	svc.state = "ready"
	svc.bootstrapNodes = []string{" ", "/ip4/127.0.0.1/tcp/9000"}
	svc.endpoints = []string{"/ip4/127.0.0.1/tcp/9000", "/ip4/127.0.0.1/tcp/9001"}
	svc.observed = map[string]endpointObservation{
		svc.endpoints[0]: {usable: true},
		svc.endpoints[1]: {usable: false},
	}

	signals := svc.HealthSignals()
	require.Equal(t, 1, signals.BootstrapSourceCount)
	require.Equal(t, 2, signals.EndpointCount)
	require.Equal(t, 1, signals.UsableEndpointCount)
}

func TestServicePartSnapshotStaysReadyWithoutBootstrapSources(t *testing.T) {
	svc := New()
	svc.state = "ready"
	svc.endpoints = []string{"/ip4/127.0.0.1/tcp/9000"}
	svc.observed = map[string]endpointObservation{
		svc.endpoints[0]: {usable: true},
	}

	part := svc.serviceStateLocked()
	require.Equal(t, "ready", part.State)
}

func TestPublicDirectProfileStaysDegradedUntilIngressIsVerified(t *testing.T) {
	svc := New(network.Config{
		ReachabilityMode:   network.ReachabilityPublicDirect,
		AdvertiseAddresses: []string{"/dns4/node.example/tcp/61000"},
	})
	svc.state = "ready"
	svc.endpoints = []string{"/dns4/node.example/tcp/61000/p2p/peer"}
	svc.observed = newEndpointObservations(svc.endpoints, true)

	snapshot := svc.ProfileSnapshot()
	require.Equal(t, network.HealthStateDegraded, snapshot.Health)
	require.Contains(t, snapshot.ReducedCapabilities, "inbound_reachability")
}

func TestOutboundOnlyProfileDoesNotRequirePublishedIngressEndpoint(t *testing.T) {
	svc := New(network.Config{ReachabilityMode: network.ReachabilityOutboundOnly})
	svc.state = "ready"

	snapshot := svc.ProfileSnapshot()
	require.Equal(t, network.HealthStateReady, snapshot.Health)
}

func TestProfileSnapshotReportsTCPOnlyBaseline(t *testing.T) {
	svc := New()
	svc.state = "ready"
	svc.bootstrap = network.BootstrapStatus{State: "ready", Joined: true}
	svc.endpoints = []string{"/ip4/127.0.0.1/tcp/9000"}
	svc.observed = map[string]endpointObservation{
		svc.endpoints[0]: {usable: true},
	}

	snapshot := svc.ProfileSnapshot()
	require.Equal(t, network.ProfileTCPOnly, snapshot.Profile)
	require.Equal(t, network.ModeSteady, snapshot.Mode)
	require.Equal(t, network.HealthStateReady, snapshot.Health)
	require.Equal(t, []network.Family{network.FamilyTCP}, snapshot.ActiveFamilies)
	require.Equal(t, network.SwitchReasonStartupDefault, snapshot.SwitchReason)
}

func TestProfileSnapshotReportsBootstrapDegradation(t *testing.T) {
	svc := New()
	svc.state = "degraded"
	svc.reason = "bootstrap peers unavailable"
	svc.bootstrapNodes = []string{"/ip4/127.0.0.1/tcp/9000"}
	svc.bootstrap = network.BootstrapStatus{State: "degraded", Reason: svc.reason}
	svc.endpoints = []string{"/ip4/127.0.0.1/tcp/9001"}
	svc.observed = map[string]endpointObservation{
		svc.endpoints[0]: {usable: true},
	}

	snapshot := svc.ProfileSnapshot()
	require.Equal(t, network.HealthStateDegraded, snapshot.Health)
	require.Equal(t, network.ModeSteady, snapshot.Mode)
	require.Equal(t, network.SwitchReasonBootstrapDegraded, snapshot.SwitchReason)
	require.True(t, snapshot.SwitchAutomatic)
	require.Equal(t, network.RecoveryStateRecoveryPending, snapshot.RecoveryState)
	require.NotEmpty(t, snapshot.ReducedCapabilities)
}

func TestProfileSnapshotFailsForUnimplementedProfile(t *testing.T) {
	svc := New(network.Config{Profile: network.ProfileTCPQUIC})

	snapshot := svc.ProfileSnapshot()
	require.Equal(t, network.ProfileTCPQUIC, snapshot.Profile)
	require.Equal(t, network.ModeSteady, snapshot.Mode)
	require.Equal(t, network.HealthStateFailed, snapshot.Health)
	require.Equal(t, network.SwitchReasonStartupFailed, snapshot.SwitchReason)
	require.Equal(t, network.RecoveryStateBlocked, snapshot.RecoveryState)
}

func TestProfileSnapshotClonesSlices(t *testing.T) {
	svc := New()
	svc.state = "ready"
	svc.bootstrap = network.BootstrapStatus{State: "ready", Joined: true}
	svc.endpoints = []string{"/ip4/127.0.0.1/tcp/9000"}
	svc.observed = map[string]endpointObservation{
		svc.endpoints[0]: {usable: true},
	}

	snapshot := svc.ProfileSnapshot()
	snapshot.ActiveFamilies[0] = network.FamilyWSS
	snapshot.SuppressedFamilies[0] = network.FamilyTCP
	endpoints := svc.Endpoints()
	endpoints[0] = "mutated"

	fresh := svc.ProfileSnapshot()
	require.Equal(t, network.FamilyTCP, fresh.ActiveFamilies[0])
	require.Equal(t, network.FamilyQUIC, fresh.SuppressedFamilies[0])
	require.Equal(t, "/ip4/127.0.0.1/tcp/9000", svc.endpoints[0])
}

func TestProfileSnapshotTransitionsToRestrictedDefenseAfterRepeatedDegradation(t *testing.T) {
	now := time.Date(2026, 3, 24, 12, 0, 0, 0, time.UTC)
	restore := overrideTransportTime(now)
	defer restore()

	svc := New()
	svc.state = "ready"
	svc.reason = "bootstrap peers unavailable"
	svc.bootstrapNodes = []string{"/ip4/127.0.0.1/tcp/9000"}
	svc.bootstrap = network.BootstrapStatus{State: "degraded", Reason: svc.reason}
	svc.endpoints = []string{"/ip4/127.0.0.1/tcp/9000"}
	svc.observed = map[string]endpointObservation{
		svc.endpoints[0]: {usable: true},
	}

	for i := range 3 {
		svc.mu.Lock()
		svc.reconcileRuntimeLocked(now)
		svc.mu.Unlock()
		snapshot := svc.ProfileSnapshot()
		if i < 2 {
			require.Equal(t, network.ProfileTCPOnly, snapshot.Profile)
		}
	}

	snapshot := svc.ProfileSnapshot()
	require.Equal(t, network.ProfileTCPOnly, snapshot.Profile)
	require.Equal(t, network.ModeRestrictedDefense, snapshot.Mode)
	require.Equal(t, network.HealthStateDegraded, snapshot.Health)
	require.Equal(t, network.SwitchReasonHealthDegraded, snapshot.SwitchReason)
	require.Contains(t, []network.RecoveryState{network.RecoveryStateRecoveryPending, network.RecoveryStateCooldownActive}, snapshot.RecoveryState)
	require.Equal(t, []network.Family{network.FamilyTCP}, snapshot.ActiveFamilies)
}

func TestProfileSnapshotRecoversFromRestrictedDefense(t *testing.T) {
	now := time.Date(2026, 3, 24, 12, 0, 0, 0, time.UTC)
	restore := overrideTransportTime(now)
	defer restore()

	svc := New()
	svc.state = "ready"
	svc.reason = "bootstrap peers unavailable"
	svc.bootstrapNodes = []string{"/ip4/127.0.0.1/tcp/9000"}
	svc.bootstrap = network.BootstrapStatus{State: "degraded", Reason: svc.reason}
	svc.endpoints = []string{"/ip4/127.0.0.1/tcp/9000"}
	svc.observed = map[string]endpointObservation{
		svc.endpoints[0]: {usable: true},
	}

	for range 4 {
		svc.mu.Lock()
		svc.reconcileRuntimeLocked(now)
		svc.mu.Unlock()
		_ = svc.ProfileSnapshot()
	}

	restore()
	restore = overrideTransportTime(now.Add(31 * time.Second))
	svc.bootstrap = network.BootstrapStatus{State: "ready", Joined: true}
	svc.reason = ""

	svc.mu.Lock()
	svc.reconcileRuntimeLocked(now.Add(31 * time.Second))
	svc.mu.Unlock()
	firstReady := svc.ProfileSnapshot()
	require.Equal(t, network.ProfileTCPOnly, firstReady.Profile)
	require.Equal(t, network.ModeRestrictedDefense, firstReady.Mode)

	restore()
	restore = overrideTransportTime(now.Add(32 * time.Second))

	svc.mu.Lock()
	svc.reconcileRuntimeLocked(now.Add(32 * time.Second))
	svc.mu.Unlock()
	recovered := svc.ProfileSnapshot()
	require.Equal(t, network.ProfileTCPOnly, recovered.Profile)
	require.Equal(t, network.ModeSteady, recovered.Mode)
	require.Equal(t, network.HealthStateReady, recovered.Health)
	require.Equal(t, network.SwitchReasonRecoveryReady, recovered.SwitchReason)
}

func overrideTransportTime(now time.Time) func() {
	prev := timeNowUTC
	timeNowUTC = func() time.Time { return now }
	return func() {
		timeNowUTC = prev
	}
}
