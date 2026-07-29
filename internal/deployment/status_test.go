package deployment

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestStatusInspectorReturnsDeterministicReadyTopologyWithoutProtectedIdentifiers(t *testing.T) {
	raw := readTopologyFixture(t, "public-direct.json")
	probe := &statusProbeFake{observations: readyPublicObservations()}
	inspector := StatusInspector{Probe: probe, PerNodeTimeout: time.Second}

	status, err := inspector.Inspect(context.Background(), raw)
	require.NoError(t, err)
	require.Equal(t, TopologyStatus{
		APIVersion: TopologyStatusVersion,
		Outcome:    TopologyOutcomeReady,
		Nodes: []NodeStatus{
			{
				Slot: "node-a", Role: "authority", Observation: NodeObservationComplete,
				Ready: true, Readiness: NodeTruthReady, Joined: true,
				Reachability: NodeTruthReady, Store: NodeTruthReady,
				Image: NodeImageMatch,
			},
			{
				Slot: "node-b", Role: "member", Observation: NodeObservationComplete,
				Ready: true, Readiness: NodeTruthReady, Joined: true,
				Reachability: NodeTruthReady, Store: NodeTruthReady,
				Image: NodeImageMatch,
			},
			{
				Slot: "node-c", Role: "member", Observation: NodeObservationComplete,
				Ready: true, Readiness: NodeTruthReady, Joined: true,
				Reachability: NodeTruthReady, Store: NodeTruthNotRequired,
				Image: NodeImageMatch,
			},
		},
	}, status)
	require.Equal(t, []string{"node-a", "node-b", "node-c"}, probe.slots())

	encoded, err := json.Marshal(status)
	require.NoError(t, err)
	for _, protected := range []string{
		"ssh-node-", "host-pin-", "operator-primary", "p1_", "12D3Koo",
		"registry.example", "sha256:", "node-a.ardents.net", "1.1.1.1",
	} {
		require.NotContains(t, string(encoded), protected)
	}
}

func TestStatusInspectorRetainsDistinctPartialFailureClassesAndContinues(t *testing.T) {
	raw := readTopologyFixture(t, "public-direct.json")
	for _, code := range []ProbeErrorCode{
		ProbeHostKeyMismatch,
		ProbeTunnelTimeout,
		ProbeTunnelFailure,
		ProbeLocalSignerUnavailable,
		ProbeRemoteUnauthenticated,
		ProbeRemoteDenied,
		ProbeNodeUnavailable,
		ProbeRemoteInvalidResponse,
	} {
		t.Run(string(code), func(t *testing.T) {
			probe := &statusProbeFake{
				observations: readyPublicObservations(),
				errors:       map[string]error{"node-b": ProbeError(code)},
			}
			status, err := (StatusInspector{Probe: probe, PerNodeTimeout: time.Second}).
				Inspect(context.Background(), raw)
			require.NoError(t, err)
			require.Equal(t, TopologyOutcomePartial, status.Outcome)
			require.Len(t, status.Nodes, 3)
			require.Equal(t, NodeObservationUnavailable, status.Nodes[1].Observation)
			require.Equal(t, StatusReason(code), status.Nodes[1].Reason)
			require.False(t, status.Nodes[1].Ready)
			require.Equal(t, []string{"node-a", "node-b", "node-c"}, probe.slots())
		})
	}
}

func TestStatusInspectorDegradesObservedIdentityImageReadinessReachabilityAndStoreTruth(t *testing.T) {
	raw := readTopologyFixture(t, "public-direct.json")
	tests := []struct {
		name   string
		mutate func(map[string]NodeObservation)
		reason StatusReason
	}{
		{
			name: "node identity mismatch",
			mutate: func(observations map[string]NodeObservation) {
				item := observations["node-a"]
				item.NodePrincipal = observations["node-b"].NodePrincipal
				observations["node-a"] = item
			},
			reason: StatusReasonNodeIdentityMismatch,
		},
		{
			name: "image mismatch",
			mutate: func(observations map[string]NodeObservation) {
				item := observations["node-a"]
				item.ImageReference = "registry.example/ardents/node@sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
				observations["node-a"] = item
			},
			reason: StatusReasonImageMismatch,
		},
		{
			name: "image unverified",
			mutate: func(observations map[string]NodeObservation) {
				item := observations["node-a"]
				item.ImageReference = ""
				observations["node-a"] = item
			},
			reason: StatusReasonImageUnverified,
		},
		{
			name: "composite readiness degraded",
			mutate: func(observations map[string]NodeObservation) {
				item := observations["node-a"]
				item.RuntimeReady = false
				item.RuntimeReason = "network"
				observations["node-a"] = item
			},
			reason: StatusReasonCompositeReadinessDegraded,
		},
		{
			name: "public reachability unavailable",
			mutate: func(observations map[string]NodeObservation) {
				item := observations["node-a"]
				item.ReachabilityState = "unknown"
				item.Reachable = false
				observations["node-a"] = item
			},
			reason: StatusReasonPublicReachabilityUnverified,
		},
		{
			name: "persistent store unavailable",
			mutate: func(observations map[string]NodeObservation) {
				item := observations["node-a"]
				item.StoreEnabled = false
				observations["node-a"] = item
			},
			reason: StatusReasonPersistentStoreUnavailable,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			observations := readyPublicObservations()
			tc.mutate(observations)
			status, err := (StatusInspector{
				Probe: &statusProbeFake{observations: observations}, PerNodeTimeout: time.Second,
			}).Inspect(context.Background(), raw)
			require.NoError(t, err)
			require.Equal(t, TopologyOutcomeDegraded, status.Outcome)
			require.False(t, status.Nodes[0].Ready)
			require.Equal(t, tc.reason, status.Nodes[0].Reason)
		})
	}
}

