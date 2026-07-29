package deployment

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRolloutCoordinatorAppliesCompatibleReleaseSerially(t *testing.T) {
	raw := readTopologyFixture(t, "public-direct.json")
	request := validRolloutRequest(t, raw, AuthorityChangeCompatible, false)
	journal := &rolloutJournalFake{}
	hosts := newRolloutHostsFake()
	coordinator := RolloutCoordinator{
		Journal: journal, Preflight: rolloutPreflightFake{},
		Hosts: hosts, Authority: &rolloutAuthorityFake{},
		Committer: &rolloutCommitterFake{},
		Clock:     func() time.Time { return request.StartedAt.Add(time.Minute) },
	}

	status, err := coordinator.Rollout(context.Background(), request)
	require.NoError(t, err)
	require.Equal(t, RolloutOutcomeReady, status.Outcome)
	require.Equal(t, 3, status.NodesApplied)
	require.Equal(t, []string{"node-b", "node-c", "node-a"}, hosts.forwardSlots())
	require.Equal(t, 1, hosts.maxInFlight)
	require.True(t, journal.cleared)
	require.True(t, journal.persistedBeforeEveryEffect)
}

func TestRolloutCoordinatorCompensatesInReverseIncludingAmbiguousNode(t *testing.T) {
	raw := readTopologyFixture(t, "private-lan.json")
	request := validRolloutRequest(t, raw, AuthorityChangeCompatible, true)
	journal := &rolloutJournalFake{}
	hosts := newRolloutHostsFake()
	hosts.failReadiness["node-c"] = errors.New("readiness failed")
	coordinator := RolloutCoordinator{
		Journal: journal, Preflight: rolloutPreflightFake{},
		Hosts: hosts, Authority: &rolloutAuthorityFake{},
		Committer: &rolloutCommitterFake{},
		Clock:     func() time.Time { return request.StartedAt.Add(time.Minute) },
	}

	status, err := coordinator.Rollout(context.Background(), request)
	require.NoError(t, err)
	require.Equal(t, RolloutOutcomeCompensated, status.Outcome)
	require.Equal(t, []string{"node-b", "node-c"}, hosts.forwardSlots())
	require.Equal(t, []string{"node-c", "node-b"}, hosts.compensationSlots())
	require.Equal(t, []bool{true, true}, hosts.compensationRestoreFlags())
	require.True(t, journal.cleared)
}

func TestRolloutCoordinatorResumesPendingCompensationWithoutNewRollout(t *testing.T) {
	raw := readTopologyFixture(t, "private-lan.json")
	request := validRolloutRequest(t, raw, AuthorityChangeCompatible, false)
	journal := &rolloutJournalFake{}
	hosts := newRolloutHostsFake()
	hosts.failReadiness["node-c"] = errors.New("forward readiness failed")
	hosts.failCompensation["node-b"] = errors.New("fallback unavailable")
	coordinator := RolloutCoordinator{
		Journal: journal, Preflight: rolloutPreflightFake{},
		Hosts: hosts, Authority: &rolloutAuthorityFake{},
		Committer: &rolloutCommitterFake{},
		Clock:     func() time.Time { return request.StartedAt.Add(time.Minute) },
	}

	first, err := coordinator.Rollout(context.Background(), request)
	require.NoError(t, err)
	require.Equal(t, RolloutOutcomeRecoveryRequired, first.Outcome)
	require.False(t, journal.cleared)
	forwardCount := len(hosts.forwardSlots())

	delete(hosts.failCompensation, "node-b")
	second, err := coordinator.Rollout(context.Background(), request)
	require.NoError(t, err)
	require.Equal(t, RolloutOutcomeRecovered, second.Outcome)
	require.Equal(t, forwardCount, len(hosts.forwardSlots()),
		"recovery invocation must not start the requested rollout")
	require.True(t, journal.cleared)
}

