package authority

import (
	"bytes"
	"context"
	"crypto/ed25519"
	cryptorand "crypto/rand"
	"errors"
	"path/filepath"
	"testing"
	"time"

	identityapi "ardents/internal/identity"
	identitycapability "ardents/internal/identity/capability"
	identityprincipal "ardents/internal/identity/principal"
	identitytrust "ardents/internal/identity/trust"

	"github.com/stretchr/testify/require"
)

func TestAuthorityAddsThenRemovesMemberWithFreshGenerationRevocationAndFence(t *testing.T) {
	ctx := context.Background()
	fixture := newServiceFixture(t)
	fixture.service.random = cryptorand.Reader
	genesis, err := fixture.service.CreateOrReopen(
		ctx, fixture.createCommand(),
		CreateRequest{
			Version: ContractVersion, RequestID: "membership-genesis",
			RealmClass: RealmClassProduction,
		},
	)
	require.NoError(t, err)

	memberA, principalA, attestationA := newMembershipMember(t, fixture, 0xa1)
	memberB, principalB, attestationB := newMembershipMember(t, fixture, 0xb1)
	initial := issueAndInstallMembershipInitial(
		t, ctx, fixture, genesis.RealmID, memberA, attestationA,
	)

	added, err := fixture.service.ChangeChannelMembership(
		ctx, membershipCommand(genesis.RealmID, initial.ChannelID),
		MembershipChangeRequest{
			Version: ContractVersion, RequestID: "membership-add-b",
			RealmID: genesis.RealmID, ChannelID: initial.ChannelID,
			Change: MembershipChangeAdd, TargetPrincipal: principalB,
			RecipientAttestations: []identityapi.CapabilityDeliveryAttestation{
				attestationB, attestationA,
			},
			ValidFor: time.Hour, DrainFor: 15 * time.Minute,
		},
	)
	require.NoError(t, err)
	require.Equal(t, MemberStateCandidate, added.MembershipChange.State)
	require.Equal(t, uint32(2), added.PendingGeneration)
	require.Len(t, added.Deliveries, 2)
	require.NoError(t, validateMembershipChangeRecord(fixture.store.state.Rotations[0]))
	require.NoError(t, validateRotationRecord(fixture.store.state, fixture.store.state.Rotations[0]))
	require.NoError(t, validateLedger(fixture.store.state))
	restartMembershipAuthority(fixture)

	addReceipts := installMembershipDeliveries(
		t, ctx, fixture, genesis.RealmID, added, map[string]*identitycapability.Service{
			principalA: memberA,
			principalB: memberB,
		},
	)
	require.Zero(t, memberB.GenerationReadiness(initial.ChannelID).CurrentGeneration)
	require.Equal(t, uint32(2), memberB.GenerationReadiness(initial.ChannelID).PendingGeneration)
	restartMembershipAuthority(fixture)
	addActivation := commitMembershipActivation(t, ctx, fixture, genesis.RealmID, added)
	restartMembershipAuthority(fixture)
	for index, principal := range []string{principalA, principalB} {
		member := map[string]*identitycapability.Service{
			principalA: memberA,
			principalB: memberB,
		}[principal]
		active, activateErr := member.ActivateGeneration(addActivation.Activation)
		require.NoError(t, activateErr)
		active, activateErr = member.ConfirmGenerationRuntimeAdoption(addActivation.Activation)
		require.NoError(t, activateErr)
		addReceipts[principal] = active
		result := acknowledgeMembershipActive(
			t, ctx, fixture, genesis.RealmID, added, active,
		)
		if index == 1 {
			require.Equal(t, DeliveryPhaseCompleted, result.Phase)
		}
	}
	require.Equal(t, uint32(2), memberB.GenerationReadiness(initial.ChannelID).CurrentGeneration)
	require.Zero(t, memberB.GenerationReadiness(initial.ChannelID).PreviousGeneration)
	require.Len(t, fixture.store.state.Members, 2)
	addReplay, err := fixture.service.ChangeChannelMembership(
		ctx, membershipCommand(genesis.RealmID, initial.ChannelID),
		MembershipChangeRequest{
			Version: ContractVersion, RequestID: "membership-add-b",
			RealmID: genesis.RealmID, ChannelID: initial.ChannelID,
			Change: MembershipChangeAdd, TargetPrincipal: principalB,
			RecipientAttestations: []identityapi.CapabilityDeliveryAttestation{
				attestationB, attestationA,
			},
			ValidFor: time.Hour, DrainFor: 15 * time.Minute,
		},
	)
	require.NoError(t, err)
	require.Equal(t, added.OperationID, addReplay.OperationID)
	require.Equal(t, DeliveryPhaseCompleted, addReplay.Phase)

	var removedGrant identityapi.CapabilityGrant
	for _, stored := range fixture.store.state.Channels[0].CurrentGrants {
		grant, ok := stored.restore()
		require.True(t, ok)
		if grant.SubjectPrincipal == principalB {
			removedGrant = grant
		}
	}
	require.Equal(t, principalB, removedGrant.SubjectPrincipal)

	removed, err := fixture.service.ChangeChannelMembership(
		ctx, membershipCommand(genesis.RealmID, initial.ChannelID),
		MembershipChangeRequest{
			Version: ContractVersion, RequestID: "membership-remove-b",
			RealmID: genesis.RealmID, ChannelID: initial.ChannelID,
			Change: MembershipChangeRemove, TargetPrincipal: principalB,
			RecipientAttestations: []identityapi.CapabilityDeliveryAttestation{attestationA},
			ValidFor:              time.Hour, DrainFor: 15 * time.Minute,
		},
	)
	require.NoError(t, err)
	require.Equal(t, MemberStateSuspended, removed.MembershipChange.State)
	require.Len(t, removed.Deliveries, 1)
	require.Equal(t, principalA, removed.Deliveries[0].RecipientPrincipal)
	restartMembershipAuthority(fixture)
	removeReceipts := installMembershipDeliveries(
		t, ctx, fixture, genesis.RealmID, removed,
		map[string]*identitycapability.Service{principalA: memberA},
	)
	restartMembershipAuthority(fixture)
	removeActivation := commitMembershipActivation(t, ctx, fixture, genesis.RealmID, removed)
	activeA, err := memberA.ActivateGeneration(removeActivation.Activation)
	require.NoError(t, err)
	activeA, err = memberA.ConfirmGenerationRuntimeAdoption(removeActivation.Activation)
	require.NoError(t, err)
	removeReceipts[principalA] = activeA
	resourceA, valid := GenerationDeliveryResource(
		genesis.RealmID, removed.OperationID, activeA.DeliveryID,
	)
	require.True(t, valid)
	_, err = fixture.service.AcknowledgeChannelActivation(
		ctx, Command{
			Actor: "operator", Effective: "operator", Action: ActionAcknowledgeActivation,
			ResourceKind: ResourceKindGenerationDelivery, ResourceID: resourceA,
		},
		ActivationAcknowledgeRequest{
			Version: ContractVersion, RealmID: genesis.RealmID,
			OperationID: removed.OperationID, ApprovedHost: false, Receipt: activeA,
		},
	)
	require.ErrorIs(t, err, ErrPermissionDenied)
	pendingFence := acknowledgeMembershipActive(
		t, ctx, fixture, genesis.RealmID, removed, activeA,
	)
	require.Equal(t, DeliveryPhaseActivationCommitted, pendingFence.Phase)
	restartMembershipAuthority(fixture)

	err = memberA.AuthorizeCapabilitySender(identityapi.CapabilitySenderUse{
		GrantID: removedGrant.GrantID, ChannelID: removedGrant.ChannelID,
		Generation: removedGrant.Generation, Subject: principalB,
		Permission: identityapi.CapabilityPublish, Scope: removedGrant.Scope,
		At: fixture.clock(), ObservedAt: fixture.clock(),
	})
	require.Error(t, err, "removed sender must fail authorization before replay admission")

	evidence := validFenceEvidence(
		genesis.RealmID, removed.OperationID, principalB, "operator", fixture.clock(),
	)
	forged := evidence
	forged.Controls = append([]DeploymentFenceControl(nil), evidence.Controls...)
	forged.Controls[0].Actor = "unapproved-operator"
	_, err = fixture.service.SubmitDeploymentFenceEvidence(
		ctx, Command{
			Actor: "operator", Effective: "operator",
			Action: ActionChangeMembership, ResourceKind: ResourceKindChannel,
			ResourceID: ChannelResource(genesis.RealmID, initial.ChannelID),
		},
		FenceEvidenceRequest{
			Version: ContractVersion, RealmID: genesis.RealmID,
			ChannelID: initial.ChannelID, OperationID: removed.OperationID,
			Evidence: forged,
		},
	)
	require.ErrorIs(t, err, ErrPermissionDenied)
	fenced, err := fixture.service.SubmitDeploymentFenceEvidence(
		ctx, Command{
			Actor: "operator", Effective: "operator",
			Action: ActionChangeMembership, ResourceKind: ResourceKindChannel,
			ResourceID: ChannelResource(genesis.RealmID, initial.ChannelID),
		},
		FenceEvidenceRequest{
			Version: ContractVersion, RealmID: genesis.RealmID,
			ChannelID: initial.ChannelID, OperationID: removed.OperationID,
			Evidence: evidence,
		},
	)
	require.NoError(t, err)
	require.Equal(t, DeliveryPhaseCompleted, fenced.Phase)
	fenceReplay, err := fixture.service.SubmitDeploymentFenceEvidence(
		ctx, Command{
			Actor: "operator", Effective: "operator",
			Action: ActionChangeMembership, ResourceKind: ResourceKindChannel,
			ResourceID: ChannelResource(genesis.RealmID, initial.ChannelID),
		},
		FenceEvidenceRequest{
			Version: ContractVersion, RealmID: genesis.RealmID,
			ChannelID: initial.ChannelID, OperationID: removed.OperationID,
			Evidence: evidence,
		},
	)
	require.NoError(t, err)
	require.Equal(t, fenced, fenceReplay)
	require.Len(t, fixture.store.state.Members, 1)
	require.Equal(t, principalA, fixture.store.state.Members[0].Principal)
	require.Equal(t, uint32(1), fixture.store.state.Channels[0].MemberCount)
	require.Len(t, fixture.store.state.Channels[0].CurrentGrants, 1)
	require.Len(t, fixture.store.state.Channels[0].PreviousGrants, 2)
	require.Equal(
		t, MemberStateRemoved,
		fixture.store.state.Rotations[len(fixture.store.state.Rotations)-1].
			MembershipChange.State,
	)

	rejoined, err := fixture.service.ChangeChannelMembership(
		ctx, membershipCommand(genesis.RealmID, initial.ChannelID),
		MembershipChangeRequest{
			Version: ContractVersion, RequestID: "membership-rejoin-b",
			RealmID: genesis.RealmID, ChannelID: initial.ChannelID,
			Change: MembershipChangeAdd, TargetPrincipal: principalB,
			RecipientAttestations: []identityapi.CapabilityDeliveryAttestation{
				attestationA, attestationB,
			},
			ValidFor: time.Hour, DrainFor: 15 * time.Minute,
		},
	)
	require.NoError(t, err)
	require.Equal(t, uint32(4), rejoined.PendingGeneration)
	installMembershipDeliveries(
		t, ctx, fixture, genesis.RealmID, rejoined,
		map[string]*identitycapability.Service{
			principalA: memberA, principalB: memberB,
		},
	)
	rejoinActivation := commitMembershipActivation(
		t, ctx, fixture, genesis.RealmID, rejoined,
	)
	for _, item := range []struct {
		principal string
		member    *identitycapability.Service
	}{
		{principalA, memberA},
		{principalB, memberB},
	} {
		active, activateErr := item.member.ActivateGeneration(rejoinActivation.Activation)
		require.NoError(t, activateErr)
		active, activateErr = item.member.ConfirmGenerationRuntimeAdoption(
			rejoinActivation.Activation,
		)
		require.NoError(t, activateErr)
		acknowledgeMembershipActive(
			t, ctx, fixture, genesis.RealmID, rejoined, active,
		)
	}
	require.Equal(t, uint32(4), memberB.GenerationReadiness(initial.ChannelID).CurrentGeneration)
	require.Zero(t, memberB.GenerationReadiness(initial.ChannelID).PreviousGeneration)
	require.Len(t, fixture.store.state.Members, 2)
	for _, audit := range fixture.store.state.AuditLog {
		require.Equal(t, "operator", audit.Actor)
		require.Equal(t, audit.Actor, audit.Effective)
		if audit.Action == ActionChangeMembership {
			require.NotEmpty(t, audit.TargetPrincipal)
		}
	}
}

