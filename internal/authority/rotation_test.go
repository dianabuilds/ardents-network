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

func TestAuthorityRotatesAndCompletesOneApprovedMemberGeneration(t *testing.T) {
	ctx := context.Background()
	fixture := newServiceFixture(t)
	genesis, err := fixture.service.CreateOrReopen(
		ctx, fixture.createCommand(),
		CreateRequest{Version: ContractVersion, RequestID: "rotation-genesis", RealmClass: RealmClassProduction},
	)
	require.NoError(t, err)
	memberPrivate := newTestSigner(t, 0xd1).private
	memberPrincipal, err := identityprincipal.FromEd25519PublicKey(
		memberPrivate.Public().(ed25519.PublicKey),
	)
	require.NoError(t, err)
	trust, err := identitytrust.NewRegistry([]identitytrust.Entry{{
		Principal: fixture.signer.principal,
		PublicKey: fixture.signer.private.Public().(ed25519.PublicKey),
		Purposes:  []identitytrust.Purpose{identitytrust.PurposeChannelIssue},
	}})
	require.NoError(t, err)
	member, err := identitycapability.NewService(
		filepath.Join(t.TempDir(), "member.db"), bytes.Repeat([]byte{0xd2}, 32),
		memberPrincipal.String(), trust, authorityCapabilityPolicy{}, fixture.clock,
	)
	require.NoError(t, err)
	attestation, err := member.AttestDeliveryPublicKey(memberPrivate, fixture.clock().Add(time.Hour))
	require.NoError(t, err)
	initialRequest := InitialGenerationRequest{
		Version: ContractVersion, RequestID: "initial-generation", RealmID: genesis.RealmID,
		ChannelClass: identityapi.CapabilityRealmDiscovery,
		Permissions: identityapi.CapabilityPublish | identityapi.CapabilitySubscribe |
			identityapi.CapabilityStoreFetch,
		RecipientAttestation: attestation, ValidFor: time.Hour,
	}
	initialResource := InitialGenerationDeliveryResource(genesis.RealmID, initialRequest.RequestID)
	issued, err := fixture.service.IssueInitialGeneration(ctx, Command{
		Actor: "operator", Effective: "operator", Action: ActionIssueDelivery,
		ResourceKind: ResourceKindGenerationDelivery, ResourceID: initialResource,
	}, initialRequest)
	require.NoError(t, err)
	installed, err := member.InstallGenerationDelivery(issued.Sealed)
	require.NoError(t, err)
	_, err = fixture.service.AcknowledgeInitialGeneration(ctx, Command{
		Actor: "operator", Effective: "operator", Action: ActionAcknowledgeDelivery,
		ResourceKind: ResourceKindGenerationDelivery, ResourceID: initialResource,
	}, InitialGenerationAcknowledgeRequest{
		Version: ContractVersion, RealmID: genesis.RealmID, Receipt: installed,
	})
	require.NoError(t, err)
	rotationRandom := append([]byte(nil), bytes.Repeat([]byte{0xd3}, 32)...)
	rotationRandom = append(rotationRandom, bytes.Repeat([]byte{0xd4}, 16)...)
	rotationRandom = append(rotationRandom, bytes.Repeat([]byte{0xd5}, 32)...)
	fixture.service.random = bytes.NewReader(rotationRandom)

	rotationRequest := RotationRequest{
		Version: ContractVersion, RequestID: "rotation-001", RealmID: genesis.RealmID,
		ChannelID: issued.ChannelID, RecipientAttestations: []identityapi.CapabilityDeliveryAttestation{attestation},
		ValidFor: time.Hour, DrainFor: 15 * time.Minute,
	}
	rotationResource := ChannelResource(genesis.RealmID, issued.ChannelID)
	rotated, err := fixture.service.RotateChannel(ctx, Command{
		Actor: "operator", Effective: "operator", Action: ActionRotateGeneration,
		ResourceKind: ResourceKindChannel, ResourceID: rotationResource,
	}, rotationRequest)
	require.NoError(t, err)
	require.Equal(t, uint32(1), rotated.PreviousGeneration)
	require.Equal(t, uint32(2), rotated.PendingGeneration)
	require.Len(t, rotated.Deliveries, 1)
	replayed, err := fixture.service.RotateChannel(ctx, Command{
		Actor: "operator", Effective: "operator", Action: ActionRotateGeneration,
		ResourceKind: ResourceKindChannel, ResourceID: rotationResource,
	}, rotationRequest)
	require.NoError(t, err)
	require.Equal(t, rotated, replayed)
	conflicting := rotationRequest
	conflicting.RequestID = "rotation-002"
	_, err = fixture.service.RotateChannel(ctx, Command{
		Actor: "operator", Effective: "operator", Action: ActionRotateGeneration,
		ResourceKind: ResourceKindChannel, ResourceID: rotationResource,
	}, conflicting)
	require.ErrorIs(t, err, ErrConflict)

	pendingReceipt, err := member.InstallGenerationDelivery(rotated.Deliveries[0].Sealed)
	require.NoError(t, err)
	deliveryResource, valid := GenerationDeliveryResource(
		genesis.RealmID, rotated.OperationID, rotated.Deliveries[0].DeliveryID,
	)
	require.True(t, valid)
	_, err = fixture.service.AcknowledgeInitialGeneration(ctx, Command{
		Actor: "operator", Effective: "operator", Action: ActionAcknowledgeDelivery,
		ResourceKind: ResourceKindGenerationDelivery, ResourceID: deliveryResource,
	}, InitialGenerationAcknowledgeRequest{
		Version: ContractVersion, RealmID: genesis.RealmID, Receipt: pendingReceipt,
	})
	require.NoError(t, err)
	committed, err := fixture.service.CommitChannelActivation(ctx, Command{
		Actor: "operator", Effective: "operator", Action: ActionCommitActivation,
		ResourceKind: ResourceKindOperation, ResourceID: OperationResource(genesis.RealmID, rotated.OperationID),
	}, ActivationCommitRequest{
		Version: ContractVersion, RealmID: genesis.RealmID, OperationID: rotated.OperationID,
	})
	require.NoError(t, err)
	require.Equal(t, uint32(2), committed.Activation.Generation)
	active, err := member.ActivateGeneration(committed.Activation)
	require.NoError(t, err)
	_, err = fixture.service.AcknowledgeChannelActivation(ctx, Command{
		Actor: "operator", Effective: "operator", Action: ActionAcknowledgeActivation,
		ResourceKind: ResourceKindGenerationDelivery, ResourceID: deliveryResource,
	}, ActivationAcknowledgeRequest{
		Version: ContractVersion, RealmID: genesis.RealmID,
		OperationID: rotated.OperationID, ApprovedHost: false, Receipt: active,
	})
	require.ErrorIs(t, err, ErrPermissionDenied)
	completed, err := fixture.service.AcknowledgeChannelActivation(ctx, Command{
		Actor: "operator", Effective: "operator", Action: ActionAcknowledgeActivation,
		ResourceKind: ResourceKindGenerationDelivery, ResourceID: deliveryResource,
	}, ActivationAcknowledgeRequest{
		Version: ContractVersion, RealmID: genesis.RealmID,
		OperationID: rotated.OperationID, ApprovedHost: true, Receipt: active,
	})
	require.NoError(t, err)
	require.Equal(t, uint32(2), completed.CurrentGeneration)
	require.Equal(t, uint32(1), completed.PreviousGeneration)
	require.Equal(t, DeliveryPhaseCompleted, completed.Phase)
	require.Equal(t, uint32(2), fixture.service.Readiness().CurrentGeneration)
	replayedCompletion, err := fixture.service.AcknowledgeChannelActivation(ctx, Command{
		Actor: "operator", Effective: "operator", Action: ActionAcknowledgeActivation,
		ResourceKind: ResourceKindGenerationDelivery, ResourceID: deliveryResource,
	}, ActivationAcknowledgeRequest{
		Version: ContractVersion, RealmID: genesis.RealmID,
		OperationID: rotated.OperationID, ApprovedHost: true, Receipt: active,
	})
	require.NoError(t, err)
	require.Equal(t, completed, replayedCompletion)
}