func TestRolloutCoordinatorRecoversInterruptedNoEffectJournal(t *testing.T) {
	raw := readTopologyFixture(t, "private-lan.json")
	request := validRolloutRequest(t, raw, AuthorityChangeCompatible, false)
	manifest, _, order, compatibilityDigest, err := validateRolloutRequest(request)
	require.NoError(t, err)

	for _, phase := range []RolloutPhase{
		RolloutPhasePreflighted,
		RolloutPhaseApplying,
	} {
		t.Run(string(phase), func(t *testing.T) {
			transaction := newRolloutTransaction(
				request,
				manifest,
				order,
				compatibilityDigest,
			)
			transaction.Revision = 1
			transaction.Phase = phase
			if phase == RolloutPhaseApplying {
				transaction.Revision++
			}
			journal := &rolloutJournalFake{
				transaction: transaction,
				found:       true,
			}
			hosts := newRolloutHostsFake()
			coordinator := RolloutCoordinator{
				Journal: journal, Preflight: &rolloutPreflightFake{},
				Hosts: hosts, Authority: &rolloutAuthorityFake{},
				Committer: &rolloutCommitterFake{},
				Clock:     func() time.Time { return request.StartedAt.Add(time.Minute) },
			}

			status, err := coordinator.Rollout(context.Background(), request)
			require.NoError(t, err)
			require.Equal(t, RolloutOutcomeRecovered, status.Outcome)
			require.Equal(t, RolloutPhaseCompensated, status.Phase)
			require.Equal(t, RolloutReasonInterrupted, status.Reason)
			require.Empty(t, hosts.forwardSlots())
			require.Empty(t, hosts.compensationSlots())
			require.True(t, journal.cleared)
		})
	}
}

func TestRolloutCoordinatorBoundsForwardAndRecoveryEffects(t *testing.T) {
	raw := readTopologyFixture(t, "private-lan.json")
	request := validRolloutRequest(t, raw, AuthorityChangeCompatible, false)

	t.Run("forward effect deadline compensates ambiguous node", func(t *testing.T) {
		hosts := newRolloutHostsFake()
		hosts.blockForwardRecreate = true
		coordinator := RolloutCoordinator{
			Journal: &rolloutJournalFake{}, Preflight: &rolloutPreflightFake{},
			Hosts: hosts, Authority: &rolloutAuthorityFake{},
			Committer:     &rolloutCommitterFake{},
			Clock:         func() time.Time { return request.StartedAt.Add(time.Minute) },
			EffectTimeout: time.Millisecond,
		}

		started := time.Now()
		status, err := coordinator.Rollout(context.Background(), request)
		require.NoError(t, err)
		require.Less(t, time.Since(started), time.Second)
		require.Equal(t, RolloutOutcomeCompensated, status.Outcome)
		require.Equal(t, RolloutReasonDeadlineExceeded, status.Reason)
		require.Equal(t, []string{"node-b"}, hosts.compensationSlots())
	})

	t.Run("recovery effect deadline retains journal", func(t *testing.T) {
		hosts := newRolloutHostsFake()
		hosts.failReadiness["node-c"] = errors.New("forward readiness failed")
		hosts.blockCompensationRecreate = true
		journal := &rolloutJournalFake{}
		coordinator := RolloutCoordinator{
			Journal: journal, Preflight: &rolloutPreflightFake{},
			Hosts: hosts, Authority: &rolloutAuthorityFake{},
			Committer:     &rolloutCommitterFake{},
			Clock:         func() time.Time { return request.StartedAt.Add(time.Minute) },
			EffectTimeout: time.Millisecond,
		}

		started := time.Now()
		status, err := coordinator.Rollout(context.Background(), request)
		require.NoError(t, err)
		require.Less(t, time.Since(started), time.Second)
		require.Equal(t, RolloutOutcomeRecoveryRequired, status.Outcome)
		require.Equal(t, RolloutReasonCompensationFailed, status.Reason)
		require.True(t, journal.found)
	})
}

func TestRolloutCoordinatorUsesAuthorityFirstMigrationAndActivation(t *testing.T) {
	raw := readTopologyFixture(t, "public-direct.json")
	request := validRolloutRequest(t, raw, AuthorityChangeMigration, true)
	journal := &rolloutJournalFake{}
	hosts := newRolloutHostsFake()
	authority := &rolloutAuthorityFake{}
	coordinator := RolloutCoordinator{
		Journal: journal, Preflight: rolloutPreflightFake{},
		Hosts: hosts, Authority: authority,
		Committer: &rolloutCommitterFake{},
		Clock:     func() time.Time { return request.StartedAt.Add(time.Minute) },
	}

	status, err := coordinator.Rollout(context.Background(), request)
	require.NoError(t, err)
	require.Equal(t, RolloutOutcomeReady, status.Outcome)
	require.Equal(t, []string{"node-a", "node-b", "node-c"}, hosts.forwardSlots())
	require.Equal(t, 1, authority.calls)
	require.True(t, authority.persistedBeforeEffect)
}