func TestDeploymentFenceEvidenceRejectsMissingControlsStaleClockAndWrongBinding(t *testing.T) {
	now := time.Date(2026, 7, 28, 8, 0, 0, 0, time.UTC)
	realmID := "r1_00112233445566778899aabbccddeeff"
	operationID := "rao1_00112233445566778899aabbccddeeff"
	valid := validFenceEvidence(realmID, operationID, "p1_target", "operator", now)
	require.NoError(t, validateDeploymentFenceEvidence(
		valid, realmID, operationID, "operator", now,
	))

	missing := valid
	missing.Controls = missing.Controls[:2]
	require.ErrorIs(t, validateDeploymentFenceEvidence(
		missing, realmID, operationID, "operator", now,
	), ErrInvalidArgument)

	stale := valid
	stale.ObservedAt = now.Add(-MaxFenceEvidenceAge - time.Second)
	require.ErrorIs(t, validateDeploymentFenceEvidence(
		stale, realmID, operationID, "operator", now,
	), ErrInvalidArgument)

	skewed := valid
	skewed.ClockSkewSecond = MaxDeploymentClockSkew + 1
	require.ErrorIs(t, validateDeploymentFenceEvidence(
		skewed, realmID, operationID, "operator", now,
	), ErrInvalidArgument)

	require.ErrorIs(t, validateDeploymentFenceEvidence(
		valid, realmID, "rao1_ffeeddccbbaa99887766554433221100", "operator", now,
	), ErrInvalidArgument)
}

