package deployment

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestFenceCoordinatorPersistsEveryMonotonicBoundary(t *testing.T) {
	raw := fenceManifest(t)
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	store := &memoryFenceJournal{}
	isolation := &fakeFenceIsolation{
		controls: validFenceControls("p1_euydwrsrlrtxe7misopktnf7zlk6b27waegboirnhbbu4wlen55a"),
	}
	authority := &fakeFenceAuthority{
		operationID: "rao1_00112233445566778899aabbccddeeff",
		result: FenceAuthorityResult{
			Generation: 4, CheckpointDigest: fenceDigest('d'),
			RepositoryPersisted: true,
			SurvivorReceipts: map[string]string{
				"node-b": fenceDigest('e'),
				"node-c": fenceDigest('f'),
			},
		},
	}
	status, err := (FenceCoordinator{
		Journal: store, Isolation: isolation, Authority: authority,
		Clock: func() time.Time { return now },
	}).Fence(context.Background(), FenceRequest{
		Manifest: raw, TargetSlot: "node-a", Reason: FenceReasonMembershipRemoved,
		Actor:     "p1_euydwrsrlrtxe7misopktnf7zlk6b27waegboirnhbbu4wlen55a",
		RequestID: "fence-node-a-001", StartedAt: now, Deadline: now.Add(5 * time.Minute),
	})
	require.NoError(t, err)
	require.Equal(t, FenceOutcomeFenced, status.Outcome)
	require.Equal(t, FencePhaseFenced, status.Phase)
	require.Equal(t, 3, status.ControlCount)
	require.Equal(t, 2, status.SurvivorCount)
	require.Equal(t, []FencePhase{
		FencePhaseRequested, FencePhaseIsolationPending,
		FencePhaseEvidencePersisted, FencePhaseAuthorityPending,
		FencePhaseCheckpointPersisted, FencePhasePeersAcknowledged,
		FencePhaseFenced,
	}, store.savedPhases)
	require.Equal(t, 1, isolation.preflightCalls)
	require.Equal(t, 1, isolation.enforceCalls)
	require.Equal(t, 1, authority.prepareCalls)
	require.Equal(t, 1, authority.completeCalls)
	require.Equal(t, ActionTopologyNodeFence, authority.lastPrepare.Action)
	require.Equal(t, "node:p1_euydwrsrlrtxe7misopktnf7zlk6b27waegboirnhbbu4wlen55a", authority.lastPrepare.Resource)
	require.Equal(t, authority.operationID, authority.lastEvidence.OperationID)
	require.NotContains(t, status.String(), "p1_")
	require.NotContains(t, status.String(), "sha256:")
}

func TestFenceCoordinatorResumesWithoutRepeatingIsolation(t *testing.T) {
	raw := fenceManifest(t)
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	request := FenceRequest{
		Manifest: raw, TargetSlot: "node-a", Reason: FenceReasonMembershipRemoved,
		Actor:     "p1_euydwrsrlrtxe7misopktnf7zlk6b27waegboirnhbbu4wlen55a",
		RequestID: "fence-node-a-resume", StartedAt: now, Deadline: now.Add(5 * time.Minute),
	}
	transaction, err := newFenceTransaction(request, mustFenceManifest(t, raw))
	require.NoError(t, err)
	transaction.Revision = 3
	transaction.Phase = FencePhaseEvidencePersisted
	transaction.OperationID = "rao1_00112233445566778899aabbccddeeff"
	transaction.Evidence = &DeploymentFenceEvidence{
		Version: DeploymentFenceEvidenceVersion, RealmID: "r1_00112233445566778899aabbccddeeff",
		OperationID:     transaction.OperationID,
		TargetPrincipal: request.Actor, ManifestDigest: transaction.ManifestDigest,
		RequestID: request.RequestID, Reason: request.Reason, ObservedAt: now,
		Controls: validFenceControls(request.Actor),
	}
	store := &memoryFenceJournal{current: &transaction}
	isolation := &fakeFenceIsolation{}
	authority := &fakeFenceAuthority{
		result: FenceAuthorityResult{
			Generation: 5, CheckpointDigest: fenceDigest('a'), RepositoryPersisted: true,
			SurvivorReceipts: map[string]string{
				"node-b": fenceDigest('b'), "node-c": fenceDigest('c'),
			},
		},
	}
	status, err := (FenceCoordinator{
		Journal: store, Isolation: isolation, Authority: authority,
		Clock: func() time.Time { return now },
	}).Fence(context.Background(), request)
	require.NoError(t, err)
	require.Equal(t, FenceOutcomeFenced, status.Outcome)
	require.Zero(t, isolation.preflightCalls)
	require.Zero(t, isolation.enforceCalls)
	require.Zero(t, authority.prepareCalls)
	require.Equal(t, 1, authority.completeCalls)
}

