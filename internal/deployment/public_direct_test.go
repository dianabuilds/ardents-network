package deployment

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPublicDirectCoordinatorReconcilesExactTopology(t *testing.T) {
	raw := readTopologyFixture(t, "public-direct.json")
	now := time.Date(2026, 7, 29, 15, 0, 0, 0, time.UTC)
	preflight := &publicDirectPreflightFake{now: now}
	hosts := &publicDirectHostsFake{}
	status := &publicDirectStatusFake{status: readyPublicDirectStatus()}
	coordinator := PublicDirectCoordinator{
		Preflight: preflight, Hosts: hosts, Status: status,
		StepTimeout: time.Second, Now: func() time.Time { return now },
	}

	result, err := coordinator.Reconcile(context.Background(), raw)
	require.NoError(t, err)
	require.Equal(t, PublicDirectResult{
		APIVersion:       PublicDirectResultVersion,
		Outcome:          PublicDirectOutcomeReady,
		PublicNodesReady: 2, OutboundNodesReady: 1,
	}, result)
	require.Equal(t, []string{"node-a", "node-b"}, preflight.slots())
	require.Equal(t, []string{"node-a", "node-b", "node-c"}, hosts.slots())
	require.Equal(t, 1, status.calls)
	for _, target := range hosts.targets {
		require.NotEmpty(t, target.ManifestDigest)
		require.NotEmpty(t, target.ConfigurationDigest)
		require.Len(t, target.Plan.StaticRecoveryPeers, 2)
		switch target.Slot {
		case "node-a":
			require.Equal(t, "/dns4/node-a.ardents.net/tcp/443/wss", target.Address)
			require.Equal(t, "wss-certificate-node-a", target.CertificateRef)
			require.Equal(t, "node-a.ardents.net", target.CertificateIdentity)
		case "node-b":
			require.Equal(t, "/ip4/1.1.1.1/tcp/60000", target.Address)
			require.Empty(t, target.CertificateRef)
			require.Empty(t, target.CertificateIdentity)
		case "node-c":
			require.Empty(t, target.Address)
			require.Empty(t, target.CertificateRef)
			require.Empty(t, target.CertificateIdentity)
		}
	}

	encoded, err := json.Marshal(result)
	require.NoError(t, err)
	for _, protected := range []string{
		"ardents.net", "1.1.1.1", "wss-certificate", "ssh-node-",
		"host-pin-", "p1_", "12D3Koo", "registry.example", "operator-primary",
	} {
		require.NotContains(t, string(encoded), protected)
	}
}

func TestPublicDirectCoordinatorPreflightsAllPublicNodesBeforeMutation(t *testing.T) {
	raw := readTopologyFixture(t, "public-direct.json")
	now := time.Date(2026, 7, 29, 15, 0, 0, 0, time.UTC)
	preflight := &publicDirectPreflightFake{
		now: now,
		mutate: func(observation *PublicDirectPreflightObservation) {
			if observation.Slot == "node-b" {
				observation.FirewallReady = false
			}
		},
	}
	hosts := &publicDirectHostsFake{}
	coordinator := PublicDirectCoordinator{
		Preflight: preflight, Hosts: hosts,
		Status:      &publicDirectStatusFake{},
		StepTimeout: time.Second, Now: func() time.Time { return now },
	}

	result, err := coordinator.Reconcile(context.Background(), raw)
	require.Error(t, err)
	require.Equal(t, PublicDirectReasonPreflightDenied, result.Reason)
	require.Equal(t, []string{"node-a", "node-b"}, preflight.slots())
	require.Empty(t, hosts.targets)
}