func TestMembershipCapacityReservesTargetFenceCompletion(t *testing.T) {
	below := Ledger{
		Operations: make([]OperationRecord, MaxOperations-1),
		Rotations:  make([]RotationRecord, MaxOperations-1),
		AuditLog:   make([]AuditRecord, MaxAuditRecords-(2*MaxMembersPerChannel+3)),
		AuditOutbox: make(
			[]AuditRecord, MaxAuditOutboxRecords-(2*MaxMembersPerChannel+3),
		),
	}
	require.NoError(t, membershipCapacity(below, MaxMembersPerChannel))
	below.AuditLog = append(below.AuditLog, AuditRecord{})
	require.ErrorIs(t, membershipCapacity(below, MaxMembersPerChannel), ErrResourceExhausted)
}

func TestProductPolicyDeniesMembershipBeforeGenerationMutation(t *testing.T) {
	ctx := context.Background()
	fixture := newServiceFixture(t)
	fixture.service.random = cryptorand.Reader
	genesis, err := fixture.service.CreateOrReopen(
		ctx, fixture.createCommand(),
		CreateRequest{
			Version: ContractVersion, RequestID: "membership-policy-genesis",
			RealmClass: RealmClassProduction,
		},
	)
	require.NoError(t, err)
	memberA, _, attestationA := newMembershipMember(t, fixture, 0x81)
	_, principalB, attestationB := newMembershipMember(t, fixture, 0x83)
	initial := issueAndInstallMembershipInitial(
		t, ctx, fixture, genesis.RealmID, memberA, attestationA,
	)
	denied := New(Config{
		Store: fixture.store, Signer: fixture.signer, Repository: fixture.repository,
		Random: cryptorand.Reader, Clock: fixture.clock, Policy: denyPolicy{},
	})
	_, err = denied.ChangeChannelMembership(
		ctx, membershipCommand(genesis.RealmID, initial.ChannelID),
		MembershipChangeRequest{
			Version: ContractVersion, RequestID: "membership-policy-add",
			RealmID: genesis.RealmID, ChannelID: initial.ChannelID,
			Change: MembershipChangeAdd, TargetPrincipal: principalB,
			RecipientAttestations: []identityapi.CapabilityDeliveryAttestation{
				attestationA, attestationB,
			},
			ValidFor: time.Hour, DrainFor: 15 * time.Minute,
		},
	)
	require.ErrorIs(t, err, ErrPermissionDenied)
	require.Empty(t, fixture.store.state.Rotations)
}