func TestRotationResumesEveryAuthorityTransitionAfterLedgerOrCheckpointCrash(t *testing.T) {
	for _, stage := range []string{"rotate", "installed", "activate", "active"} {
		for _, boundary := range []CrashBoundary{
			CrashAfterLedgerCommit,
			CrashAfterCheckpointCreate,
		} {
			t.Run(stage+"/"+string(boundary), func(t *testing.T) {
				runRotationCrashCase(t, stage, boundary)
			})
		}
	}
}

func TestProductPolicyDeniesChannelRotationBeforeSecretGeneration(t *testing.T) {
	ctx := context.Background()
	fixture := newServiceFixture(t)
	genesis, err := fixture.service.CreateOrReopen(
		ctx, fixture.createCommand(),
		CreateRequest{Version: ContractVersion, RequestID: "rotation-policy-genesis", RealmClass: RealmClassProduction},
	)
	require.NoError(t, err)
	denied := New(Config{
		Store: fixture.store, Signer: fixture.signer, Repository: fixture.repository,
		Random: cryptorand.Reader, Clock: fixture.clock, Policy: denyPolicy{},
	})
	channelID := [16]byte{1}
	_, err = denied.RotateChannel(ctx, Command{
		Actor: "operator", Effective: "operator", Action: ActionRotateGeneration,
		ResourceKind: ResourceKindChannel, ResourceID: ChannelResource(genesis.RealmID, channelID),
	}, RotationRequest{
		Version: ContractVersion, RequestID: "rotation-policy-denied", RealmID: genesis.RealmID,
		ChannelID: channelID,
		RecipientAttestations: []identityapi.CapabilityDeliveryAttestation{{Version: 1}},
		ValidFor: time.Hour, DrainFor: time.Minute,
	})
	require.ErrorIs(t, err, ErrPermissionDenied)
	require.Len(t, fixture.store.state.AuditLog, 1)
	require.Empty(t, fixture.store.state.Rotations)
}