func TestPublicDirectCoordinatorRejectsUnboundOrStalePreflight(t *testing.T) {
	raw := readTopologyFixture(t, "public-direct.json")
	now := time.Date(2026, 7, 29, 15, 0, 0, 0, time.UTC)
	tests := []struct {
		name   string
		mutate func(*PublicDirectPreflightObservation)
	}{
		{name: "manifest", mutate: func(value *PublicDirectPreflightObservation) {
			value.ManifestDigest = strings.Repeat("b", 64)
		}},
		{name: "slot", mutate: func(value *PublicDirectPreflightObservation) {
			value.Slot = "node-c"
		}},
		{name: "address", mutate: func(value *PublicDirectPreflightObservation) {
			value.Address = "/ip4/8.8.8.8/tcp/443"
		}},
		{name: "certificate", mutate: func(value *PublicDirectPreflightObservation) {
			if value.CertificateRef != "" {
				value.CertificateRef = "different-certificate"
			}
		}},
		{name: "certificate material digest", mutate: func(value *PublicDirectPreflightObservation) {
			if value.CertificateRef != "" {
				value.CertificateMaterialDigest = "not-a-digest"
			}
		}},
		{name: "stale", mutate: func(value *PublicDirectPreflightObservation) {
			value.ObservedAt = now.Add(-publicDirectPreflightMaxAge - time.Second)
		}},
		{name: "future", mutate: func(value *PublicDirectPreflightObservation) {
			value.ObservedAt = now.Add(publicDirectPreflightFutureSkew + time.Second)
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			hosts := &publicDirectHostsFake{}
			coordinator := PublicDirectCoordinator{
				Preflight: &publicDirectPreflightFake{now: now, mutate: test.mutate},
				Hosts:     hosts, Status: &publicDirectStatusFake{},
				StepTimeout: time.Second, Now: func() time.Time { return now },
			}
			result, err := coordinator.Reconcile(context.Background(), raw)
			require.Error(t, err)
			require.Equal(t, PublicDirectReasonPreflightInvalid, result.Reason)
			require.Empty(t, hosts.targets)
		})
	}
}

func TestPublicDirectCoordinatorRequiresCertificateReadinessOnlyForWSS(t *testing.T) {
	raw := readTopologyFixture(t, "public-direct.json")
	now := time.Date(2026, 7, 29, 15, 0, 0, 0, time.UTC)
	preflight := &publicDirectPreflightFake{
		now: now,
		mutate: func(value *PublicDirectPreflightObservation) {
			value.CertificateReady = false
		},
	}
	coordinator := PublicDirectCoordinator{
		Preflight: preflight, Hosts: &publicDirectHostsFake{},
		Status:      &publicDirectStatusFake{},
		StepTimeout: time.Second, Now: func() time.Time { return now },
	}

	result, err := coordinator.Reconcile(context.Background(), raw)
	require.Error(t, err)
	require.Equal(t, PublicDirectReasonPreflightDenied, result.Reason)
	require.Equal(t, []string{"node-a"}, preflight.slots(),
		"WSS denial stops before the TCP target")
}

func TestPublicDirectCoordinatorAcceptsControlledRotationOnly(t *testing.T) {
	raw := readTopologyFixture(t, "public-direct.json")
	now := time.Date(2026, 7, 29, 15, 0, 0, 0, time.UTC)
	hosts := &publicDirectHostsFake{}
	coordinator := PublicDirectCoordinator{
		Preflight:   &publicDirectPreflightFake{now: now},
		Hosts:       hosts,
		Status:      &publicDirectStatusFake{status: readyPublicDirectStatus()},
		StepTimeout: time.Second, Now: func() time.Time { return now },
	}

	result, err := coordinator.Reconcile(context.Background(), raw)
	require.NoError(t, err)
	require.Zero(t, result.RestartedNodes)
	require.Equal(t, []PublicDirectApplyAction{
		PublicDirectApplyInstalled,
		PublicDirectApplyInstalled,
		PublicDirectApplyInstalled,
	}, publicDirectActions(hosts.observations))

	result, err = coordinator.Reconcile(context.Background(), raw)
	require.NoError(t, err)
	require.Zero(t, result.RestartedNodes)
	require.Equal(t, []PublicDirectApplyAction{
		PublicDirectApplyUnchanged,
		PublicDirectApplyUnchanged,
		PublicDirectApplyUnchanged,
	}, publicDirectActions(hosts.observations[3:]))

	for _, test := range []struct {
		name    string
		rotate  func([]byte) []byte
		actions []PublicDirectApplyAction
	}{
		{
			name: "tcp address",
			rotate: func(value []byte) []byte {
				return bytes.Replace(
					value,
					[]byte("/ip4/1.1.1.1/tcp/60000"),
					[]byte("/ip4/8.8.8.8/tcp/60000"),
					1,
				)
			},
			actions: []PublicDirectApplyAction{
				PublicDirectApplyUnchanged,
				PublicDirectApplyRestarted,
				PublicDirectApplyUnchanged,
			},
		},
		{
			name: "certificate reference",
			rotate: func(value []byte) []byte {
				return bytes.Replace(
					value,
					[]byte("wss-certificate-node-a"),
					[]byte("wss-certificate-node-a2"),
					1,
				)
			},
			actions: []PublicDirectApplyAction{
				PublicDirectApplyRestarted,
				PublicDirectApplyUnchanged,
				PublicDirectApplyUnchanged,
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			rotationHosts := &publicDirectHostsFake{}
			rotationCoordinator := coordinator
			rotationCoordinator.Hosts = rotationHosts
			_, err := rotationCoordinator.Reconcile(context.Background(), raw)
			require.NoError(t, err)

			result, err := rotationCoordinator.Reconcile(
				context.Background(),
				test.rotate(raw),
			)
			require.NoError(t, err)
			require.Equal(t, 1, result.RestartedNodes)
			require.Equal(t, test.actions, publicDirectActions(
				rotationHosts.observations[3:],
			))
		})
	}

	t.Run("certificate material behind stable reference", func(t *testing.T) {
		material := strings.Repeat("e", 64)
		preflight := &publicDirectPreflightFake{
			now:                now,
			certificateDigests: map[string]string{"node-a": material},
		}
		rotationHosts := &publicDirectHostsFake{}
		rotationCoordinator := coordinator
		rotationCoordinator.Preflight = preflight
		rotationCoordinator.Hosts = rotationHosts
		_, err := rotationCoordinator.Reconcile(context.Background(), raw)
		require.NoError(t, err)

		preflight.certificateDigests["node-a"] = strings.Repeat("f", 64)
		result, err := rotationCoordinator.Reconcile(context.Background(), raw)
		require.NoError(t, err)
		require.Equal(t, 1, result.RestartedNodes)
		require.Equal(t, []PublicDirectApplyAction{
			PublicDirectApplyRestarted,
			PublicDirectApplyUnchanged,
			PublicDirectApplyUnchanged,
		}, publicDirectActions(rotationHosts.observations[3:]))
		require.NotEqual(
			t,
			rotationHosts.targets[0].ConfigurationDigest,
			rotationHosts.targets[3].ConfigurationDigest,
		)
	})

	hosts = &publicDirectHostsFake{
		previous:      map[string]string{"node-a": strings.Repeat("a", 64)},
		invalidAction: true,
	}
	coordinator.Hosts = hosts
	result, err = coordinator.Reconcile(context.Background(), raw)
	require.Error(t, err)
	require.Equal(t, PublicDirectReasonApplyInvalid, result.Reason)
}