func TestMembershipFenceResumesAtBothCheckpointCrashBoundaries(t *testing.T) {
	for _, boundary := range []CrashBoundary{
		CrashAfterLedgerCommit,
		CrashAfterCheckpointCreate,
	} {
		t.Run(string(boundary), func(t *testing.T) {
			ctx := context.Background()
			fixture := newServiceFixture(t)
			fixture.service.random = cryptorand.Reader
			realmID, channelID, operationID, target := prepareRemovalAwaitingFence(
				t, ctx, fixture,
			)
			evidence := validFenceEvidence(
				realmID, operationID, target, "operator", fixture.clock(),
			)
			armed := true
			fixture.service = New(Config{
				Store: fixture.store, Signer: fixture.signer,
				Repository: fixture.repository, Random: cryptorand.Reader,
				Clock: fixture.clock, Policy: allowPolicy{},
				Crash: func(at CrashBoundary) error {
					if armed && at == boundary {
						armed = false
						return errors.New("injected membership fence crash")
					}
					return nil
				},
			})
			command := Command{
				Actor: "operator", Effective: "operator",
				Action: ActionChangeMembership, ResourceKind: ResourceKindChannel,
				ResourceID: ChannelResource(realmID, channelID),
			}
			request := FenceEvidenceRequest{
				Version: ContractVersion, RealmID: realmID, ChannelID: channelID,
				OperationID: operationID, Evidence: evidence,
			}
			_, err := fixture.service.SubmitDeploymentFenceEvidence(
				ctx, command, request,
			)
			require.ErrorIs(t, err, ErrUnavailable)
			require.Equal(t, PhaseCheckpointing, fixture.store.state.Phase)
			require.Len(
				t,
				fixture.store.state.Rotations[len(fixture.store.state.Rotations)-1].
					FenceEvidence,
				1,
			)

			restartMembershipAuthority(fixture)
			completed, err := fixture.service.SubmitDeploymentFenceEvidence(
				ctx, command, request,
			)
			require.NoError(t, err)
			require.Equal(t, DeliveryPhaseCompleted, completed.Phase)
			rotation := fixture.store.state.Rotations[len(fixture.store.state.Rotations)-1]
			require.Equal(t, operationID, rotation.OperationID)
			require.Len(t, rotation.FenceEvidence, 1)
		})
	}
}