func TestRolloutCoordinatorResolvesCommittedManifestAfterLostResponse(t *testing.T) {
	raw := readTopologyFixture(t, "public-direct.json")
	request := validRolloutRequest(t, raw, AuthorityChangeCompatible, false)
	committer := &rolloutCommitterFake{loseCommitResponse: true}
	coordinator := RolloutCoordinator{
		Journal: &rolloutJournalFake{}, Preflight: rolloutPreflightFake{},
		Hosts: newRolloutHostsFake(), Authority: &rolloutAuthorityFake{},
		Committer: committer,
		Clock:     func() time.Time { return request.StartedAt.Add(time.Minute) },
	}

	status, err := coordinator.Rollout(context.Background(), request)
	require.NoError(t, err)
	require.Equal(t, RolloutOutcomeReady, status.Outcome)
	require.True(t, committer.committed)
}

func TestRolloutCoordinatorReconcilesAmbiguousMonotonicActivation(t *testing.T) {
	raw := readTopologyFixture(t, "public-direct.json")
	request := validRolloutRequest(t, raw, AuthorityChangeMigration, true)
	t.Run("lost response but committed", func(t *testing.T) {
		authority := &rolloutAuthorityFake{loseResponse: true}
		coordinator := RolloutCoordinator{
			Journal: &rolloutJournalFake{}, Preflight: &rolloutPreflightFake{},
			Hosts: newRolloutHostsFake(), Authority: authority,
			Committer: &rolloutCommitterFake{},
			Clock:     func() time.Time { return request.StartedAt.Add(time.Minute) },
		}
		status, err := coordinator.Rollout(context.Background(), request)
		require.NoError(t, err)
		require.Equal(t, RolloutOutcomeReady, status.Outcome)
		require.True(t, authority.activated)
	})
	t.Run("unknown status remains recovery required", func(t *testing.T) {
		journal := &rolloutJournalFake{}
		hosts := newRolloutHostsFake()
		authority := &rolloutAuthorityFake{
			loseResponse: true, statusErr: errors.New("repository unavailable"),
		}
		coordinator := RolloutCoordinator{
			Journal: journal, Preflight: &rolloutPreflightFake{},
			Hosts: hosts, Authority: authority,
			Committer: &rolloutCommitterFake{},
			Clock:     func() time.Time { return request.StartedAt.Add(time.Minute) },
		}
		first, err := coordinator.Rollout(context.Background(), request)
		require.NoError(t, err)
		require.Equal(t, RolloutOutcomeRecoveryRequired, first.Outcome)
		require.Equal(t, RolloutPhaseActivationPending, journal.transaction.ResumeFrom)
		require.Empty(t, hosts.compensationSlots())
		forwardCount := len(hosts.forwardSlots())

		authority.statusErr = nil
		second, err := coordinator.Rollout(context.Background(), request)
		require.NoError(t, err)
		require.Equal(t, RolloutOutcomeReady, second.Outcome)
		require.Equal(t, forwardCount, len(hosts.forwardSlots()))
		require.Empty(t, hosts.compensationSlots())
	})
}

func TestRolloutCoordinatorNeverCompensatesPastMigrationActivation(t *testing.T) {
	raw := readTopologyFixture(t, "public-direct.json")
	request := validRolloutRequest(t, raw, AuthorityChangeMigration, true)
	journal := &rolloutJournalFake{}
	hosts := newRolloutHostsFake()
	committer := &rolloutCommitterFake{failCommit: true}
	coordinator := RolloutCoordinator{
		Journal: journal, Preflight: &rolloutPreflightFake{},
		Hosts: hosts, Authority: &rolloutAuthorityFake{},
		Committer: committer,
		Clock:     func() time.Time { return request.StartedAt.Add(time.Minute) },
	}

	first, err := coordinator.Rollout(context.Background(), request)
	require.NoError(t, err)
	require.Equal(t, RolloutOutcomeRecoveryRequired, first.Outcome)
	require.Equal(t, RolloutPhaseReadyToCommit, journal.transaction.ResumeFrom)
	require.Empty(t, hosts.compensationSlots())
	forwardCount := len(hosts.forwardSlots())

	committer.failCommit = false
	second, err := coordinator.Rollout(context.Background(), request)
	require.NoError(t, err)
	require.Equal(t, RolloutOutcomeReady, second.Outcome)
	require.Equal(t, forwardCount, len(hosts.forwardSlots()))
	require.Empty(t, hosts.compensationSlots())
}