func TestPublicDirectCoordinatorRechecksFreshnessBeforeFirstMutation(t *testing.T) {
	raw := readTopologyFixture(t, "public-direct.json")
	now := time.Date(2026, 7, 29, 15, 0, 0, 0, time.UTC)
	calls := 0
	hosts := &publicDirectHostsFake{}
	coordinator := PublicDirectCoordinator{
		Preflight:   &publicDirectPreflightFake{now: now},
		Hosts:       hosts,
		Status:      &publicDirectStatusFake{},
		StepTimeout: time.Second,
		Now: func() time.Time {
			calls++
			if calls >= 3 {
				return now.Add(publicDirectPreflightMaxAge + time.Second)
			}
			return now
		},
	}

	result, err := coordinator.Reconcile(context.Background(), raw)
	require.Error(t, err)
	require.Equal(t, PublicDirectReasonPreflightInvalid, result.Reason)
	require.Empty(t, hosts.targets)
}

func publicDirectActions(
	observations []PublicDirectApplyObservation,
) []PublicDirectApplyAction {
	out := make([]PublicDirectApplyAction, 0, len(observations))
	for _, observation := range observations {
		out = append(out, observation.Action)
	}
	return out
}

func TestPublicDirectCoordinatorFailsClosedOnApplyOrStatusAmbiguity(t *testing.T) {
	raw := readTopologyFixture(t, "public-direct.json")
	now := time.Date(2026, 7, 29, 15, 0, 0, 0, time.UTC)
	t.Run("apply response", func(t *testing.T) {
		hosts := &publicDirectHostsFake{mutate: func(value *PublicDirectApplyObservation) {
			value.ManifestDigest = strings.Repeat("c", 64)
		}}
		coordinator := PublicDirectCoordinator{
			Preflight: &publicDirectPreflightFake{now: now}, Hosts: hosts,
			Status:      &publicDirectStatusFake{},
			StepTimeout: time.Second, Now: func() time.Time { return now },
		}
		result, err := coordinator.Reconcile(context.Background(), raw)
		require.Error(t, err)
		require.Equal(t, PublicDirectReasonApplyInvalid, result.Reason)
	})
	t.Run("status digest", func(t *testing.T) {
		status := &publicDirectStatusFake{
			status: readyPublicDirectStatus(),
			digest: strings.Repeat("d", 64),
		}
		coordinator := PublicDirectCoordinator{
			Preflight: &publicDirectPreflightFake{now: now},
			Hosts:     &publicDirectHostsFake{}, Status: status,
			StepTimeout: time.Second, Now: func() time.Time { return now },
		}
		result, err := coordinator.Reconcile(context.Background(), raw)
		require.Error(t, err)
		require.Equal(t, PublicDirectReasonStatusDegraded, result.Reason)
	})
	t.Run("runtime configuration generation", func(t *testing.T) {
		status := &publicDirectStatusFake{
			status: readyPublicDirectStatus(),
			configurations: []PublicDirectConfigurationObservation{
				{
					Slot:                "node-a",
					ConfigurationDigest: strings.Repeat("a", 64),
				},
				{
					Slot:                "node-b",
					ConfigurationDigest: strings.Repeat("b", 64),
				},
				{
					Slot:                "node-c",
					ConfigurationDigest: strings.Repeat("c", 64),
				},
			},
		}
		coordinator := PublicDirectCoordinator{
			Preflight: &publicDirectPreflightFake{now: now},
			Hosts:     &publicDirectHostsFake{}, Status: status,
			StepTimeout: time.Second, Now: func() time.Time { return now },
		}
		result, err := coordinator.Reconcile(context.Background(), raw)
		require.Error(t, err)
		require.Equal(t, PublicDirectReasonStatusDegraded, result.Reason)
	})
	t.Run("private or unknown projection", func(t *testing.T) {
		statusValue := readyPublicDirectStatus()
		statusValue.Outcome = TopologyOutcomeDegraded
		statusValue.Nodes[0].Ready = false
		statusValue.Nodes[0].Reachability = NodeTruthDegraded
		statusValue.Nodes[0].Reason = StatusReasonPublicReachabilityUnverified
		coordinator := PublicDirectCoordinator{
			Preflight:   &publicDirectPreflightFake{now: now},
			Hosts:       &publicDirectHostsFake{},
			Status:      &publicDirectStatusFake{status: statusValue},
			StepTimeout: time.Second, Now: func() time.Time { return now },
		}
		result, err := coordinator.Reconcile(context.Background(), raw)
		require.Error(t, err)
		require.Equal(t, PublicDirectReasonStatusDegraded, result.Reason)
	})
}

