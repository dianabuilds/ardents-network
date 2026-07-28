package authority

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"errors"
	"path/filepath"
	"testing"
	"time"

	identityapi "ardents/internal/identity"
	identitycapability "ardents/internal/identity/capability"
	identityprincipal "ardents/internal/identity/principal"
	identitytrust "ardents/internal/identity/trust"
	"ardents/internal/storage"

	"github.com/stretchr/testify/require"
)

func TestAuthorityIssuesAndAcknowledgesOneInitialGeneration(t *testing.T) {
	ctx := context.Background()
	fixture := newServiceFixture(t)
	genesis, err := fixture.service.CreateOrReopen(ctx, fixture.createCommand(), CreateRequest{
		Version: ContractVersion, RequestID: "genesis-001", RealmClass: RealmClassProduction,
	})
	require.NoError(t, err)
	memberPrivate := newTestSigner(t, 0x62).private
	memberPrincipal, err := identityprincipal.FromEd25519PublicKey(memberPrivate.Public().(ed25519.PublicKey))
	require.NoError(t, err)
	trust, err := identitytrust.NewRegistry([]identitytrust.Entry{{
		Principal: fixture.signer.principal,
		PublicKey: fixture.signer.private.Public().(ed25519.PublicKey),
		Purposes:  []identitytrust.Purpose{identitytrust.PurposeChannelIssue},
	}})
	require.NoError(t, err)
	member, err := identitycapability.NewService(
		filepath.Join(t.TempDir(), "member-capabilities.db"),
		bytes.Repeat([]byte{0x63}, 32), memberPrincipal.String(), trust,
		authorityCapabilityPolicy{}, fixture.clock,
	)
	require.NoError(t, err)
	attestation, err := member.AttestDeliveryPublicKey(
		memberPrivate, fixture.clock().Add(time.Hour),
	)
	require.NoError(t, err)
	request := InitialGenerationRequest{
		Version: ContractVersion, RequestID: "delivery-001", RealmID: genesis.RealmID,
		ChannelClass: identityapi.CapabilityRealmDiscovery,
		Permissions: identityapi.CapabilityPublish | identityapi.CapabilitySubscribe |
			identityapi.CapabilityStoreFetch,
		RecipientAttestation: attestation, ValidFor: time.Hour,
	}
	resource := InitialGenerationDeliveryResource(request.RealmID, request.RequestID)
	command := Command{
		Actor: "operator", Effective: "operator", Action: ActionIssueDelivery,
		ResourceKind: ResourceKindGenerationDelivery, ResourceID: resource,
	}
	issued, err := fixture.service.IssueInitialGeneration(ctx, command, request)
	require.NoError(t, err)
	require.Equal(t, uint64(2), issued.AuthoritySequence)
	require.Equal(t, uint32(1), issued.Generation)
	require.NotEmpty(t, issued.Sealed.Envelope)

	replayed, err := fixture.service.IssueInitialGeneration(ctx, command, request)
	require.NoError(t, err)
	require.Equal(t, issued, replayed)
	conflicting := request
	conflicting.Permissions = identityapi.CapabilitySubscribe
	_, err = fixture.service.IssueInitialGeneration(ctx, command, conflicting)
	require.ErrorIs(t, err, ErrConflict)

	retained := fixture.store.state.InitialGenerationDeliveries[0]
	forgedReceipt := identitycapability.GenerationDeliveryReceipt{
		Version: ContractVersion, RealmID: issued.Sealed.Binding.RealmID,
		AuthorityPrincipal: issued.Sealed.Binding.AuthorityPrincipal,
		AuthorityEpoch:     issued.Sealed.Binding.AuthorityEpoch,
		AuthoritySequence:  issued.Sealed.Binding.AuthoritySequence,
		OperationID:        issued.Sealed.Binding.OperationID, DeliveryID: issued.Sealed.Binding.DeliveryID,
		EnvelopeDigest: issued.Sealed.EnvelopeDigest, ChannelID: issued.Sealed.Binding.ChannelID,
		ChannelClass: issued.Sealed.Binding.ChannelClass, Generation: issued.Sealed.Binding.Generation,
		RecipientPrincipal: issued.Sealed.Binding.RecipientPrincipal,
		DeliveryKeyDigest:  issued.Sealed.Binding.DeliveryKeyDigest,
		Phase:              identitycapability.DeliveryPhaseInstalled, CreatedAt: fixture.clock(),
	}
	forgedReceipt, err = identitycapability.AuthenticateGenerationDeliveryReceipt(
		forgedReceipt, retained.ReceiptKey,
	)
	require.NoError(t, err)
	acknowledge := Command{
		Actor: "operator", Effective: "operator", Action: ActionAcknowledgeDelivery,
		ResourceKind: ResourceKindGenerationDelivery, ResourceID: resource,
	}
	wrongGeneration := forgedReceipt
	wrongGeneration.Generation++
	wrongGeneration, err = identitycapability.AuthenticateGenerationDeliveryReceipt(
		wrongGeneration, retained.ReceiptKey,
	)
	require.NoError(t, err)
	_, err = fixture.service.AcknowledgeInitialGeneration(
		ctx, acknowledge, InitialGenerationAcknowledgeRequest{
			Version: ContractVersion, RealmID: genesis.RealmID, Receipt: wrongGeneration,
		},
	)
	require.ErrorIs(t, err, ErrInvalidArgument)
	futureReceipt := forgedReceipt
	futureReceipt.CreatedAt = fixture.clock().Add(time.Second)
	futureReceipt, err = identitycapability.AuthenticateGenerationDeliveryReceipt(
		futureReceipt, retained.ReceiptKey,
	)
	require.NoError(t, err)
	_, err = fixture.service.AcknowledgeInitialGeneration(
		ctx, acknowledge, InitialGenerationAcknowledgeRequest{
			Version: ContractVersion, RealmID: genesis.RealmID, Receipt: futureReceipt,
		},
	)
	require.ErrorIs(t, err, ErrInvalidArgument)
	fixture.service.clock = func() time.Time { return fixture.clock().Add(2 * time.Hour) }
	_, err = fixture.service.AcknowledgeInitialGeneration(
		ctx, acknowledge, InitialGenerationAcknowledgeRequest{
			Version: ContractVersion, RealmID: genesis.RealmID, Receipt: forgedReceipt,
		},
	)
	require.ErrorIs(t, err, ErrInvalidArgument)
	fixture.service.clock = fixture.clock
	acknowledged, err := fixture.service.AcknowledgeInitialGeneration(
		ctx, acknowledge, InitialGenerationAcknowledgeRequest{
			Version: ContractVersion, RealmID: genesis.RealmID, Receipt: forgedReceipt,
		},
	)
	require.NoError(t, err)
	receipt, err := member.InstallGenerationDelivery(issued.Sealed)
	require.NoError(t, err)
	require.Equal(t, forgedReceipt, receipt,
		"a receipt-key holder can forge the installed assertion before member persistence")
	replayedAcknowledgement, err := fixture.service.AcknowledgeInitialGeneration(
		ctx, acknowledge, InitialGenerationAcknowledgeRequest{
			Version: ContractVersion, RealmID: genesis.RealmID, Receipt: receipt,
		},
	)
	require.NoError(t, err)
	require.Equal(t, acknowledged, replayedAcknowledgement)
	require.Equal(t, DeliveryPhaseInstalled, acknowledged.Phase)
	require.Equal(t, uint64(3), acknowledged.AuthoritySequence)
	require.Equal(t, uint32(1), fixture.service.Readiness().CurrentGeneration)
	require.Equal(t, uint32(1), fixture.service.Readiness().ChannelCount)
	require.Equal(t, uint32(1), fixture.service.Readiness().MemberCount)
}

