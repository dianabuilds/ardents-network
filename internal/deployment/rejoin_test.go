package deployment

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRejoinCoordinatorPersistsEveryMonotonicBoundary(t *testing.T) {
	request := validRejoinRequest(t)
	store := &memoryRejoinJournal{}
	restoration := &fakeRejoinRestoration{}
	members := &fakeRejoinMembers{}
	authority := &fakeRejoinAuthority{
		operationID: "rao1_11223344556677889900aabbccddeeff",
		generation:  request.Fence.AuthorityGeneration + 1,
	}
	readiness := &fakeRejoinReadiness{}
	status, err := (RejoinCoordinator{
		Journal: store, Restoration: restoration, Members: members,
		Authority: authority, Readiness: readiness,
		Clock: func() time.Time { return request.StartedAt },
	}).Rejoin(context.Background(), request)

	require.NoError(t, err)
	require.Equal(t, RejoinOutcomeRejoined, status.Outcome)
	require.Equal(t, RejoinPhaseRejoined, status.Phase)
	require.Equal(t, 3, status.DeliveryCount)
	require.Equal(t, 2, status.SurvivorCount)
	require.Equal(t, []RejoinPhase{
		RejoinPhaseRequested,
		RejoinPhasePreflightPersisted,
		RejoinPhaseTargetQuarantined,
		RejoinPhaseAttestationsPrepared,
		RejoinPhaseAuthorityPending,
		RejoinPhaseDeliveriesPrepared,
		RejoinPhaseDeliveriesInstalled,
		RejoinPhaseActivationCommitted,
		RejoinPhaseSurvivorsAcknowledged,
		RejoinPhaseRestorationPending,
		RejoinPhaseRestorationPending,
		RejoinPhaseReadinessVerified,
		RejoinPhaseTargetAcknowledgementPending,
		RejoinPhaseCheckpointPersisted,
		RejoinPhaseRejoined,
	}, store.savedPhases)
	require.Equal(t, 1, restoration.preflightCalls)
	require.Equal(t, 1, restoration.startCalls)
	require.Equal(t, 1, restoration.restoreCalls)
	require.Zero(t, restoration.reisolateCalls)
	require.Equal(t, 1, members.attestCalls)
	require.Equal(t, 1, members.installCalls)
	require.Equal(t, 1, members.survivorCalls)
	require.Equal(t, 1, members.targetCalls)
	require.Equal(t, 1, authority.prepareCalls)
	require.Equal(t, 1, authority.commitCalls)
	require.Equal(t, 1, authority.survivorCalls)
	require.Equal(t, 1, authority.targetCalls)
	require.Equal(t, 1, readiness.verifyCalls)
	require.NotContains(t, status.String(), "p1_")
	require.NotContains(t, status.String(), "sha256:")
}

func TestRejoinCoordinatorRejectsFenceBindingBeforeMutation(t *testing.T) {
	request := validRejoinRequest(t)
	request.Fence.TargetSlot = "node-b"
	store := &memoryRejoinJournal{}
	restoration := &fakeRejoinRestoration{}
	members := &fakeRejoinMembers{}
	authority := &fakeRejoinAuthority{}
	readiness := &fakeRejoinReadiness{}

	_, err := (RejoinCoordinator{
		Journal: store, Restoration: restoration, Members: members,
		Authority: authority, Readiness: readiness,
		Clock: func() time.Time { return request.StartedAt },
	}).Rejoin(context.Background(), request)

	require.ErrorIs(t, err, ErrRejoinJournalBinding)
	require.Nil(t, store.current)
	require.Zero(t, restoration.preflightCalls)
	require.Zero(t, restoration.startCalls)
	require.Zero(t, restoration.restoreCalls)
	require.Zero(t, restoration.reisolateCalls)
	require.Zero(t, members.attestCalls)
	require.Zero(t, authority.prepareCalls)
	require.Zero(t, readiness.verifyCalls)
}