func TestPublicDirectCoordinatorRejectsBeforeMutationAndBoundsSteps(t *testing.T) {
	private := readTopologyFixture(t, "private-lan.json")
	hosts := &publicDirectHostsFake{}
	coordinator := PublicDirectCoordinator{
		Preflight: &publicDirectPreflightFake{now: time.Now().UTC()},
		Hosts:     hosts, Status: &publicDirectStatusFake{},
		StepTimeout: time.Second,
	}
	_, err := coordinator.Reconcile(context.Background(), private)
	require.ErrorContains(t, err, "public_direct")
	require.Empty(t, hosts.targets)

	coordinator.Preflight = nil
	_, err = coordinator.Reconcile(context.Background(), readTopologyFixture(t, "public-direct.json"))
	require.ErrorContains(t, err, "required")

	now := time.Date(2026, 7, 29, 15, 0, 0, 0, time.UTC)
	coordinator = PublicDirectCoordinator{
		Preflight: &publicDirectPreflightFake{now: now, waitForContext: true},
		Hosts:     &publicDirectHostsFake{}, Status: &publicDirectStatusFake{},
		StepTimeout: time.Millisecond, Now: func() time.Time { return now },
	}
	result, err := coordinator.Reconcile(context.Background(), readTopologyFixture(t, "public-direct.json"))
	require.Error(t, err)
	require.Equal(t, PublicDirectReasonPreflightUnavailable, result.Reason)
}

func readyPublicDirectStatus() TopologyStatus {
	return TopologyStatus{
		APIVersion: TopologyStatusVersion,
		Outcome:    TopologyOutcomeReady,
		Nodes: []NodeStatus{
			{
				Slot: "node-a", Role: "authority",
				Observation: NodeObservationComplete, Ready: true,
				Readiness: NodeTruthReady, Joined: true,
				Reachability: NodeTruthReady, Store: NodeTruthReady,
				Image: NodeImageMatch,
			},
			{
				Slot: "node-b", Role: "member",
				Observation: NodeObservationComplete, Ready: true,
				Readiness: NodeTruthReady, Joined: true,
				Reachability: NodeTruthReady, Store: NodeTruthReady,
				Image: NodeImageMatch,
			},
			{
				Slot: "node-c", Role: "member",
				Observation: NodeObservationComplete, Ready: true,
				Readiness: NodeTruthReady, Joined: true,
				Reachability: NodeTruthReady, Store: NodeTruthNotRequired,
				Image: NodeImageMatch,
			},
		},
	}
}