func TestRolloutTransactionRejectsImpossibleTerminalTruth(t *testing.T) {
	transaction := RolloutTransaction{
		Version: RolloutTransactionVersion, Revision: 1,
		ManifestDigest: strings.Repeat("a", 64), RequestID: "rollout-invalid",
		CompatibilityDigest: "sha256:" + strings.Repeat("b", 64),
		ChangeKind:          AuthorityChangeCompatible,
		StartedAt:           time.Date(2026, 7, 29, 16, 0, 0, 0, time.UTC),
		Deadline:            time.Date(2026, 7, 29, 16, 20, 0, 0, time.UTC),
		Order:               []string{"node-b", "node-c", "node-a"},
		Phase:               RolloutPhaseReadyToCommit,
		Nodes: []RolloutNodeTransaction{{
			Slot: "node-b", TargetImage: rolloutTestTargetImage(),
			FallbackImage: rolloutTestFallbackImage(),
			Phase:         RolloutNodeApplied,
		}},
	}
	require.ErrorIs(t, ValidateRolloutTransaction(transaction), ErrRolloutJournalInvalid)

	transaction.Phase = RolloutPhaseApplying
	transaction.AuthorityGeneration = 2
	transaction.CheckpointDigest = "sha256:" + strings.Repeat("c", 64)
	transaction.RepositoryPersisted = true
	transaction.ActiveReceiptCount = 3
	require.ErrorIs(t, ValidateRolloutTransaction(transaction), ErrRolloutJournalInvalid)
}

func rolloutTestTargetImage() string {
	return "registry.example/ardents/node@sha256:" + strings.Repeat("a", 64)
}

func rolloutTestFallbackImage() string {
	return "registry.example/ardents/node@sha256:" + strings.Repeat("d", 64)
}

func TestRolloutCoordinatorRejectsInvalidOrStalePreflightBeforeJournal(t *testing.T) {
	raw := readTopologyFixture(t, "public-direct.json")
	request := validRolloutRequest(t, raw, AuthorityChangeCompatible, false)
	for _, test := range []struct {
		name   string
		mutate func(*RolloutPreflightObservation)
	}{
		{name: "manifest binding", mutate: func(value *RolloutPreflightObservation) {
			value.ManifestDigest = strings.Repeat("a", 64)
		}},
		{name: "stale clock", mutate: func(value *RolloutPreflightObservation) {
			value.ClockObservedAt = request.StartedAt.Add(-time.Second)
		}},
		{name: "materials", mutate: func(value *RolloutPreflightObservation) {
			value.MaterialsVerified = false
		}},
		{name: "authority backup", mutate: func(value *RolloutPreflightObservation) {
			value.AuthorityBackupVerified = false
		}},
		{name: "provider", mutate: func(value *RolloutPreflightObservation) {
			value.Nodes[0].StoreReady = false
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			journal := &rolloutJournalFake{}
			preflight := &rolloutPreflightFake{mutate: test.mutate}
			coordinator := RolloutCoordinator{
				Journal: journal, Preflight: preflight,
				Hosts: newRolloutHostsFake(), Authority: &rolloutAuthorityFake{},
				Committer: &rolloutCommitterFake{},
				Clock:     func() time.Time { return request.StartedAt.Add(time.Minute) },
			}
			status, err := coordinator.Rollout(context.Background(), request)
			require.NoError(t, err)
			require.Equal(t, RolloutOutcomeRecoveryRequired, status.Outcome)
			require.Equal(t, RolloutReasonPreflightInvalid, status.Reason)
			require.False(t, journal.found)
		})
	}
}