func TestRejoinCoordinatorPreservesPhaseTruthAcrossActivationCommit(t *testing.T) {
	t.Run("before commit removal remains current", func(t *testing.T) {
		request := validRejoinRequest(t)
		store := &memoryRejoinJournal{}
		restoration := &fakeRejoinRestoration{}
		members := &fakeRejoinMembers{}
		authority := &fakeRejoinAuthority{
			operationID: "rao1_11223344556677889900aabbccddeeff",
			generation:  request.Fence.AuthorityGeneration + 1,
			prepareErr:  RejoinDependencyError(RejoinFailureAuthorityUnavailable),
		}

		status, err := (RejoinCoordinator{
			Journal: store, Restoration: restoration, Members: members,
			Authority: authority, Readiness: &fakeRejoinReadiness{},
			Clock: func() time.Time { return request.StartedAt },
		}).Rejoin(context.Background(), request)

		require.NoError(t, err)
		require.Equal(t, RejoinOutcomeRecoveryRequired, status.Outcome)
		require.Equal(t, RejoinFailureAuthorityUnavailable, status.Reason)
		require.Equal(t, RejoinPhaseAuthorityPending, store.current.ResumeFrom)
		require.Empty(t, store.current.OperationID)
		require.Zero(t, store.current.Generation)
		require.Empty(t, store.current.ActivationCheckpointDigest)
		require.Equal(t, 1, restoration.reisolateCalls)
	})

	t.Run("after commit fresh membership is current but incomplete", func(t *testing.T) {
		request := validRejoinRequest(t)
		store := &memoryRejoinJournal{}
		restoration := &fakeRejoinRestoration{}
		members := &fakeRejoinMembers{
			survivorErr: RejoinDependencyError(RejoinFailureSurvivorUnavailable),
		}
		authority := &fakeRejoinAuthority{
			operationID: "rao1_11223344556677889900aabbccddeeff",
			generation:  request.Fence.AuthorityGeneration + 1,
		}
		readiness := &fakeRejoinReadiness{}
		coordinator := RejoinCoordinator{
			Journal: store, Restoration: restoration, Members: members,
			Authority: authority, Readiness: readiness,
			Clock: func() time.Time { return request.StartedAt },
		}

		status, err := coordinator.Rejoin(context.Background(), request)
		require.NoError(t, err)
		require.Equal(t, RejoinOutcomeRecoveryRequired, status.Outcome)
		require.Equal(t, RejoinPhaseRecoveryRequired, status.Phase)
		require.Equal(t, RejoinPhaseActivationCommitted, store.current.ResumeFrom)
		require.Equal(t, request.Fence.AuthorityGeneration+1, store.current.Generation)
		require.NotEmpty(t, store.current.ActivationCheckpointDigest)
		require.Equal(t, 1, restoration.reisolateCalls)
		require.Zero(t, restoration.restoreCalls)

		members.survivorErr = nil
		status, err = coordinator.Rejoin(context.Background(), request)
		require.NoError(t, err)
		require.Equal(t, RejoinOutcomeRejoined, status.Outcome)
		require.Equal(t, 1, authority.prepareCalls)
		require.Equal(t, 1, authority.commitCalls)
		require.Equal(t, 2, members.survivorCalls)
	})
}

func TestRejoinCoordinatorReprovesRestorationAfterAmbiguousTargetReceipt(t *testing.T) {
	request := validRejoinRequest(t)
	store := &memoryRejoinJournal{}
	restoration := &fakeRejoinRestoration{}
	members := &fakeRejoinMembers{}
	authority := &fakeRejoinAuthority{
		operationID: "rao1_11223344556677889900aabbccddeeff",
		generation:  request.Fence.AuthorityGeneration + 1,
		targetErr:   RejoinDependencyError(RejoinFailureAuthorityUnavailable),
	}
	readiness := &fakeRejoinReadiness{}
	coordinator := RejoinCoordinator{
		Journal: store, Restoration: restoration, Members: members,
		Authority: authority, Readiness: readiness,
		Clock: func() time.Time { return request.StartedAt },
	}

	status, err := coordinator.Rejoin(context.Background(), request)
	require.NoError(t, err)
	require.Equal(t, RejoinOutcomeRecoveryRequired, status.Outcome)
	require.Equal(t, RejoinPhaseRestorationPending, store.current.ResumeFrom)
	require.Equal(t, 1, restoration.restoreCalls)
	require.Equal(t, 1, restoration.reisolateCalls)
	require.Equal(t, 1, readiness.verifyCalls)
	require.Equal(t, 1, members.targetCalls)

	authority.targetErr = nil
	status, err = coordinator.Rejoin(context.Background(), request)
	require.NoError(t, err)
	require.Equal(t, RejoinOutcomeRejoined, status.Outcome)
	require.Equal(t, 2, restoration.restoreCalls)
	require.Equal(t, 2, readiness.verifyCalls)
	require.Equal(t, 2, members.targetCalls)
	require.Equal(t, 2, authority.targetCalls)
	require.Equal(t, 1, authority.prepareCalls)
	require.Equal(t, 1, authority.commitCalls)
}