type publicDirectPreflightFake struct {
	now                time.Time
	targets            []PublicDirectPreflightTarget
	mutate             func(*PublicDirectPreflightObservation)
	waitForContext     bool
	certificateDigests map[string]string
}

func (fake *publicDirectPreflightFake) Observe(
	ctx context.Context,
	target PublicDirectPreflightTarget,
) (PublicDirectPreflightObservation, error) {
	fake.targets = append(fake.targets, target)
	if fake.waitForContext {
		<-ctx.Done()
		return PublicDirectPreflightObservation{}, ctx.Err()
	}
	value := PublicDirectPreflightObservation{
		ManifestDigest: target.ManifestDigest,
		Slot:           target.Slot, Address: target.Address,
		CertificateRef:      target.CertificateRef,
		CertificateIdentity: target.CertificateIdentity,
		RouteReady:          true, FirewallReady: true,
		CertificateReady: target.CertificateRef != "",
		ObservedAt:       fake.now,
	}
	if target.CertificateRef != "" {
		value.CertificateMaterialDigest = strings.Repeat("e", 64)
		if configured := fake.certificateDigests[target.Slot]; configured != "" {
			value.CertificateMaterialDigest = configured
		}
	}
	if fake.mutate != nil {
		fake.mutate(&value)
	}
	return value, nil
}

func (fake *publicDirectPreflightFake) slots() []string {
	out := make([]string, 0, len(fake.targets))
	for _, target := range fake.targets {
		out = append(out, target.Slot)
	}
	return out
}

type publicDirectHostsFake struct {
	targets       []PublicDirectHostTarget
	observations  []PublicDirectApplyObservation
	previous      map[string]string
	mutate        func(*PublicDirectApplyObservation)
	invalidAction bool
	err           error
}

func (fake *publicDirectHostsFake) Apply(
	_ context.Context,
	target PublicDirectHostTarget,
) (PublicDirectApplyObservation, error) {
	fake.targets = append(fake.targets, target)
	if fake.err != nil {
		return PublicDirectApplyObservation{}, fake.err
	}
	previous := fake.previous[target.Slot]
	action := PublicDirectApplyInstalled
	switch {
	case previous == target.ConfigurationDigest:
		action = PublicDirectApplyUnchanged
	case previous != "":
		action = PublicDirectApplyRestarted
	}
	if fake.invalidAction {
		action = PublicDirectApplyUnchanged
	}
	value := PublicDirectApplyObservation{
		ManifestDigest:              target.ManifestDigest,
		Slot:                        target.Slot,
		ConfigurationDigest:         target.ConfigurationDigest,
		PreviousConfigurationDigest: previous,
		Action:                      action,
		IdentityPreserved:           true,
	}
	if fake.mutate != nil {
		fake.mutate(&value)
	}
	fake.observations = append(fake.observations, value)
	if fake.previous == nil {
		fake.previous = make(map[string]string)
	}
	fake.previous[target.Slot] = target.ConfigurationDigest
	return value, nil
}

func (fake *publicDirectHostsFake) slots() []string {
	out := make([]string, 0, len(fake.targets))
	for _, target := range fake.targets {
		out = append(out, target.Slot)
	}
	return out
}

type publicDirectStatusFake struct {
	status         TopologyStatus
	digest         string
	configurations []PublicDirectConfigurationObservation
	err            error
	calls          int
}

func (fake *publicDirectStatusFake) Observe(
	_ context.Context,
	target PublicDirectStatusTarget,
) (PublicDirectStatusObservation, error) {
	fake.calls++
	if fake.err != nil {
		return PublicDirectStatusObservation{}, fake.err
	}
	digest := fake.digest
	if digest == "" {
		digest = target.ManifestDigest
	}
	configurations := fake.configurations
	if configurations == nil {
		configurations = target.Configurations
	}
	return PublicDirectStatusObservation{
		ManifestDigest: digest,
		Status:         fake.status,
		Configurations: append(
			[]PublicDirectConfigurationObservation(nil),
			configurations...,
		),
	}, nil
}
