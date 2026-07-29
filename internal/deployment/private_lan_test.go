package deployment

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPrivateLANCoordinatorFormsDeterministicCrossHostTopology(t *testing.T) {
	raw := readTopologyFixture(t, "private-lan.json")
	now := time.Date(2026, 7, 29, 13, 0, 0, 0, time.UTC)
	hosts := &privateLANHostsFake{}
	dialer := &privateLANDialerFake{observedAt: now}
	status := &privateLANStatusFake{observation: readyPrivateLANObservation(4, 1)}
	coordinator := PrivateLANCoordinator{
		Hosts: hosts, Dialer: dialer, Status: status,
		StepTimeout: time.Second, Now: func() time.Time { return now },
	}

	result, err := coordinator.Reconcile(context.Background(), raw)
	require.NoError(t, err)
	require.Equal(t, PrivateLANResult{
		APIVersion: PrivateLANResultVersion,
		Outcome:    PrivateLANOutcomeReady,
		NodesReady: 3, RetainedStoreFetches: 4, StoreGaps: 1,
	}, result)
	require.Equal(t, []string{"node-a", "node-b", "node-c"}, hosts.appliedSlots())
	require.Equal(t, []string{
		"node-b->node-a", "node-c->node-b", "node-a->node-c",
	}, dialer.routes())
	require.Equal(t, []string{
		"node-b->node-a", "node-c->node-b", "node-a->node-c",
	}, hosts.probeRoutes())
	for _, target := range hosts.applied {
		require.Len(t, target.StaticRecoveryPeers, 2)
		require.Equal(t, "private_probe_required", target.Plan.Ingress)
		require.NotEmpty(t, target.ManifestDigest)
	}

	encoded, err := json.Marshal(result)
	require.NoError(t, err)
	for _, protected := range []string{
		"10.23.0.", "ssh-node-", "host-pin-", "p1_", "12D3Koo",
		"registry.example", "operator-primary",
	} {
		require.NotContains(t, string(encoded), protected)
	}
}

func TestPrivateLANCoordinatorRejectsBeforeMutationAndFailsClosed(t *testing.T) {
	public := readTopologyFixture(t, "public-direct.json")
	hosts := &privateLANHostsFake{}
	dialer := &privateLANDialerFake{observedAt: time.Now().UTC()}
	status := &privateLANStatusFake{}
	coordinator := PrivateLANCoordinator{
		Hosts: hosts, Dialer: dialer, Status: status, StepTimeout: time.Second,
	}
	_, err := coordinator.Reconcile(context.Background(), public)
	require.ErrorContains(t, err, "private_lan")
	require.Empty(t, hosts.applied)
	require.Empty(t, dialer.targets)
	require.Zero(t, status.calls)

	coordinator.Hosts = nil
	_, err = coordinator.Reconcile(context.Background(), readTopologyFixture(t, "private-lan.json"))
	require.ErrorContains(t, err, "required")
	require.Empty(t, hosts.applied)
}

func TestPrivateLANCoordinatorReconcilesPartitionDNSAndStoreGap(t *testing.T) {
	raw := readTopologyFixture(t, "private-lan.json")
	now := time.Date(2026, 7, 29, 13, 0, 0, 0, time.UTC)
	hosts := &privateLANHostsFake{}
	dialer := &privateLANDialerFake{
		observedAt: now,
		failures:   map[string]error{"node-b": errors.New("segment unavailable")},
	}
	status := &privateLANStatusFake{observation: readyPrivateLANObservation(2, 2)}
	coordinator := PrivateLANCoordinator{
		Hosts: hosts, Dialer: dialer, Status: status,
		StepTimeout: time.Second, Now: func() time.Time { return now },
	}

	result, err := coordinator.Reconcile(context.Background(), raw)
	require.Error(t, err)
	require.Equal(t, PrivateLANOutcomeRecoveryRequired, result.Outcome)
	require.Equal(t, PrivateLANReasonProbeFailed, result.Reason)
	require.Zero(t, status.calls)

	delete(dialer.failures, "node-b")
	result, err = coordinator.Reconcile(context.Background(), raw)
	require.NoError(t, err)
	require.Equal(t, PrivateLANOutcomeReady, result.Outcome)
	require.Equal(t, 2, result.StoreGaps)
	require.Len(t, hosts.applied, 6, "recovery reapplies all exact host plans")
	require.Len(t, dialer.targets, 5, "failed pass stops before the third probe")
}