func TestInitialGenerationRealStoreCrashReconciliationPreservesIdentities(t *testing.T) {
	for _, stage := range []string{"issue", "acknowledge"} {
		for _, boundary := range []CrashBoundary{CrashAfterLedgerCommit, CrashAfterCheckpointCreate} {
			t.Run(stage+"/"+string(boundary), func(t *testing.T) {
				ctx := context.Background()
				root := t.TempDir()
				key := bytes.Repeat([]byte{0x71}, AuthorityStoreKeyBytes)
				storePath := filepath.Join(root, "authority.db")
				repositoryPath := filepath.Join(root, "checkpoints")
				require.NoError(t, storage.EnsurePrivateDir(root))
				require.NoError(t, storage.EnsurePrivateDir(repositoryPath))
				store, err := OpenFileStore(ctx, storePath, key)
				require.NoError(t, err)
				t.Cleanup(func() { _ = store.Close() })
				repository, err := NewFileCheckpointRepository(repositoryPath)
				require.NoError(t, err)
				signer := newTestSigner(t, 0x72)
				clock := func() time.Time {
					return time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
				}
				newService := func(store *FileStore) *Service {
					return New(Config{
						Store: store, Signer: signer, Repository: repository,
						Random: bytes.NewReader(bytes.Repeat([]byte{0x73}, 4096)),
						Clock:  clock, Policy: allowPolicy{},
					})
				}
				service := newService(store)
				genesis, err := service.CreateOrReopen(ctx, Command{
					Actor: "operator", Effective: "operator", Action: ActionCreate,
					ResourceKind: ResourceKindAuthorityInstance, ResourceID: PrimaryAuthorityInstance,
				}, CreateRequest{
					Version: ContractVersion, RequestID: "genesis-real-store",
					RealmClass: RealmClassProduction,
				})
				require.NoError(t, err)

				memberPrivate := newTestSigner(t, 0x74).private
				memberPrincipal, err := identityprincipal.FromEd25519PublicKey(
					memberPrivate.Public().(ed25519.PublicKey),
				)
				require.NoError(t, err)
				trust, err := identitytrust.NewRegistry([]identitytrust.Entry{{
					Principal: signer.principal,
					PublicKey: signer.private.Public().(ed25519.PublicKey),
					Purposes:  []identitytrust.Purpose{identitytrust.PurposeChannelIssue},
				}})
				require.NoError(t, err)
				member, err := identitycapability.NewService(
					filepath.Join(root, "member.db"), bytes.Repeat([]byte{0x75}, 32),
					memberPrincipal.String(), trust, authorityCapabilityPolicy{}, clock,
				)
				require.NoError(t, err)
				attestation, err := member.AttestDeliveryPublicKey(memberPrivate, clock().Add(time.Hour))
				require.NoError(t, err)
				request := InitialGenerationRequest{
					Version: ContractVersion, RequestID: "delivery-real-store",
					RealmID: genesis.RealmID, ChannelClass: identityapi.CapabilityRealmDiscovery,
					Permissions:          identityapi.CapabilityPublish | identityapi.CapabilitySubscribe,
					RecipientAttestation: attestation, ValidFor: time.Hour,
				}
				resource := InitialGenerationDeliveryResource(request.RealmID, request.RequestID)
				issueCommand := Command{
					Actor: "operator", Effective: "operator", Action: ActionIssueDelivery,
					ResourceKind: ResourceKindGenerationDelivery, ResourceID: resource,
				}

				if stage == "issue" {
					service.crash = crashAt(boundary)
				}
				issued, err := service.IssueInitialGeneration(ctx, issueCommand, request)
				if stage == "issue" {
					require.ErrorIs(t, err, ErrUnavailable)
					persisted, found, loadErr := store.Load(ctx)
					require.NoError(t, loadErr)
					require.True(t, found)
					require.Len(t, persisted.InitialGenerationDeliveries, 1)
					crashedDelivery := persisted.InitialGenerationDeliveries[0]
					require.NoError(t, store.Close())
					store, err = OpenFileStore(ctx, storePath, key)
					require.NoError(t, err)
					service = newService(store)
					issued, err = service.IssueInitialGeneration(ctx, issueCommand, request)
					require.Equal(t, crashedDelivery.DeliveryID, issued.DeliveryID)
					require.Equal(t, crashedDelivery.Sealed, issued.Sealed)
				}
				require.NoError(t, err)
				require.Equal(t, InitialGenerationDeliveryResource(request.RealmID, request.RequestID), resource)
				receipt, err := member.InstallGenerationDelivery(issued.Sealed)
				require.NoError(t, err)
				ackCommand := Command{
					Actor: "operator", Effective: "operator", Action: ActionAcknowledgeDelivery,
					ResourceKind: ResourceKindGenerationDelivery, ResourceID: resource,
				}
				if stage == "acknowledge" {
					service.crash = crashAt(boundary)
				}
				acknowledged, err := service.AcknowledgeInitialGeneration(
					ctx, ackCommand, InitialGenerationAcknowledgeRequest{
						Version: ContractVersion, RealmID: genesis.RealmID, Receipt: receipt,
					},
				)
				if stage == "acknowledge" {
					require.ErrorIs(t, err, ErrUnavailable)
					require.NoError(t, store.Close())
					store, err = OpenFileStore(ctx, storePath, key)
					require.NoError(t, err)
					service = newService(store)
					acknowledged, err = service.AcknowledgeInitialGeneration(
						ctx, ackCommand, InitialGenerationAcknowledgeRequest{
							Version: ContractVersion, RealmID: genesis.RealmID, Receipt: receipt,
						},
					)
				}
				require.NoError(t, err)
				require.Equal(t, uint64(3), acknowledged.AuthoritySequence)
				require.Equal(t, DeliveryPhaseInstalled, acknowledged.Phase)
				require.NoError(t, store.Close())
			})
		}
	}
}

func crashAt(target CrashBoundary) func(CrashBoundary) error {
	return func(boundary CrashBoundary) error {
		if boundary == target {
			return errors.New("injected crash")
		}
		return nil
	}
}

type authorityCapabilityPolicy struct{}

func (authorityCapabilityPolicy) AllowCapabilityUse(identityapi.CapabilityUse) error {
	return nil
}