func TestFenceCoordinatorResumesFromEveryDurablePhase(t *testing.T) {
	raw := fenceManifest(t)
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	request := FenceRequest{
		Manifest: raw, TargetSlot: "node-a", Reason: FenceReasonMembershipRemoved,
		Actor:     "p1_euydwrsrlrtxe7misopktnf7zlk6b27waegboirnhbbu4wlen55a",
		RequestID: "fence-node-a-crash-boundaries",
		StartedAt: now, Deadline: now.Add(5 * time.Minute),
	}
	tests := []struct {
		phase                                 FencePhase
		preflight, enforce, prepare, complete int
	}{
		{FencePhaseRequested, 1, 1, 1, 1},
		{FencePhaseIsolationPending, 0, 1, 1, 1},
		{FencePhaseEvidencePersisted, 0, 0, 0, 1},
		{FencePhaseAuthorityPending, 0, 0, 0, 1},
		{FencePhaseCheckpointPersisted, 0, 0, 0, 0},
		{FencePhasePeersAcknowledged, 0, 0, 0, 0},
		{FencePhaseFenced, 0, 0, 0, 0},
	}
	for _, test := range tests {
		t.Run(string(test.phase), func(t *testing.T) {
			transaction := fenceTransactionAtPhase(t, request, test.phase)
			store := &memoryFenceJournal{current: &transaction}
			isolation := &fakeFenceIsolation{controls: validFenceControls(request.Actor)}
			authority := &fakeFenceAuthority{
				operationID: transaction.OperationID,
				result: FenceAuthorityResult{
					Generation: 4, CheckpointDigest: fenceDigest('d'),
					RepositoryPersisted: true,
					SurvivorReceipts: map[string]string{
						"node-b": fenceDigest('e'), "node-c": fenceDigest('f'),
					},
				},
			}
			if authority.operationID == "" {
				authority.operationID = "rao1_00112233445566778899aabbccddeeff"
			}
			status, err := (FenceCoordinator{
				Journal: store, Isolation: isolation, Authority: authority,
				Clock: func() time.Time { return now },
			}).Fence(context.Background(), request)
			require.NoError(t, err)
			require.Equal(t, FenceOutcomeFenced, status.Outcome)
			require.Equal(t, test.preflight, isolation.preflightCalls)
			require.Equal(t, test.enforce, isolation.enforceCalls)
			require.Equal(t, test.prepare, authority.prepareCalls)
			require.Equal(t, test.complete, authority.completeCalls)
		})
	}
}

func TestFenceCoordinatorFailsClosedBeforeMutationForBindingMismatch(t *testing.T) {
	raw := fenceManifest(t)
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	request := FenceRequest{
		Manifest: raw, TargetSlot: "node-a", Reason: FenceReasonMembershipRemoved,
		Actor:     "p1_euydwrsrlrtxe7misopktnf7zlk6b27waegboirnhbbu4wlen55a",
		RequestID: "fence-node-a-binding", StartedAt: now, Deadline: now.Add(5 * time.Minute),
	}
	transaction, err := newFenceTransaction(request, mustFenceManifest(t, raw))
	require.NoError(t, err)
	transaction.RequestID = "another-request"
	store := &memoryFenceJournal{current: &transaction}
	isolation := &fakeFenceIsolation{}
	authority := &fakeFenceAuthority{}
	_, err = (FenceCoordinator{
		Journal: store, Isolation: isolation, Authority: authority,
		Clock: func() time.Time { return now },
	}).Fence(context.Background(), request)
	require.ErrorIs(t, err, ErrFenceJournalBinding)
	require.Zero(t, isolation.preflightCalls)
	require.Zero(t, isolation.enforceCalls)
	require.Zero(t, authority.prepareCalls)
	require.Zero(t, authority.completeCalls)
}