func TestRolloutCoordinatorFaultInjectionCompensatesEveryForwardBoundary(t *testing.T) {
	raw := readTopologyFixture(t, "public-direct.json")
	for _, boundary := range []string{
		"recreate", "start", "readiness", "activation", "commit",
	} {
		t.Run(boundary, func(t *testing.T) {
			kind := AuthorityChangeCompatible
			if boundary == "activation" {
				kind = AuthorityChangeMigration
			}
			request := validRolloutRequest(t, raw, kind, false)
			hosts := newRolloutHostsFake()
			authority := &rolloutAuthorityFake{}
			committer := &rolloutCommitterFake{}
			switch boundary {
			case "recreate":
				hosts.failRecreate["node-b"] = errors.New("recreate failed")
			case "start":
				hosts.failStart["node-b"] = errors.New("start failed")
			case "readiness":
				hosts.failReadiness["node-b"] = errors.New("readiness failed")
			case "activation":
				authority.err = errors.New("activation failed")
			case "commit":
				committer.failCommit = true
			}
			coordinator := RolloutCoordinator{
				Journal: &rolloutJournalFake{}, Preflight: &rolloutPreflightFake{},
				Hosts: hosts, Authority: authority, Committer: committer,
				Clock: func() time.Time { return request.StartedAt.Add(time.Minute) },
			}
			status, err := coordinator.Rollout(context.Background(), request)
			require.NoError(t, err)
			require.Equal(t, RolloutOutcomeCompensated, status.Outcome)
			require.NotEmpty(t, hosts.compensationSlots())
			if boundary == "activation" {
				require.Equal(
					t,
					[]string{"node-c", "node-b", "node-a"},
					hosts.compensationSlots(),
				)
			} else if boundary == "commit" {
				require.Equal(
					t,
					[]string{"node-a", "node-c", "node-b"},
					hosts.compensationSlots(),
				)
			}
		})
	}
}

func TestRolloutCoordinatorRecoversAfterPostEffectJournalFailure(t *testing.T) {
	raw := readTopologyFixture(t, "private-lan.json")
	request := validRolloutRequest(t, raw, AuthorityChangeCompatible, false)
	journal := &rolloutJournalFake{failSaveAt: 4}
	hosts := newRolloutHostsFake()
	coordinator := RolloutCoordinator{
		Journal: journal, Preflight: &rolloutPreflightFake{},
		Hosts: hosts, Authority: &rolloutAuthorityFake{},
		Committer: &rolloutCommitterFake{},
		Clock:     func() time.Time { return request.StartedAt.Add(time.Minute) },
	}

	_, err := coordinator.Rollout(context.Background(), request)
	require.ErrorIs(t, err, ErrRolloutJournalConflict)
	require.Equal(t, []string{"node-b"}, hosts.forwardSlots())
	require.True(t, journal.found)
	require.Equal(t, RolloutNodeMutationPending, journal.transaction.Nodes[0].Phase)

	journal.failSaveAt = 0
	status, err := coordinator.Rollout(context.Background(), request)
	require.NoError(t, err)
	require.Equal(t, RolloutOutcomeRecovered, status.Outcome)
	require.Equal(t, []string{"node-b"}, hosts.compensationSlots())
	require.Equal(t, 1, len(hosts.forwardSlots()))
}

func TestRolloutCoordinatorRetainsRecoveryForEveryCompensationBoundary(t *testing.T) {
	raw := readTopologyFixture(t, "private-lan.json")
	for _, boundary := range []string{"recreate", "start", "readiness"} {
		t.Run(boundary, func(t *testing.T) {
			request := validRolloutRequest(t, raw, AuthorityChangeCompatible, false)
			hosts := newRolloutHostsFake()
			hosts.failReadiness["node-c"] = errors.New("forward failure")
			switch boundary {
			case "recreate":
				hosts.failCompensation["node-c"] = errors.New("fallback recreate")
			case "start":
				hosts.failCompensationStart["node-c"] = errors.New("fallback start")
			case "readiness":
				hosts.failCompensationReadiness["node-c"] =
					errors.New("fallback readiness")
			}
			journal := &rolloutJournalFake{}
			coordinator := RolloutCoordinator{
				Journal: journal, Preflight: &rolloutPreflightFake{},
				Hosts: hosts, Authority: &rolloutAuthorityFake{},
				Committer: &rolloutCommitterFake{},
				Clock:     func() time.Time { return request.StartedAt.Add(time.Minute) },
			}
			status, err := coordinator.Rollout(context.Background(), request)
			require.NoError(t, err)
			require.Equal(t, RolloutOutcomeRecoveryRequired, status.Outcome)
			require.True(t, status.RecoveryPending)
			require.True(t, journal.found)
			require.Equal(t, RolloutPhaseRecoveryRequired, journal.transaction.Phase)
		})
	}
}