func runRotationCrashCase(t *testing.T, crashStage string, boundary CrashBoundary) {
	t.Helper()
	ctx := context.Background()
	fixture := newServiceFixture(t)
	armed := false
	fixture.service = New(Config{
		Store: fixture.store, Signer: fixture.signer, Repository: fixture.repository,
		Random: cryptorand.Reader, Clock: fixture.clock, Policy: allowPolicy{},
		Crash: func(at CrashBoundary) error {
			if armed && at == boundary {
				armed = false
				return errors.New("injected rotation crash")
			}
			return nil
		},
	})
	restart := func() {
		fixture.service = New(Config{
			Store: fixture.store, Signer: fixture.signer, Repository: fixture.repository,
			Random: cryptorand.Reader, Clock: fixture.clock, Policy: allowPolicy{},
		})
	}
	invoke := func(stage string, call func() error) {
		t.Helper()
		if crashStage != stage {
			require.NoError(t, call())
			return
		}
		armed = true
		require.ErrorIs(t, call(), ErrUnavailable)
		require.Equal(t, PhaseCheckpointing, fixture.store.state.Phase)
		restart()
		require.NoError(t, call())
	}

	genesis, err := fixture.service.CreateOrReopen(
		ctx, fixture.createCommand(),
		CreateRequest{Version: ContractVersion, RequestID: "rotation-crash-genesis", RealmClass: RealmClassProduction},
	)
	require.NoError(t, err)
	memberPrivate := newTestSigner(t, 0xe1).private
	memberPrincipal, err := identityprincipal.FromEd25519PublicKey(
		memberPrivate.Public().(ed25519.PublicKey),
	)
	require.NoError(t, err)
	trust, err := identitytrust.NewRegistry([]identitytrust.Entry{{
		Principal: fixture.signer.principal,
		PublicKey: fixture.signer.private.Public().(ed25519.PublicKey),
		Purposes:  []identitytrust.Purpose{identitytrust.PurposeChannelIssue},
	}})
	require.NoError(t, err)
	member, err := identitycapability.NewService(
		filepath.Join(t.TempDir(), "member.db"), bytes.Repeat([]byte{0xe2}, 32),
		memberPrincipal.String(), trust, authorityCapabilityPolicy{}, fixture.clock,
	)
	require.NoError(t, err)
	attestation, err := member.AttestDeliveryPublicKey(memberPrivate, fixture.clock().Add(time.Hour))
	require.NoError(t, err)
	initialRequest := InitialGenerationRequest{
		Version: ContractVersion, RequestID: "rotation-crash-initial", RealmID: genesis.RealmID,
		ChannelClass: identityapi.CapabilityRealmDiscovery,
		Permissions: identityapi.CapabilityPublish | identityapi.CapabilitySubscribe |
			identityapi.CapabilityStoreFetch,
		RecipientAttestation: attestation, ValidFor: time.Hour,
	}
	initialResource := InitialGenerationDeliveryResource(genesis.RealmID, initialRequest.RequestID)
	issued, err := fixture.service.IssueInitialGeneration(ctx, Command{
		Actor: "operator", Effective: "operator", Action: ActionIssueDelivery,
		ResourceKind: ResourceKindGenerationDelivery, ResourceID: initialResource,
	}, initialRequest)
	require.NoError(t, err)
	installed, err := member.InstallGenerationDelivery(issued.Sealed)
	require.NoError(t, err)
	_, err = fixture.service.AcknowledgeInitialGeneration(ctx, Command{
		Actor: "operator", Effective: "operator", Action: ActionAcknowledgeDelivery,
		ResourceKind: ResourceKindGenerationDelivery, ResourceID: initialResource,
	}, InitialGenerationAcknowledgeRequest{
		Version: ContractVersion, RealmID: genesis.RealmID, Receipt: installed,
	})
	require.NoError(t, err)

	rotationRequest := RotationRequest{
		Version: ContractVersion, RequestID: "rotation-crash-001", RealmID: genesis.RealmID,
		ChannelID:             issued.ChannelID,
		RecipientAttestations: []identityapi.CapabilityDeliveryAttestation{attestation},
		ValidFor:              time.Hour, DrainFor: 15 * time.Minute,
	}
	rotationCommand := Command{
		Actor: "operator", Effective: "operator", Action: ActionRotateGeneration,
		ResourceKind: ResourceKindChannel, ResourceID: ChannelResource(genesis.RealmID, issued.ChannelID),
	}
	var rotated RotationResult
	invoke("rotate", func() error {
		var callErr error
		rotated, callErr = fixture.service.RotateChannel(ctx, rotationCommand, rotationRequest)
		return callErr
	})
	require.Len(t, rotated.Deliveries, 1)
	pending, err := member.InstallGenerationDelivery(rotated.Deliveries[0].Sealed)
	require.NoError(t, err)
	deliveryResource, valid := GenerationDeliveryResource(
		genesis.RealmID, rotated.OperationID, rotated.Deliveries[0].DeliveryID,
	)
	require.True(t, valid)
	installedCommand := Command{
		Actor: "operator", Effective: "operator", Action: ActionAcknowledgeDelivery,
		ResourceKind: ResourceKindGenerationDelivery, ResourceID: deliveryResource,
	}
	invoke("installed", func() error {
		_, callErr := fixture.service.AcknowledgeInitialGeneration(
			ctx, installedCommand, InitialGenerationAcknowledgeRequest{
				Version: ContractVersion, RealmID: genesis.RealmID, Receipt: pending,
			},
		)
		return callErr
	})

	commitCommand := Command{
		Actor: "operator", Effective: "operator", Action: ActionCommitActivation,
		ResourceKind: ResourceKindOperation,
		ResourceID:   OperationResource(genesis.RealmID, rotated.OperationID),
	}
	var committed ActivationCommitResult
	invoke("activate", func() error {
		var callErr error
		committed, callErr = fixture.service.CommitChannelActivation(
			ctx, commitCommand, ActivationCommitRequest{
				Version: ContractVersion, RealmID: genesis.RealmID, OperationID: rotated.OperationID,
			},
		)
		return callErr
	})
	active, err := member.ActivateGeneration(committed.Activation)
	require.NoError(t, err)
	activeCommand := Command{
		Actor: "operator", Effective: "operator", Action: ActionAcknowledgeActivation,
		ResourceKind: ResourceKindGenerationDelivery, ResourceID: deliveryResource,
	}
	var completed ActivationAcknowledgeResult
	invoke("active", func() error {
		var callErr error
		completed, callErr = fixture.service.AcknowledgeChannelActivation(
			ctx, activeCommand, ActivationAcknowledgeRequest{
				Version: ContractVersion, RealmID: genesis.RealmID,
				OperationID: rotated.OperationID, ApprovedHost: true, Receipt: active,
			},
		)
		return callErr
	})
	require.Equal(t, DeliveryPhaseCompleted, completed.Phase)
	require.Len(t, fixture.store.state.Rotations, 1)
	require.Equal(t, rotated.OperationID, fixture.store.state.Rotations[0].OperationID)
	require.Equal(t, uint32(2), fixture.store.state.Channels[0].CurrentGeneration)
	require.Zero(t, fixture.store.state.Channels[0].PendingGenerationCount)
}