func TestFenceCoordinatorRejectsInvalidEvidenceBeforeAuthority(t *testing.T) {
	raw := fenceManifest(t)
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	controls := validFenceControls(
		"p1_euydwrsrlrtxe7misopktnf7zlk6b27waegboirnhbbu4wlen55a",
	)
	controls = controls[:2]
	store := &memoryFenceJournal{}
	authority := &fakeFenceAuthority{}
	status, err := (FenceCoordinator{
		Journal:   store,
		Isolation: &fakeFenceIsolation{controls: controls},
		Authority: authority, Clock: func() time.Time { return now },
	}).Fence(context.Background(), FenceRequest{
		Manifest: raw, TargetSlot: "node-a", Reason: FenceReasonMembershipRemoved,
		Actor:     "p1_euydwrsrlrtxe7misopktnf7zlk6b27waegboirnhbbu4wlen55a",
		RequestID: "fence-node-a-invalid-evidence",
		StartedAt: now, Deadline: now.Add(5 * time.Minute),
	})
	require.NoError(t, err)
	require.Equal(t, FenceOutcomeRecoveryRequired, status.Outcome)
	require.Equal(t, FenceFailureInvalidEvidence, status.Reason)
	require.Zero(t, authority.prepareCalls)
	require.Zero(t, authority.completeCalls)
	require.Equal(t, FencePhaseRecoveryRequired, store.current.Phase)
	require.Equal(t, FencePhaseIsolationPending, store.current.ResumeFrom)
}

func TestFenceCoordinatorRequiresExactTwoSurvivorReceipts(t *testing.T) {
	raw := fenceManifest(t)
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	store := &memoryFenceJournal{}
	status, err := (FenceCoordinator{
		Journal: store,
		Isolation: &fakeFenceIsolation{controls: validFenceControls(
			"p1_euydwrsrlrtxe7misopktnf7zlk6b27waegboirnhbbu4wlen55a",
		)},
		Authority: &fakeFenceAuthority{
			operationID: "rao1_00112233445566778899aabbccddeeff",
			result: FenceAuthorityResult{
				Generation: 4, CheckpointDigest: fenceDigest('d'),
				RepositoryPersisted: true,
				SurvivorReceipts:    map[string]string{"node-b": fenceDigest('e')},
			},
		},
		Clock: func() time.Time { return now },
	}).Fence(context.Background(), FenceRequest{
		Manifest: raw, TargetSlot: "node-a", Reason: FenceReasonMembershipRemoved,
		Actor:     "p1_euydwrsrlrtxe7misopktnf7zlk6b27waegboirnhbbu4wlen55a",
		RequestID: "fence-node-a-one-survivor",
		StartedAt: now, Deadline: now.Add(5 * time.Minute),
	})
	require.NoError(t, err)
	require.Equal(t, FenceOutcomeRecoveryRequired, status.Outcome)
	require.Equal(t, FenceFailureSurvivorMismatch, status.Reason)
	require.Equal(t, FencePhaseRecoveryRequired, store.current.Phase)
	require.Equal(t, FencePhaseAuthorityPending, store.current.ResumeFrom)
}

func fenceManifest(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "public-direct.json"))
	require.NoError(t, err)
	return raw
}

func mustFenceManifest(t *testing.T, raw []byte) topologyManifest {
	t.Helper()
	manifest, err := decodeTopology(raw)
	require.NoError(t, err)
	require.NoError(t, validateTopology(manifest))
	return manifest
}

func fenceDigest(value byte) string {
	raw := make([]byte, 64)
	for index := range raw {
		raw[index] = value
	}
	return "sha256:" + string(raw)
}