func TestRolloutStatusIsRedactedAndPendingJournalRejectsRebinding(t *testing.T) {
	raw := readTopologyFixture(t, "private-lan.json")
	request := validRolloutRequest(t, raw, AuthorityChangeCompatible, false)
	journal := &rolloutJournalFake{}
	hosts := newRolloutHostsFake()
	hosts.failReadiness["node-c"] = errors.New("forward failure")
	hosts.failCompensation["node-b"] = errors.New("fallback failure")
	coordinator := RolloutCoordinator{
		Journal: journal, Preflight: &rolloutPreflightFake{},
		Hosts: hosts, Authority: &rolloutAuthorityFake{},
		Committer: &rolloutCommitterFake{},
		Clock:     func() time.Time { return request.StartedAt.Add(time.Minute) },
	}
	status, err := coordinator.Rollout(context.Background(), request)
	require.NoError(t, err)
	encoded, err := json.Marshal(status)
	require.NoError(t, err)
	for _, protected := range []string{
		"registry.example", "ssh-node", "host-pin", "p1_", "12D3Koo",
		"sha256:", "1.0.0", "1.1.0",
	} {
		require.NotContains(t, string(encoded), protected)
		require.NotContains(t, status.String(), protected)
	}

	rebound := request
	rebound.RequestID = "different-rollout"
	_, err = coordinator.Rollout(context.Background(), rebound)
	require.ErrorIs(t, err, ErrRolloutJournalBinding)
	require.Equal(t, 2, len(hosts.forwardSlots()))
}

func validRolloutRequest(
	t *testing.T,
	raw []byte,
	kind AuthorityChangeKind,
	restore bool,
) RolloutRequest {
	t.Helper()
	manifest, err := decodeTopology(raw)
	require.NoError(t, err)
	fallbacks := make(map[string]string, len(manifest.Nodes))
	for _, node := range manifest.Nodes {
		fallbacks[node.Slot] =
			"registry.example/ardents/node@sha256:" +
				"dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
	}
	started := time.Date(2026, 7, 29, 16, 0, 0, 0, time.UTC)
	return RolloutRequest{
		Manifest: raw, RequestID: "rollout-test",
		Compatibility: RolloutCompatibility{
			Kind: kind, FromVersion: "1.0.0", ToVersion: "1.1.0",
			MixedGenerationAllowed:      kind == AuthorityChangeCompatible,
			AuthorityActivationRequired: kind == AuthorityChangeMigration,
			CompleteDataRestoreRequired: restore,
			MaterialsPolicyDigest: "sha256:" +
				"eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee",
		},
		FallbackImages: fallbacks,
		StartedAt:      started, Deadline: started.Add(20 * time.Minute),
	}
}

type rolloutJournalFake struct {
	transaction                RolloutTransaction
	found                      bool
	cleared                    bool
	persistedBeforeEveryEffect bool
	saveCalls                  int
	failSaveAt                 int
}

func (fake *rolloutJournalFake) Load(context.Context) (RolloutTransaction, bool, error) {
	return cloneRolloutTransaction(fake.transaction), fake.found, nil
}

func (fake *rolloutJournalFake) Save(
	_ context.Context,
	expected uint64,
	transaction RolloutTransaction,
) error {
	fake.saveCalls++
	if fake.failSaveAt > 0 && fake.saveCalls == fake.failSaveAt {
		return ErrRolloutJournalConflict
	}
	if transaction.Revision != expected+1 {
		return ErrRolloutJournalConflict
	}
	fake.transaction = cloneRolloutTransaction(transaction)
	fake.found = true
	fake.cleared = false
	if transaction.Phase == RolloutPhaseActivationPending ||
		transaction.Phase == RolloutPhaseReadyToCommit ||
		len(transaction.Nodes) > 0 {
		fake.persistedBeforeEveryEffect = true
	}
	return nil
}

func (fake *rolloutJournalFake) Clear(
	_ context.Context,
	expected RolloutTransaction,
) error {
	if !fake.found || fake.transaction.Revision != expected.Revision {
		return ErrRolloutJournalConflict
	}
	fake.transaction = RolloutTransaction{}
	fake.found = false
	fake.cleared = true
	return nil
}

type rolloutPreflightFake struct {
	mutate func(*RolloutPreflightObservation)
	err    error
}