func TestMembershipFenceCannotCompleteAfterOperationBoundsExpire(t *testing.T) {
	ctx := context.Background()
	fixture := newServiceFixture(t)
	fixture.service.random = cryptorand.Reader
	realmID, channelID, operationID, target := prepareRemovalAwaitingFence(
		t, ctx, fixture,
	)
	rotation := fixture.store.state.Rotations[len(fixture.store.state.Rotations)-1]
	expiredAt := rotation.DrainDeadline
	fixture.clock = func() time.Time { return expiredAt }
	fixture.service.clock = fixture.clock
	evidence := validFenceEvidence(
		realmID, operationID, target, "operator", fixture.clock(),
	)

	_, err := fixture.service.SubmitDeploymentFenceEvidence(
		ctx,
		Command{
			Actor: "operator", Effective: "operator",
			Action: ActionChangeMembership, ResourceKind: ResourceKindChannel,
			ResourceID: ChannelResource(realmID, channelID),
		},
		FenceEvidenceRequest{
			Version: ContractVersion, RealmID: realmID, ChannelID: channelID,
			OperationID: operationID, Evidence: evidence,
		},
	)
	require.ErrorIs(t, err, ErrInvalidArgument)
	require.Equal(
		t, DeliveryPhaseActivationCommitted,
		fixture.store.state.Rotations[len(fixture.store.state.Rotations)-1].Phase,
	)
	require.Empty(
		t,
		fixture.store.state.Rotations[len(fixture.store.state.Rotations)-1].
			FenceEvidence,
	)
}

