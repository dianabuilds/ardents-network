package transport

import (
	"testing"
	"time"

	discovery "ardents/internal/discovery"
	networkreadiness "ardents/internal/network/readiness"

	"github.com/stretchr/testify/require"
)

func TestBuildCandidates(t *testing.T) {
	svc := New()
	record := discovery.Record{
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

	candidates := svc.BuildCandidates(discovery.Record{
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
	svc := New(Config{
		ReachabilityMode:   networkreadiness.ReachabilityPublicDirect,
		AdvertiseAddresses: []string{"/dns4/node.example/tcp/61000"},
	})
	svc.state = "ready"
	svc.endpoints = []string{"/dns4/node.example/tcp/61000/p2p/peer"}
	svc.observed = newEndpointObservations(svc.endpoints, true)

	snapshot := svc.ProfileSnapshot()
	require.Equal(t, networkreadiness.HealthStateDegraded, snapshot.Health)
	require.Contains(t, snapshot.ReducedCapabilities, "inbound_reachability")
}

func TestOutboundOnlyProfileDoesNotRequirePublishedIngressEndpoint(t *testing.T) {
	svc := New(Config{ReachabilityMode: networkreadiness.ReachabilityOutboundOnly})
	svc.state = "ready"

	snapshot := svc.ProfileSnapshot()
	require.Equal(t, networkreadiness.HealthStateReady, snapshot.Health)
}

func TestProfileSnapshotReportsTCPOnlyBaseline(t *testing.T) {
	svc := New()
	svc.state = "ready"
	svc.bootstrap = networkreadiness.BootstrapStatus{State: "ready", Joined: true}
	svc.endpoints = []string{"/ip4/127.0.0.1/tcp/9000"}
	svc.observed = map[string]endpointObservation{
		svc.endpoints[0]: {usable: true},
	}

	snapshot := svc.ProfileSnapshot()
	require.Equal(t, networkreadiness.ProfileTCPOnly, snapshot.Profile)
	require.Equal(t, networkreadiness.ModeSteady, snapshot.Mode)
	require.Equal(t, networkreadiness.HealthStateReady, snapshot.Health)
	require.Equal(t, []networkreadiness.Family{networkreadiness.FamilyTCP}, snapshot.ActiveFamilies)
	require.Equal(t, networkreadiness.SwitchReasonStartupDefault, snapshot.SwitchReason)
}

func TestProfileSnapshotReportsBootstrapDegradation(t *testing.T) {
	svc := New()
	svc.state = "degraded"
	svc.reason = "bootstrap peers unavailable"
	svc.bootstrapNodes = []string{"/ip4/127.0.0.1/tcp/9000"}
	svc.bootstrap = networkreadiness.BootstrapStatus{State: "degraded", Reason: svc.reason}
	svc.endpoints = []string{"/ip4/127.0.0.1/tcp/9001"}
	svc.observed = map[string]endpointObservation{
		svc.endpoints[0]: {usable: true},
	}

	snapshot := svc.ProfileSnapshot()
	require.Equal(t, networkreadiness.HealthStateDegraded, snapshot.Health)
	require.Equal(t, networkreadiness.ModeSteady, snapshot.Mode)
	require.Equal(t, networkreadiness.SwitchReasonBootstrapDegraded, snapshot.SwitchReason)
	require.True(t, snapshot.SwitchAutomatic)
	require.Equal(t, networkreadiness.RecoveryStateRecoveryPending, snapshot.RecoveryState)
	require.NotEmpty(t, snapshot.ReducedCapabilities)
}

func TestProfileSnapshotFailsForUnimplementedProfile(t *testing.T) {
	svc := New(Config{Profile: networkreadiness.ProfileTCPQUIC})

	snapshot := svc.ProfileSnapshot()
	require.Equal(t, networkreadiness.ProfileTCPQUIC, snapshot.Profile)
	require.Equal(t, networkreadiness.ModeSteady, snapshot.Mode)
	require.Equal(t, networkreadiness.HealthStateFailed, snapshot.Health)
	require.Equal(t, networkreadiness.SwitchReasonStartupFailed, snapshot.SwitchReason)
	require.Equal(t, networkreadiness.RecoveryStateBlocked, snapshot.RecoveryState)
}

func TestProfileSnapshotClonesSlices(t *testing.T) {
	svc := New()
	svc.state = "ready"
	svc.bootstrap = networkreadiness.BootstrapStatus{State: "ready", Joined: true}
	svc.endpoints = []string{"/ip4/127.0.0.1/tcp/9000"}
	svc.observed = map[string]endpointObservation{
		svc.endpoints[0]: {usable: true},
	}

	snapshot := svc.ProfileSnapshot()
	snapshot.ActiveFamilies[0] = networkreadiness.FamilyWSS
	snapshot.SuppressedFamilies[0] = networkreadiness.FamilyTCP
	endpoints := svc.Endpoints()
	endpoints[0] = "mutated"

	fresh := svc.ProfileSnapshot()
	require.Equal(t, networkreadiness.FamilyTCP, fresh.ActiveFamilies[0])
	require.Equal(t, networkreadiness.FamilyQUIC, fresh.SuppressedFamilies[0])
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
	svc.bootstrap = networkreadiness.BootstrapStatus{State: "degraded", Reason: svc.reason}
	svc.endpoints = []string{"/ip4/127.0.0.1/tcp/9000"}
	svc.observed = map[string]endpointObservation{
		svc.endpoints[0]: {usable: true},
	}

	for i := 0; i < 3; i++ {
		svc.mu.Lock()
		svc.reconcileRuntimeLocked(now)
		svc.mu.Unlock()
		snapshot := svc.ProfileSnapshot()
		if i < 2 {
			require.Equal(t, networkreadiness.ProfileTCPOnly, snapshot.Profile)
		}
	}

	snapshot := svc.ProfileSnapshot()
	require.Equal(t, networkreadiness.ProfileTCPOnly, snapshot.Profile)
	require.Equal(t, networkreadiness.ModeRestrictedDefense, snapshot.Mode)
	require.Equal(t, networkreadiness.HealthStateDegraded, snapshot.Health)
	require.Equal(t, networkreadiness.SwitchReasonHealthDegraded, snapshot.SwitchReason)
	require.Contains(t, []networkreadiness.RecoveryState{networkreadiness.RecoveryStateRecoveryPending, networkreadiness.RecoveryStateCooldownActive}, snapshot.RecoveryState)
	require.Equal(t, []networkreadiness.Family{networkreadiness.FamilyTCP}, snapshot.ActiveFamilies)
}

func TestProfileSnapshotRecoversFromRestrictedDefense(t *testing.T) {
	now := time.Date(2026, 3, 24, 12, 0, 0, 0, time.UTC)
	restore := overrideTransportTime(now)
	defer restore()

	svc := New()
	svc.state = "ready"
	svc.reason = "bootstrap peers unavailable"
	svc.bootstrapNodes = []string{"/ip4/127.0.0.1/tcp/9000"}
	svc.bootstrap = networkreadiness.BootstrapStatus{State: "degraded", Reason: svc.reason}
	svc.endpoints = []string{"/ip4/127.0.0.1/tcp/9000"}
	svc.observed = map[string]endpointObservation{
		svc.endpoints[0]: {usable: true},
	}

	for i := 0; i < 4; i++ {
		svc.mu.Lock()
		svc.reconcileRuntimeLocked(now)
		svc.mu.Unlock()
		_ = svc.ProfileSnapshot()
	}

	restore()
	restore = overrideTransportTime(now.Add(31 * time.Second))
	svc.bootstrap = networkreadiness.BootstrapStatus{State: "ready", Joined: true}
	svc.reason = ""

	svc.mu.Lock()
	svc.reconcileRuntimeLocked(now.Add(31 * time.Second))
	svc.mu.Unlock()
	firstReady := svc.ProfileSnapshot()
	require.Equal(t, networkreadiness.ProfileTCPOnly, firstReady.Profile)
	require.Equal(t, networkreadiness.ModeRestrictedDefense, firstReady.Mode)

	restore()
	restore = overrideTransportTime(now.Add(32 * time.Second))

	svc.mu.Lock()
	svc.reconcileRuntimeLocked(now.Add(32 * time.Second))
	svc.mu.Unlock()
	recovered := svc.ProfileSnapshot()
	require.Equal(t, networkreadiness.ProfileTCPOnly, recovered.Profile)
	require.Equal(t, networkreadiness.ModeSteady, recovered.Mode)
	require.Equal(t, networkreadiness.HealthStateReady, recovered.Health)
	require.Equal(t, networkreadiness.SwitchReasonRecoveryReady, recovered.SwitchReason)
}

func overrideTransportTime(now time.Time) func() {
	prev := timeNowUTC
	timeNowUTC = func() time.Time { return now }
	return func() {
		timeNowUTC = prev
	}
}