func TestStatusInspectorValidatesManifestBeforeObservationAndBoundsEachNode(t *testing.T) {
	probe := &statusProbeFake{}
	_, err := (StatusInspector{Probe: probe, PerNodeTimeout: time.Second}).
		Inspect(context.Background(), []byte(`{"api_version":"ardents.topology/v2"}`))
	require.EqualError(t, err, "topology_missing_field")
	require.Empty(t, probe.slots())

	raw := readTopologyFixture(t, "public-direct.json")
	blocking := &statusProbeFake{
		observations: readyPublicObservations(),
		wait:         map[string]bool{"node-b": true},
	}
	status, err := (StatusInspector{
		Probe: blocking, PerNodeTimeout: 20 * time.Millisecond,
	}).Inspect(context.Background(), raw)
	require.NoError(t, err)
	require.Equal(t, TopologyOutcomePartial, status.Outcome)
	require.Equal(t, StatusReason(ProbeTunnelTimeout), status.Nodes[1].Reason)
	require.Equal(t, []string{"node-a", "node-b", "node-c"}, blocking.slots())
}

func TestProjectNodeStatusAcceptsPrivateLANScopedListenerTruth(t *testing.T) {
	target := NodeStatusTarget{
		Slot: "node-a", Role: "authority", ExpectedIngress: "private_lan",
		ExpectedNodePrincipal: "p1_euydwrsrlrtxe7misopktnf7zlk6b27waegboirnhbbu4wlen55a",
		ExpectedImage:         "registry.example/ardents/node@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		PersistentStore:       true,
	}
	observation := readyPublicObservations()["node-a"]
	observation.ReachabilityMode = "private_lan"
	observation.ReachabilityState = "lan"
	status := projectNodeStatus(target, observation)
	require.True(t, status.Ready)
	require.Equal(t, NodeTruthReady, status.Reachability)
}

func readyPublicObservations() map[string]NodeObservation {
	return map[string]NodeObservation{
		"node-a": {
			NodeName:      "node-a",
			NodePrincipal: "p1_euydwrsrlrtxe7misopktnf7zlk6b27waegboirnhbbu4wlen55a",
			RuntimeReady:  true, Joined: true, ReachabilityMode: "public_direct",
			ReachabilityState: "public", Reachable: true,
			StoreEnabled: true, StoreState: "ready",
			ImageReference: "registry.example/ardents/node@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
		"node-b": {
			NodeName:      "node-b",
			NodePrincipal: "p1_jjkwa23wqggjpivnxdb45wpe575akea3eyytyr2slvuhg7ujsspq",
			RuntimeReady:  true, Joined: true, ReachabilityMode: "public_direct",
			ReachabilityState: "public", Reachable: true,
			StoreEnabled: true, StoreState: "ready",
			ImageReference: "registry.example/ardents/node@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		},
		"node-c": {
			NodeName:      "node-c",
			NodePrincipal: "p1_n55ilee3u2y3zr6s3xuph7qjcqpsunkajnlgc3dxqkgzri5oxhca",
			RuntimeReady:  true, Joined: true, ReachabilityMode: "outbound_only",
			ReachabilityState: "outbound_only", Reachable: false,
			StoreEnabled: false, StoreState: "disabled",
			ImageReference: "registry.example/ardents/node@sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
		},
	}
}

type statusProbeFake struct {
	mu           sync.Mutex
	observations map[string]NodeObservation
	errors       map[string]error
	wait         map[string]bool
	targets      []NodeStatusTarget
}

func (f *statusProbeFake) Observe(ctx context.Context, target NodeStatusTarget) (NodeObservation, error) {
	f.mu.Lock()
	f.targets = append(f.targets, target)
	f.mu.Unlock()
	if f.wait[target.Slot] {
		<-ctx.Done()
		return NodeObservation{}, ctx.Err()
	}
	if err := f.errors[target.Slot]; err != nil {
		return NodeObservation{}, err
	}
	value, ok := f.observations[target.Slot]
	if !ok {
		return NodeObservation{}, errors.New("unexpected probe target")
	}
	return value, nil
}

func (f *statusProbeFake) slots() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.targets))
	for index, target := range f.targets {
		out[index] = target.Slot
	}
	return out
}