func TestMembershipChangeResumesAtBothCheckpointCrashBoundaries(t *testing.T) {
	for _, boundary := range []CrashBoundary{
		CrashAfterLedgerCommit,
		CrashAfterCheckpointCreate,
	} {
		t.Run(string(boundary), func(t *testing.T) {
			ctx := context.Background()
			fixture := newServiceFixture(t)
			fixture.service.random = cryptorand.Reader
			genesis, err := fixture.service.CreateOrReopen(
				ctx, fixture.createCommand(),
				CreateRequest{
					Version: ContractVersion, RequestID: "membership-change-crash-genesis",
					RealmClass: RealmClassProduction,
				},
			)
			require.NoError(t, err)
			memberA, _, attestationA := newMembershipMember(t, fixture, 0xf1)
			_, principalB, attestationB := newMembershipMember(t, fixture, 0xf3)
			initial := issueAndInstallMembershipInitial(
				t, ctx, fixture, genesis.RealmID, memberA, attestationA,
			)
			request := MembershipChangeRequest{
				Version: ContractVersion, RequestID: "membership-change-crash-add",
				RealmID: genesis.RealmID, ChannelID: initial.ChannelID,
				Change: MembershipChangeAdd, TargetPrincipal: principalB,
				RecipientAttestations: []identityapi.CapabilityDeliveryAttestation{
					attestationA, attestationB,
				},
				ValidFor: time.Hour, DrainFor: 15 * time.Minute,
			}
			armed := true
			fixture.service = New(Config{
				Store: fixture.store, Signer: fixture.signer,
				Repository: fixture.repository, Random: cryptorand.Reader,
				Clock: fixture.clock, Policy: allowPolicy{},
				Crash: func(at CrashBoundary) error {
					if armed && at == boundary {
						armed = false
						return errors.New("injected membership change crash")
					}
					return nil
				},
			})
			_, err = fixture.service.ChangeChannelMembership(
				ctx, membershipCommand(genesis.RealmID, initial.ChannelID), request,
			)
			require.ErrorIs(t, err, ErrUnavailable)
			require.Equal(t, PhaseCheckpointing, fixture.store.state.Phase)
			require.Len(t, fixture.store.state.Rotations, 1)
			operationID := fixture.store.state.Rotations[0].OperationID
			deliveryIDs := append(
				[]string(nil), fixture.store.state.Rotations[0].DeliveryIDs...,
			)

			restartMembershipAuthority(fixture)
			replayed, err := fixture.service.ChangeChannelMembership(
				ctx, membershipCommand(genesis.RealmID, initial.ChannelID), request,
			)
			require.NoError(t, err)
			require.Equal(t, operationID, replayed.OperationID)
			require.Equal(t, deliveryIDs, fixture.store.state.Rotations[0].DeliveryIDs)
			require.Len(t, fixture.store.state.Rotations, 1)
			require.Equal(t, MemberStateCandidate, replayed.MembershipChange.State)
		})
	}
}

func prepareRemovalAwaitingFence(
	t *testing.T,
	ctx context.Context,
	fixture *serviceFixture,
) (string, [16]byte, string, string) {
	t.Helper()
	genesis, err := fixture.service.CreateOrReopen(
		ctx, fixture.createCommand(),
		CreateRequest{
			Version: ContractVersion, RequestID: "fence-crash-genesis",
			RealmClass: RealmClassProduction,
		},
	)
	require.NoError(t, err)
	memberA, principalA, attestationA := newMembershipMember(t, fixture, 0xd1)
	memberB, principalB, attestationB := newMembershipMember(t, fixture, 0xe1)
	initial := issueAndInstallMembershipInitial(
		t, ctx, fixture, genesis.RealmID, memberA, attestationA,
	)
	added, err := fixture.service.ChangeChannelMembership(
		ctx, membershipCommand(genesis.RealmID, initial.ChannelID),
		MembershipChangeRequest{
			Version: ContractVersion, RequestID: "fence-crash-add",
			RealmID: genesis.RealmID, ChannelID: initial.ChannelID,
			Change: MembershipChangeAdd, TargetPrincipal: principalB,
			RecipientAttestations: []identityapi.CapabilityDeliveryAttestation{
				attestationA, attestationB,
			},
			ValidFor: time.Hour, DrainFor: 15 * time.Minute,
		},
	)
	require.NoError(t, err)
	installMembershipDeliveries(
		t, ctx, fixture, genesis.RealmID, added,
		map[string]*identitycapability.Service{
			principalA: memberA, principalB: memberB,
		},
	)
	addActivation := commitMembershipActivation(
		t, ctx, fixture, genesis.RealmID, added,
	)
	for _, item := range []struct {
		principal string
		member    *identitycapability.Service
	}{
		{principalA, memberA},
		{principalB, memberB},
	} {
		active, activateErr := item.member.ActivateGeneration(addActivation.Activation)
		require.NoError(t, activateErr)
		active, activateErr = item.member.ConfirmGenerationRuntimeAdoption(addActivation.Activation)
		require.NoError(t, activateErr)
		acknowledgeMembershipActive(
			t, ctx, fixture, genesis.RealmID, added, active,
		)
	}
	removed, err := fixture.service.ChangeChannelMembership(
		ctx, membershipCommand(genesis.RealmID, initial.ChannelID),
		MembershipChangeRequest{
			Version: ContractVersion, RequestID: "fence-crash-remove",
			RealmID: genesis.RealmID, ChannelID: initial.ChannelID,
			Change: MembershipChangeRemove, TargetPrincipal: principalB,
			RecipientAttestations: []identityapi.CapabilityDeliveryAttestation{
				attestationA,
			},
			ValidFor: time.Hour, DrainFor: 15 * time.Minute,
		},
	)
	require.NoError(t, err)
	installMembershipDeliveries(
		t, ctx, fixture, genesis.RealmID, removed,
		map[string]*identitycapability.Service{principalA: memberA},
	)
	removeActivation := commitMembershipActivation(
		t, ctx, fixture, genesis.RealmID, removed,
	)
	active, err := memberA.ActivateGeneration(removeActivation.Activation)
	require.NoError(t, err)
	active, err = memberA.ConfirmGenerationRuntimeAdoption(removeActivation.Activation)
	require.NoError(t, err)
	pending := acknowledgeMembershipActive(
		t, ctx, fixture, genesis.RealmID, removed, active,
	)
	require.Equal(t, DeliveryPhaseActivationCommitted, pending.Phase)
	return genesis.RealmID, initial.ChannelID, removed.OperationID, principalB
}