func TestRejoinCoordinatorResumesAfterEveryDurableBoundary(t *testing.T) {
	for failAfter := 1; failAfter <= 14; failAfter++ {
		t.Run(RejoinPhase(rejoinPhaseName(failAfter)).String(), func(t *testing.T) {
			request := validRejoinRequest(t)
			base := &memoryRejoinJournal{}
			store := &crashingRejoinJournal{base: base, failAfter: failAfter}
			restoration := &fakeRejoinRestoration{}
			members := &fakeRejoinMembers{}
			authority := &fakeRejoinAuthority{
				operationID: "rao1_11223344556677889900aabbccddeeff",
				generation:  request.Fence.AuthorityGeneration + 1,
			}
			coordinator := RejoinCoordinator{
				Journal: store, Restoration: restoration, Members: members,
				Authority: authority, Readiness: &fakeRejoinReadiness{},
				Clock: func() time.Time { return request.StartedAt },
			}

			_, err := coordinator.Rejoin(context.Background(), request)
			require.ErrorIs(t, err, errRejoinCrash)
			status, err := coordinator.Rejoin(context.Background(), request)
			require.NoError(t, err)
			require.Equal(t, RejoinOutcomeRejoined, status.Outcome)
			require.Equal(t, RejoinPhaseRejoined, status.Phase)
		})
	}
}

func TestRejoinCoordinatorRequiresFreshAllRecipientDeliveryBeforeCommit(t *testing.T) {
	t.Run("generation must be newer than removal", func(t *testing.T) {
		request := validRejoinRequest(t)
		authority := &fakeRejoinAuthority{
			operationID: "rao1_11223344556677889900aabbccddeeff",
			generation:  request.Fence.AuthorityGeneration,
		}
		members := &fakeRejoinMembers{}
		status, err := (RejoinCoordinator{
			Journal:     &memoryRejoinJournal{},
			Restoration: &fakeRejoinRestoration{}, Members: members,
			Authority: authority, Readiness: &fakeRejoinReadiness{},
			Clock: func() time.Time { return request.StartedAt },
		}).Rejoin(context.Background(), request)
		require.NoError(t, err)
		require.Equal(t, RejoinFailureDeliveryMismatch, status.Reason)
		require.Zero(t, members.installCalls)
		require.Zero(t, authority.commitCalls)
	})

	t.Run("all three pending deliveries are required", func(t *testing.T) {
		request := validRejoinRequest(t)
		authority := &fakeRejoinAuthority{
			operationID: "rao1_11223344556677889900aabbccddeeff",
			generation:  request.Fence.AuthorityGeneration + 1,
		}
		members := &fakeRejoinMembers{omitInstallReceipt: true}
		status, err := (RejoinCoordinator{
			Journal:     &memoryRejoinJournal{},
			Restoration: &fakeRejoinRestoration{}, Members: members,
			Authority: authority, Readiness: &fakeRejoinReadiness{},
			Clock: func() time.Time { return request.StartedAt },
		}).Rejoin(context.Background(), request)
		require.NoError(t, err)
		require.Equal(t, RejoinFailureDeliveryMismatch, status.Reason)
		require.Zero(t, authority.commitCalls)
	})
}