func (fake rolloutPreflightFake) Verify(
	_ context.Context,
	target RolloutPreflightTarget,
) (RolloutPreflightObservation, error) {
	if fake.err != nil {
		return RolloutPreflightObservation{}, fake.err
	}
	nodes := make([]RolloutNodePreflight, 0, len(target.Nodes))
	for _, node := range target.Nodes {
		nodes = append(nodes, RolloutNodePreflight{
			Slot: node.Slot, NodePrincipal: node.ExpectedNodePrincipal,
			WakuPeerID: node.ExpectedWakuPeerID, Image: node.FallbackImage,
			CompositeReady: true, Joined: true,
			ReachabilityReady: true, StoreReady: !node.PersistentStore || true,
			BackupVerified: true,
		})
	}
	observation := RolloutPreflightObservation{
		ManifestDigest: target.ManifestDigest, RequestID: target.RequestID,
		CompatibilityDigest: target.CompatibilityDigest,
		ClockObservedAt:     target.StartedAt.Add(time.Minute),
		ClockSkewSeconds:    0, AuthorityBackupVerified: true,
		RepositoryHeadDigest: "sha256:" +
			"ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
		RepositoryHeadVerified: true,
		MaterialsPolicyDigest:  target.MaterialsPolicyDigest,
		MaterialsVerified:      true, Nodes: nodes,
	}
	if fake.mutate != nil {
		fake.mutate(&observation)
	}
	return observation, nil
}

type rolloutHostEvent struct {
	kind        string
	slot        string
	image       string
	restoreData bool
}

type rolloutHostsFake struct {
	events                    []rolloutHostEvent
	currentImages             map[string]string
	failReadiness             map[string]error
	failRecreate              map[string]error
	failStart                 map[string]error
	failCompensation          map[string]error
	failCompensationStart     map[string]error
	failCompensationReadiness map[string]error
	blockForwardRecreate      bool
	blockCompensationRecreate bool
	inFlight                  int
	maxInFlight               int
}

func newRolloutHostsFake() *rolloutHostsFake {
	return &rolloutHostsFake{
		currentImages:             make(map[string]string),
		failReadiness:             make(map[string]error),
		failRecreate:              make(map[string]error),
		failStart:                 make(map[string]error),
		failCompensation:          make(map[string]error),
		failCompensationStart:     make(map[string]error),
		failCompensationReadiness: make(map[string]error),
	}
}

func (fake *rolloutHostsFake) Recreate(
	ctx context.Context,
	target RolloutHostTarget,
	change RolloutHostChange,
) (RolloutHostObservation, error) {
	fake.inFlight++
	if fake.inFlight > fake.maxInFlight {
		fake.maxInFlight = fake.inFlight
	}
	defer func() { fake.inFlight-- }()
	kind := "forward"
	if change.Compensating {
		kind = "compensate"
		if fake.blockCompensationRecreate {
			<-ctx.Done()
			return RolloutHostObservation{}, ctx.Err()
		}
		if err := fake.failCompensation[target.Slot]; err != nil {
			return RolloutHostObservation{}, err
		}
	} else {
		if fake.blockForwardRecreate {
			<-ctx.Done()
			return RolloutHostObservation{}, ctx.Err()
		}
		if err := fake.failRecreate[target.Slot]; err != nil {
			return RolloutHostObservation{}, err
		}
	}
	fake.events = append(fake.events, rolloutHostEvent{
		kind: kind, slot: target.Slot, image: change.Image,
		restoreData: change.RestoreData,
	})
	fake.currentImages[target.Slot] = change.Image
	return rolloutHostObservation(target, change.Image, change.RestoreData), nil
}

func (fake *rolloutHostsFake) Start(
	_ context.Context,
	target RolloutHostTarget,
	change RolloutHostChange,
) (RolloutHostObservation, error) {
	if change.Compensating {
		if err := fake.failCompensationStart[target.Slot]; err != nil {
			return RolloutHostObservation{}, err
		}
	} else if err := fake.failStart[target.Slot]; err != nil {
		return RolloutHostObservation{}, err
	}
	return rolloutHostObservation(target, change.Image, change.RestoreData), nil
}

func (fake *rolloutHostsFake) Readiness(
	_ context.Context,
	target RolloutHostTarget,
	change RolloutHostChange,
) (RolloutReadinessObservation, error) {
	if !change.Compensating {
		if err := fake.failReadiness[target.Slot]; err != nil {
			return RolloutReadinessObservation{}, err
		}
	} else if err := fake.failCompensationReadiness[target.Slot]; err != nil {
		return RolloutReadinessObservation{}, err
	}
	return RolloutReadinessObservation{
		ManifestDigest: target.ManifestDigest, RequestID: target.RequestID,
		CompatibilityDigest: target.CompatibilityDigest,
		Slot:                target.Slot, Image: change.Image,
		NodePrincipal:  target.ExpectedNodePrincipal,
		WakuPeerID:     target.ExpectedWakuPeerID,
		CompositeReady: true, Joined: true,
		ReachabilityReady: true, StoreReady: !target.PersistentStore || true,
		ProviderSlots: append([]string(nil), target.RequiredProviderSlots...),
	}, nil
}