func newMembershipMember(
	t *testing.T,
	fixture *serviceFixture,
	keyByte byte,
) (*identitycapability.Service, string, identityapi.CapabilityDeliveryAttestation) {
	t.Helper()
	private := newTestSigner(t, keyByte).private
	principal, err := identityprincipal.FromEd25519PublicKey(
		private.Public().(ed25519.PublicKey),
	)
	require.NoError(t, err)
	trust, err := identitytrust.NewRegistry([]identitytrust.Entry{{
		Principal: fixture.signer.principal,
		PublicKey: fixture.signer.private.Public().(ed25519.PublicKey),
		Purposes:  []identitytrust.Purpose{identitytrust.PurposeChannelIssue},
	}})
	require.NoError(t, err)
	member, err := identitycapability.NewService(
		filepath.Join(t.TempDir(), "member.db"), bytes.Repeat([]byte{keyByte + 1}, 32),
		principal.String(), trust, authorityCapabilityPolicy{}, fixture.clock,
	)
	require.NoError(t, err)
	attestation, err := member.AttestDeliveryPublicKey(
		private, fixture.clock().Add(time.Hour),
	)
	require.NoError(t, err)
	return member, principal.String(), attestation
}

func issueAndInstallMembershipInitial(
	t *testing.T,
	ctx context.Context,
	fixture *serviceFixture,
	realmID string,
	member *identitycapability.Service,
	attestation identityapi.CapabilityDeliveryAttestation,
) InitialGenerationResult {
	t.Helper()
	request := InitialGenerationRequest{
		Version: ContractVersion, RequestID: "membership-initial",
		RealmID: realmID, ChannelClass: identityapi.CapabilityRealmDiscovery,
		Permissions: identityapi.CapabilityPublish | identityapi.CapabilitySubscribe |
			identityapi.CapabilityStoreFetch,
		RecipientAttestation: attestation, ValidFor: time.Hour,
	}
	resource := InitialGenerationDeliveryResource(realmID, request.RequestID)
	issued, err := fixture.service.IssueInitialGeneration(
		ctx, Command{
			Actor: "operator", Effective: "operator", Action: ActionIssueDelivery,
			ResourceKind: ResourceKindGenerationDelivery, ResourceID: resource,
		}, request,
	)
	require.NoError(t, err)
	receipt, err := member.InstallGenerationDelivery(issued.Sealed)
	require.NoError(t, err)
	_, err = fixture.service.AcknowledgeInitialGeneration(
		ctx, Command{
			Actor: "operator", Effective: "operator", Action: ActionAcknowledgeDelivery,
			ResourceKind: ResourceKindGenerationDelivery, ResourceID: resource,
		},
		InitialGenerationAcknowledgeRequest{
			Version: ContractVersion, RealmID: realmID, Receipt: receipt,
		},
	)
	require.NoError(t, err)
	return issued
}