func TestRejoinAuthorizationIntersectionUsesExactExistingOwners(t *testing.T) {
	request := validRejoinRequest(t)
	manifest, targetNode, err := validateRejoinRequest(request)
	require.NoError(t, err)
	target := rejoinTarget(request, manifest, targetNode)
	authorityAdapter := &fakeRejoinAuthority{
		operationID: "rao1_11223344556677889900aabbccddeeff",
		generation:  request.Fence.AuthorityGeneration + 1,
	}
	preparation, err := authorityAdapter.PrepareAdd(
		context.Background(), target, map[string]RejoinAttestation{},
	)
	require.NoError(t, err)

	bindings, err := RejoinAuthorizationIntersection(target, preparation)
	require.NoError(t, err)
	require.Len(t, bindings, 25)
	require.Contains(t, bindings, RejoinAuthorizationBinding{
		Action: "realm.channel.membership.change", ResourceKind: "realm-channel",
		ResourceID: "realm/r1_00112233445566778899aabbccddeeff/channel/00112233445566778899aabbccddeeff",
	})
	for _, principal := range target.RecipientPrincipals {
		require.Contains(t, bindings, RejoinAuthorizationBinding{
			Action: "realm.channel.delivery.prepare", ResourceKind: "principal",
			ResourceID: principal, RecipientPrincipal: principal,
		})
		delivery := preparation.Deliveries[principal]
		deliveryResource := "realm/" + target.RealmID + "/operation/" +
			preparation.OperationID + "/delivery/" + delivery.DeliveryID
		require.Contains(t, bindings, RejoinAuthorizationBinding{
			Action:       "realm.channel.delivery.install",
			ResourceKind: "realm-channel-delivery", ResourceID: deliveryResource,
			RecipientPrincipal: principal,
		})
		require.Contains(t, bindings, RejoinAuthorizationBinding{
			Action:       "realm.channel.delivery.acknowledge",
			ResourceKind: "realm-channel-delivery", ResourceID: deliveryResource,
			RecipientPrincipal: principal,
		})
		require.Contains(t, bindings, RejoinAuthorizationBinding{
			Action:       "realm.channel.generation.activate",
			ResourceKind: "realm-channel-operation",
			ResourceID: "realm/" + target.RealmID + "/operation/" +
				preparation.OperationID,
			RecipientPrincipal: principal,
		})
		require.Contains(t, bindings, RejoinAuthorizationBinding{
			Action:       "realm.channel.activation.acknowledge",
			ResourceKind: "realm-channel-delivery", ResourceID: deliveryResource,
			RecipientPrincipal: principal,
		})
	}
}

func TestRejoinCoordinatorRejectsTamperedPersistedRecipientState(t *testing.T) {
	request := validRejoinRequest(t)
	store := &memoryRejoinJournal{}
	authority := &fakeRejoinAuthority{
		operationID: "rao1_11223344556677889900aabbccddeeff",
		generation:  request.Fence.AuthorityGeneration + 1,
	}
	coordinator := RejoinCoordinator{
		Journal: store, Restoration: &fakeRejoinRestoration{},
		Members: &fakeRejoinMembers{}, Authority: authority,
		Readiness: &fakeRejoinReadiness{},
		Clock:     func() time.Time { return request.StartedAt },
	}
	_, err := coordinator.Rejoin(context.Background(), request)
	require.NoError(t, err)

	tampered := cloneRejoinTransaction(*store.current)
	for principal, delivery := range tampered.Deliveries {
		delete(tampered.Deliveries, principal)
		tampered.Deliveries["p1_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"] = delivery
		break
	}
	store.current = &tampered
	restoration := &fakeRejoinRestoration{}
	members := &fakeRejoinMembers{}
	authority = &fakeRejoinAuthority{}
	readiness := &fakeRejoinReadiness{}

	_, err = (RejoinCoordinator{
		Journal: store, Restoration: restoration, Members: members,
		Authority: authority, Readiness: readiness,
		Clock: func() time.Time { return request.StartedAt },
	}).Rejoin(context.Background(), request)

	require.ErrorIs(t, err, ErrRejoinJournalBinding)
	require.Zero(t, restoration.preflightCalls)
	require.Zero(t, members.attestCalls)
	require.Zero(t, authority.prepareCalls)
	require.Zero(t, readiness.verifyCalls)
}

func validRejoinRequest(t *testing.T) RejoinRequest {
	t.Helper()
	raw := fenceManifest(t)
	now := time.Date(2026, 7, 29, 14, 0, 0, 0, time.UTC)
	fenceRequest := FenceRequest{
		Manifest: raw, TargetSlot: "node-a", Reason: FenceReasonMembershipRemoved,
		Actor:     "p1_euydwrsrlrtxe7misopktnf7zlk6b27waegboirnhbbu4wlen55a",
		RequestID: "fence-before-rejoin", StartedAt: now.Add(-time.Hour),
		Deadline: now.Add(-55 * time.Minute),
	}
	fence := fenceTransactionAtPhase(t, fenceRequest, FencePhaseFenced)
	return RejoinRequest{
		Manifest: raw, TargetSlot: "node-a", Actor: fenceRequest.Actor,
		ChannelID: "00112233445566778899aabbccddeeff",
		RequestID: "rejoin-node-a-001", StartedAt: now,
		Deadline: now.Add(10 * time.Minute), Fence: fence,
	}
}

type memoryRejoinJournal struct {
	current     *RejoinTransaction
	savedPhases []RejoinPhase
}