func validFenceControls(actor string) []FenceControlReceipt {
	return []FenceControlReceipt{
		{Kind: FenceControlTargetIngressBlocked, Actor: actor, ReceiptDigest: fenceDigest('a')},
		{Kind: FenceControlDiscoveryWithdrawn, Actor: actor, ReceiptDigest: fenceDigest('b')},
		{Kind: FenceControlPeerIDDenied, Actor: actor, ReceiptDigest: fenceDigest('c')},
	}
}

func fenceTransactionAtPhase(
	t *testing.T,
	request FenceRequest,
	phase FencePhase,
) FenceTransaction {
	t.Helper()
	manifest := mustFenceManifest(t, request.Manifest)
	transaction, err := newFenceTransaction(request, manifest)
	require.NoError(t, err)
	transaction.Revision = uint64(fencePhaseOrder(phase))
	transaction.Phase = phase
	if fencePhaseOrder(phase) >= fencePhaseOrder(FencePhaseEvidencePersisted) {
		transaction.OperationID = "rao1_00112233445566778899aabbccddeeff"
		transaction.Evidence = &DeploymentFenceEvidence{
			Version: DeploymentFenceEvidenceVersion,
			RealmID: manifest.Authority.RealmID, OperationID: transaction.OperationID,
			TargetPrincipal: request.Actor, ManifestDigest: transaction.ManifestDigest,
			RequestID: request.RequestID, Reason: request.Reason, ObservedAt: request.StartedAt,
			Controls: validFenceControls(request.Actor),
		}
	}
	if fencePhaseOrder(phase) >= fencePhaseOrder(FencePhaseCheckpointPersisted) {
		transaction.AuthorityGeneration = 4
		transaction.CheckpointDigest = fenceDigest('d')
		transaction.RepositoryPersisted = true
		transaction.SurvivorReceipts = map[string]string{
			"node-b": fenceDigest('e'), "node-c": fenceDigest('f'),
		}
	}
	return transaction
}

type memoryFenceJournal struct {
	current     *FenceTransaction
	savedPhases []FencePhase
}

func (store *memoryFenceJournal) Load(context.Context) (FenceTransaction, bool, error) {
	if store.current == nil {
		return FenceTransaction{}, false, nil
	}
	return cloneFenceTransaction(*store.current), true, nil
}

func (store *memoryFenceJournal) Save(
	_ context.Context,
	expectedRevision uint64,
	transaction FenceTransaction,
) error {
	if store.current == nil {
		if expectedRevision != 0 {
			return ErrFenceJournalConflict
		}
	} else if store.current.Revision != expectedRevision {
		return ErrFenceJournalConflict
	}
	transaction.Revision = expectedRevision + 1
	cloned := cloneFenceTransaction(transaction)
	store.current = &cloned
	store.savedPhases = append(store.savedPhases, transaction.Phase)
	return nil
}

type fakeFenceIsolation struct {
	controls       []FenceControlReceipt
	preflightErr   error
	enforceErr     error
	preflightCalls int
	enforceCalls   int
}

func (fake *fakeFenceIsolation) Preflight(context.Context, FenceTarget) error {
	fake.preflightCalls++
	return fake.preflightErr
}

func (fake *fakeFenceIsolation) Enforce(context.Context, FenceTarget) ([]FenceControlReceipt, error) {
	fake.enforceCalls++
	return append([]FenceControlReceipt(nil), fake.controls...), fake.enforceErr
}

type fakeFenceAuthority struct {
	operationID   string
	result        FenceAuthorityResult
	prepareErr    error
	completeErr   error
	prepareCalls  int
	completeCalls int
	lastPrepare   FenceAuthorityRequest
	lastEvidence  DeploymentFenceEvidence
}

func (fake *fakeFenceAuthority) PrepareRemoval(
	_ context.Context,
	request FenceAuthorityRequest,
) (string, error) {
	fake.prepareCalls++
	fake.lastPrepare = request
	return fake.operationID, fake.prepareErr
}

func (fake *fakeFenceAuthority) CompleteRemoval(
	_ context.Context,
	evidence DeploymentFenceEvidence,
) (FenceAuthorityResult, error) {
	fake.completeCalls++
	fake.lastEvidence = evidence
	return fake.result, fake.completeErr
}