func TestPrivateLANCoordinatorReappliesExactTopologyAfterRecoverableDegradation(t *testing.T) {
	base := readTopologyFixture(t, "private-lan.json")
	const dnsRoot = "enrtree://AKPYQIUQIL7PSIACI32J7FGZW56E5FKHEFCCOFHILBIMW3M6LWXS2@nodes.example.org"
	raw := bytes.Replace(
		base,
		[]byte(`"signed_dns_roots": []`),
		[]byte(`"signed_dns_roots": ["`+dnsRoot+`"]`),
		1,
	)
	now := time.Date(2026, 7, 29, 13, 0, 0, 0, time.UTC)
	for _, cause := range []string{
		"restart", "bootstrap_loss", "peer_churn", "dns_outage", "dns_replacement",
	} {
		t.Run(cause, func(t *testing.T) {
			degraded := readyPrivateLANObservation(0, 1)
			degraded.Status.Outcome = TopologyOutcomeDegraded
			degraded.Status.Nodes[1].Ready = false
			hosts := &privateLANHostsFake{}
			status := &privateLANStatusFake{observations: []PrivateLANObservation{
				degraded,
				readyPrivateLANObservation(3, 1),
			}}
			coordinator := PrivateLANCoordinator{
				Hosts:  hosts,
				Dialer: &privateLANDialerFake{observedAt: now},
				Status: status, StepTimeout: time.Second,
				Now: func() time.Time { return now },
			}

			first, err := coordinator.Reconcile(context.Background(), raw)
			require.Error(t, err)
			require.Equal(t, PrivateLANReasonStatusDegraded, first.Reason)

			second, err := coordinator.Reconcile(context.Background(), raw)
			require.NoError(t, err)
			require.Equal(t, PrivateLANOutcomeReady, second.Outcome)
			require.Equal(t, 1, second.StoreGaps)
			require.Len(t, hosts.applied, 6)
			for _, target := range hosts.applied {
				require.Equal(t, []string{dnsRoot}, target.SignedDNSRoots)
				require.Len(t, target.StaticRecoveryPeers, 2)
			}
		})
	}
}

func TestPrivateLANCoordinatorRejectsUnboundOrStaleProbeObservation(t *testing.T) {
	raw := readTopologyFixture(t, "private-lan.json")
	now := time.Date(2026, 7, 29, 13, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		name string
		at   time.Time
	}{
		{name: "zero", at: time.Time{}},
		{name: "stale", at: now.Add(-privateLANProbeMaxAge - time.Second)},
		{name: "future", at: now.Add(privateLANProbeFutureSkew + time.Second)},
	} {
		t.Run(test.name, func(t *testing.T) {
			coordinator := PrivateLANCoordinator{
				Hosts:       &privateLANHostsFake{},
				Dialer:      &privateLANDialerFake{observedAt: test.at},
				Status:      &privateLANStatusFake{},
				StepTimeout: time.Second, Now: func() time.Time { return now },
			}
			result, err := coordinator.Reconcile(context.Background(), raw)
			require.Error(t, err)
			require.Equal(t, PrivateLANReasonProbeInvalid, result.Reason)
		})
	}
}

func readyPrivateLANObservation(retained, gaps int) PrivateLANObservation {
	nodes := make([]NodeStatus, 0, 3)
	for index, slot := range []string{"node-a", "node-b", "node-c"} {
		store := NodeTruthNotRequired
		if index < 2 {
			store = NodeTruthReady
		}
		nodes = append(nodes, NodeStatus{
			Slot: slot, Observation: NodeObservationComplete,
			Ready: true, Readiness: NodeTruthReady, Joined: true,
			Reachability: NodeTruthReady, Store: store, Image: NodeImageMatch,
		})
	}
	return PrivateLANObservation{
		Status: TopologyStatus{
			APIVersion: TopologyStatusVersion,
			Outcome:    TopologyOutcomeReady,
			Nodes:      nodes,
		},
		RetainedStoreFetches: retained,
		StoreGaps:            gaps,
	}
}

type privateLANHostsFake struct {
	applied []PrivateLANHostTarget
	probes  []PrivateLANProof
}

func (fake *privateLANHostsFake) Apply(
	_ context.Context,
	target PrivateLANHostTarget,
) error {
	fake.applied = append(fake.applied, target)
	return nil
}

func (fake *privateLANHostsFake) ApplyProbe(
	_ context.Context,
	_ PrivateLANHostTarget,
	probe PrivateLANProof,
) error {
	fake.probes = append(fake.probes, probe)
	return nil
}

func (fake *privateLANHostsFake) appliedSlots() []string {
	out := make([]string, 0, len(fake.applied))
	for _, target := range fake.applied {
		out = append(out, target.Slot)
	}
	return out
}

func (fake *privateLANHostsFake) probeRoutes() []string {
	out := make([]string, 0, len(fake.probes))
	for _, probe := range fake.probes {
		out = append(out, probe.SourceSlot+"->"+probe.TargetSlot)
	}
	return out
}

type privateLANDialerFake struct {
	observedAt time.Time
	failures   map[string]error
	targets    []PrivateLANProbeTarget
}

func (fake *privateLANDialerFake) Probe(
	_ context.Context,
	target PrivateLANProbeTarget,
) (time.Time, error) {
	fake.targets = append(fake.targets, target)
	return fake.observedAt, fake.failures[target.TargetSlot]
}

func (fake *privateLANDialerFake) routes() []string {
	out := make([]string, 0, len(fake.targets))
	for _, target := range fake.targets {
		out = append(out, target.SourceSlot+"->"+target.TargetSlot)
	}
	return out
}

type privateLANStatusFake struct {
	observation  PrivateLANObservation
	observations []PrivateLANObservation
	err          error
	calls        int
}

func (fake *privateLANStatusFake) Observe(
	context.Context,
	[]byte,
) (PrivateLANObservation, error) {
	fake.calls++
	if len(fake.observations) > 0 {
		observation := fake.observations[0]
		fake.observations = fake.observations[1:]
		return observation, fake.err
	}
	return fake.observation, fake.err
}