func (store *memoryRejoinJournal) Load(context.Context) (RejoinTransaction, bool, error) {
	if store.current == nil {
		return RejoinTransaction{}, false, nil
	}
	return cloneRejoinTransaction(*store.current), true, nil
}

func (store *memoryRejoinJournal) Save(
	_ context.Context,
	expectedRevision uint64,
	transaction RejoinTransaction,
) error {
	if store.current == nil {
		if expectedRevision != 0 {
			return ErrRejoinJournalConflict
		}
	} else if store.current.Revision != expectedRevision {
		return ErrRejoinJournalConflict
	}
	transaction.Revision = expectedRevision + 1
	cloned := cloneRejoinTransaction(transaction)
	store.current = &cloned
	store.savedPhases = append(store.savedPhases, transaction.Phase)
	return nil
}

var errRejoinCrash = errors.New("simulated coordinator crash")

type crashingRejoinJournal struct {
	base      *memoryRejoinJournal
	failAfter int
	saves     int
	failed    bool
}

func (store *crashingRejoinJournal) Load(
	ctx context.Context,
) (RejoinTransaction, bool, error) {
	return store.base.Load(ctx)
}

func (store *crashingRejoinJournal) Save(
	ctx context.Context,
	expectedRevision uint64,
	transaction RejoinTransaction,
) error {
	if err := store.base.Save(ctx, expectedRevision, transaction); err != nil {
		return err
	}
	store.saves++
	if !store.failed && store.saves == store.failAfter {
		store.failed = true
		return errRejoinCrash
	}
	return nil
}

func rejoinPhaseName(index int) string {
	phases := []RejoinPhase{
		RejoinPhaseRequested, RejoinPhasePreflightPersisted,
		RejoinPhaseTargetQuarantined, RejoinPhaseAttestationsPrepared,
		RejoinPhaseAuthorityPending, RejoinPhaseDeliveriesPrepared,
		RejoinPhaseDeliveriesInstalled, RejoinPhaseActivationCommitted,
		RejoinPhaseSurvivorsAcknowledged, RejoinPhaseRestorationPending,
		RejoinPhaseReadinessVerified, RejoinPhaseTargetAcknowledgementPending,
		RejoinPhaseCheckpointPersisted, RejoinPhaseRejoined,
	}
	return string(phases[index-1])
}

func (phase RejoinPhase) String() string { return string(phase) }

type fakeRejoinRestoration struct {
	preflightCalls int
	startCalls     int
	restoreCalls   int
	reisolateCalls int
}

func (fake *fakeRejoinRestoration) Preflight(
	_ context.Context,
	target RejoinTarget,
) (RejoinPreflightResult, error) {
	fake.preflightCalls++
	return RejoinPreflightResult{
		ObservedAt: target.StartedAt, ClockSkewSecond: 7, Isolated: true,
	}, nil
}

func (fake *fakeRejoinRestoration) StartQuarantined(
	_ context.Context,
	target RejoinTarget,
) (RejoinTargetObservation, error) {
	fake.startCalls++
	return RejoinTargetObservation{
		Principal: target.TargetPrincipal, WakuPeerID: target.ExpectedWakuPeerID,
		Image: target.ExpectedImage, ObservedAt: target.StartedAt,
		ClockSkewSecond: 7,
	}, nil
}

func (fake *fakeRejoinRestoration) Restore(context.Context, RejoinTarget) error {
	fake.restoreCalls++
	return nil
}

func (fake *fakeRejoinRestoration) Reisolate(context.Context, RejoinTarget) error {
	fake.reisolateCalls++
	return nil
}

type fakeRejoinMembers struct {
	attestCalls        int
	installCalls       int
	survivorCalls      int
	targetCalls        int
	attestErr          error
	installErr         error
	survivorErr        error
	targetErr          error
	omitInstallReceipt bool
}

func (fake *fakeRejoinMembers) Attest(
	_ context.Context,
	target RejoinTarget,
) (map[string]RejoinAttestation, error) {
	fake.attestCalls++
	if fake.attestErr != nil {
		return nil, fake.attestErr
	}
	result := make(map[string]RejoinAttestation, len(target.RecipientPrincipals))
	for index, principal := range target.RecipientPrincipals {
		result[principal] = RejoinAttestation{
			RecipientPrincipal: principal,
			Digest:             fenceDigest(byte('a' + index)),
		}
	}
	return result, nil
}