func rolloutHostObservation(
	target RolloutHostTarget,
	image string,
	restored bool,
) RolloutHostObservation {
	return RolloutHostObservation{
		ManifestDigest: target.ManifestDigest, RequestID: target.RequestID,
		CompatibilityDigest: target.CompatibilityDigest,
		Slot:                target.Slot, Image: image,
		IdentityPreserved: true, CompleteDataRestored: restored,
	}
}

func (fake *rolloutHostsFake) forwardSlots() []string {
	var out []string
	for _, event := range fake.events {
		if event.kind == "forward" {
			out = append(out, event.slot)
		}
	}
	return out
}

func (fake *rolloutHostsFake) compensationSlots() []string {
	var out []string
	for _, event := range fake.events {
		if event.kind == "compensate" {
			out = append(out, event.slot)
		}
	}
	return out
}

func (fake *rolloutHostsFake) compensationRestoreFlags() []bool {
	var out []bool
	for _, event := range fake.events {
		if event.kind == "compensate" {
			out = append(out, event.restoreData)
		}
	}
	return out
}

type rolloutAuthorityFake struct {
	calls                 int
	persistedBeforeEffect bool
	err                   error
	activated             bool
	loseResponse          bool
	statusErr             error
}

func (fake *rolloutAuthorityFake) Status(
	_ context.Context,
	target RolloutAuthorityTarget,
) (RolloutAuthorityObservation, error) {
	if fake.statusErr != nil {
		return RolloutAuthorityObservation{}, fake.statusErr
	}
	if !fake.activated {
		return RolloutAuthorityObservation{
			ManifestDigest: target.ManifestDigest, RequestID: target.RequestID,
			CompatibilityDigest: target.CompatibilityDigest,
			Activated:           false,
		}, nil
	}
	return rolloutAuthoritySuccess(target), nil
}

func (fake *rolloutAuthorityFake) Activate(
	_ context.Context,
	target RolloutAuthorityTarget,
) (RolloutAuthorityObservation, error) {
	fake.calls++
	fake.persistedBeforeEffect = true
	if fake.err != nil {
		return RolloutAuthorityObservation{}, fake.err
	}
	fake.activated = true
	if fake.loseResponse {
		return RolloutAuthorityObservation{}, errors.New("lost activation response")
	}
	return rolloutAuthoritySuccess(target), nil
}

func rolloutAuthoritySuccess(
	target RolloutAuthorityTarget,
) RolloutAuthorityObservation {
	receipts := make(map[string]string, len(target.Order))
	for _, slot := range target.Order {
		receipts[slot] = "sha256:" +
			"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	}
	return RolloutAuthorityObservation{
		ManifestDigest: target.ManifestDigest, RequestID: target.RequestID,
		CompatibilityDigest: target.CompatibilityDigest,
		Activated:           true, Generation: 2,
		CheckpointDigest: "sha256:" +
			"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		RepositoryPersisted: true, ActiveReceipts: receipts,
	}
}

type rolloutCommitterFake struct {
	committed          bool
	loseCommitResponse bool
	failCommit         bool
}

func (fake *rolloutCommitterFake) Status(
	_ context.Context,
	target RolloutCommitTarget,
) (RolloutCommitObservation, error) {
	return RolloutCommitObservation{
		ManifestDigest: target.ManifestDigest, RequestID: target.RequestID,
		CompatibilityDigest: target.CompatibilityDigest,
		Committed:           fake.committed,
	}, nil
}

func (fake *rolloutCommitterFake) Commit(
	_ context.Context,
	target RolloutCommitTarget,
) (RolloutCommitObservation, error) {
	if fake.failCommit {
		return RolloutCommitObservation{}, errors.New("commit failed")
	}
	fake.committed = true
	observation := RolloutCommitObservation{
		ManifestDigest: target.ManifestDigest, RequestID: target.RequestID,
		CompatibilityDigest: target.CompatibilityDigest,
		Committed:           true,
	}
	if fake.loseCommitResponse {
		return RolloutCommitObservation{}, errors.New("lost response")
	}
	return observation, nil
}