func membershipCommand(realmID string, channelID [16]byte) Command {
	return Command{
		Actor: "operator", Effective: "operator", Action: ActionChangeMembership,
		ResourceKind: ResourceKindChannel, ResourceID: ChannelResource(realmID, channelID),
	}
}

func installMembershipDeliveries(
	t *testing.T,
	ctx context.Context,
	fixture *serviceFixture,
	realmID string,
	rotation RotationResult,
	members map[string]*identitycapability.Service,
) map[string]identitycapability.GenerationDeliveryReceipt {
	t.Helper()
	receipts := make(map[string]identitycapability.GenerationDeliveryReceipt, len(members))
	for _, delivery := range rotation.Deliveries {
		member := members[delivery.RecipientPrincipal]
		require.NotNil(t, member)
		receipt, err := member.InstallGenerationDelivery(delivery.Sealed)
		require.NoError(t, err)
		resource, valid := GenerationDeliveryResource(
			realmID, rotation.OperationID, delivery.DeliveryID,
		)
		require.True(t, valid)
		_, err = fixture.service.AcknowledgeInitialGeneration(
			ctx, Command{
				Actor: "operator", Effective: "operator", Action: ActionAcknowledgeDelivery,
				ResourceKind: ResourceKindGenerationDelivery, ResourceID: resource,
			},
			InitialGenerationAcknowledgeRequest{
				Version: ContractVersion, RealmID: realmID, Receipt: receipt,
			},
		)
		require.NoError(t, err)
		receipts[delivery.RecipientPrincipal] = receipt
	}
	return receipts
}

func commitMembershipActivation(
	t *testing.T,
	ctx context.Context,
	fixture *serviceFixture,
	realmID string,
	rotation RotationResult,
) ActivationCommitResult {
	t.Helper()
	result, err := fixture.service.CommitChannelActivation(
		ctx, Command{
			Actor: "operator", Effective: "operator", Action: ActionCommitActivation,
			ResourceKind: ResourceKindOperation,
			ResourceID:   OperationResource(realmID, rotation.OperationID),
		},
		ActivationCommitRequest{
			Version: ContractVersion, RealmID: realmID, OperationID: rotation.OperationID,
		},
	)
	require.NoError(t, err)
	return result
}

func acknowledgeMembershipActive(
	t *testing.T,
	ctx context.Context,
	fixture *serviceFixture,
	realmID string,
	rotation RotationResult,
	receipt identitycapability.GenerationDeliveryReceipt,
) ActivationAcknowledgeResult {
	t.Helper()
	resource, valid := GenerationDeliveryResource(
		realmID, rotation.OperationID, receipt.DeliveryID,
	)
	require.True(t, valid)
	result, err := fixture.service.AcknowledgeChannelActivation(
		ctx, Command{
			Actor: "operator", Effective: "operator", Action: ActionAcknowledgeActivation,
			ResourceKind: ResourceKindGenerationDelivery, ResourceID: resource,
		},
		ActivationAcknowledgeRequest{
			Version: ContractVersion, RealmID: realmID,
			OperationID: rotation.OperationID, ApprovedHost: true, Receipt: receipt,
		},
	)
	require.NoError(t, err)
	return result
}

func validFenceEvidence(
	realmID, operationID, target, actor string,
	observedAt time.Time,
) DeploymentFenceEvidence {
	digest := "sha256:" + string(bytes.Repeat([]byte{'a'}, 64))
	return DeploymentFenceEvidence{
		Version: ContractVersion, RealmID: realmID, OperationID: operationID,
		TargetPrincipal: target, ManifestDigest: digest,
		RequestID: "fence-removed-member", Reason: "membership_removed",
		ObservedAt: observedAt.UTC().Truncate(time.Second),
		Controls: []DeploymentFenceControl{
			{Kind: "target_ingress_blocked", Actor: actor, ReceiptDigest: digest},
			{Kind: "discovery_withdrawn", Actor: actor, ReceiptDigest: digest},
			{Kind: "peer_id_denied", Actor: actor, ReceiptDigest: digest},
		},
	}
}

func restartMembershipAuthority(fixture *serviceFixture) {
	fixture.service = New(Config{
		Store: fixture.store, Signer: fixture.signer, Repository: fixture.repository,
		Random: cryptorand.Reader, Clock: fixture.clock, Policy: allowPolicy{},
	})
}