func (fake *fakeRejoinMembers) InstallPending(
	_ context.Context,
	_ RejoinTarget,
	deliveries map[string]RejoinDelivery,
) (map[string]string, error) {
	fake.installCalls++
	if fake.installErr != nil {
		return nil, fake.installErr
	}
	result := make(map[string]string, len(deliveries))
	for principal := range deliveries {
		result[principal] = fenceDigest('1')
	}
	if fake.omitInstallReceipt {
		for principal := range result {
			delete(result, principal)
			break
		}
	}
	return result, nil
}

func (fake *fakeRejoinMembers) ActivateSurvivors(
	_ context.Context,
	target RejoinTarget,
	_ string,
	_ uint32,
) (map[string]string, error) {
	fake.survivorCalls++
	if fake.survivorErr != nil {
		return nil, fake.survivorErr
	}
	return map[string]string{
		target.SurvivorSlots[0]: fenceDigest('2'),
		target.SurvivorSlots[1]: fenceDigest('3'),
	}, nil
}

func (fake *fakeRejoinMembers) ActivateTarget(
	context.Context,
	RejoinTarget,
	string,
	uint32,
) (string, error) {
	fake.targetCalls++
	if fake.targetErr != nil {
		return "", fake.targetErr
	}
	return fenceDigest('4'), nil
}

type fakeRejoinAuthority struct {
	operationID   string
	generation    uint32
	prepareCalls  int
	commitCalls   int
	survivorCalls int
	targetCalls   int
	prepareErr    error
	commitErr     error
	survivorErr   error
	targetErr     error
}

func (fake *fakeRejoinAuthority) PrepareAdd(
	_ context.Context,
	target RejoinTarget,
	_ map[string]RejoinAttestation,
) (RejoinPreparation, error) {
	fake.prepareCalls++
	if fake.prepareErr != nil {
		return RejoinPreparation{}, fake.prepareErr
	}
	deliveries := make(map[string]RejoinDelivery, len(target.RecipientPrincipals))
	for index, principal := range target.RecipientPrincipals {
		deliveries[principal] = RejoinDelivery{
			RecipientPrincipal: principal,
			DeliveryID:         "rad1_11223344556677889900aabbccddeef" + string(rune('a'+index)),
			EnvelopeDigest:     fenceDigest(byte('d' + index)),
		}
	}
	return RejoinPreparation{
		OperationID: fake.operationID, Generation: fake.generation,
		Deliveries: deliveries, CheckpointDigest: fenceDigest('5'),
		RepositoryPersisted: true,
	}, nil
}

func (fake *fakeRejoinAuthority) CommitActivation(
	context.Context,
	RejoinTarget,
	string,
	uint32,
) (RejoinActivationResult, error) {
	fake.commitCalls++
	if fake.commitErr != nil {
		return RejoinActivationResult{}, fake.commitErr
	}
	return RejoinActivationResult{
		OperationID: fake.operationID, Generation: fake.generation,
		CheckpointDigest: fenceDigest('c'), RepositoryPersisted: true,
	}, nil
}

func (fake *fakeRejoinAuthority) AcknowledgeSurvivors(
	_ context.Context,
	_ RejoinTarget,
	_ string,
	_ uint32,
	receipts map[string]string,
) error {
	fake.survivorCalls++
	if fake.survivorErr != nil {
		return fake.survivorErr
	}
	if len(receipts) != 2 {
		return RejoinDependencyError(RejoinFailureSurvivorMismatch)
	}
	return nil
}

func (fake *fakeRejoinAuthority) CompleteTarget(
	context.Context,
	RejoinTarget,
	string,
	uint32,
	string,
) (RejoinFinalResult, error) {
	fake.targetCalls++
	if fake.targetErr != nil {
		return RejoinFinalResult{}, fake.targetErr
	}
	return RejoinFinalResult{
		OperationID: fake.operationID, Generation: fake.generation,
		CheckpointDigest: fenceDigest('f'), RepositoryPersisted: true,
	}, nil
}

type fakeRejoinReadiness struct {
	verifyCalls int
}

func (fake *fakeRejoinReadiness) Verify(
	_ context.Context,
	target RejoinTarget,
	_ uint32,
) (RejoinReadinessResult, error) {
	fake.verifyCalls++
	return RejoinReadinessResult{
		Principal: target.TargetPrincipal, WakuPeerID: target.ExpectedWakuPeerID,
		Image: target.ExpectedImage, ObservedAt: target.StartedAt,
		ClockSkewSecond: 7, Joined: true, CompositeReady: true,
	}, nil
}
