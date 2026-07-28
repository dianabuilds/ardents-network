//go:build integration

package localapi_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	cryptorand "crypto/rand"
	"path/filepath"
	"strings"
	"testing"
	"time"

	domain "ardents/internal/authority"
	"ardents/internal/channeldelivery"
	runtimeinfra "ardents/internal/daemon"
	identityapi "ardents/internal/identity"
	identityaccess "ardents/internal/identity/access"
	identitycapability "ardents/internal/identity/capability"
	identityprincipal "ardents/internal/identity/principal"
	identitytrust "ardents/internal/identity/trust"
	protocol "ardents/internal/localapi/protocol"
	"ardents/internal/storage"
	"ardents/tests/testkit"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestChannelMembershipAddRemoveAndFenceAcrossThreeProtectedHosts(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	now := time.Date(2026, 7, 28, 9, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }

	storePath := filepath.Join(root, "authority", "realm-authority.db")
	checkpointPath := filepath.Join(root, "checkpoints")
	require.NoError(t, storage.EnsurePrivateDir(filepath.Dir(storePath)))
	require.NoError(t, storage.EnsurePrivateDir(checkpointPath))
	store, err := domain.OpenFileStore(
		ctx, storePath, bytes.Repeat([]byte{0xc1}, domain.AuthorityStoreKeyBytes),
	)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	repository, err := domain.NewFileCheckpointRepository(checkpointPath)
	require.NoError(t, err)
	authoritySigner := integrationAuthoritySigner(t, 0xc2)
	authorityService := domain.New(domain.Config{
		Store: store, Signer: authoritySigner, Repository: repository,
		Random: cryptorand.Reader, Clock: clock, Policy: integrationAuthorityPolicy{},
	})

	memberA := newProtectedMembershipMember(t, root, "a", 0xc3, authoritySigner, clock)
	memberB := newProtectedMembershipMember(t, root, "b", 0xc5, authoritySigner, clock)
	authorityRuntime := testkit.StartRuntime(t, runtimeinfra.Config{
		Name: "membership-authority-host",
		Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.DataConfig{Dir: filepath.Join(root, "authority-node")},
	}).Runtime
	authorityHost := testkit.NewAuthorityDeliveryOperatorCLIFixture(
		t, authorityRuntime, authorityService, nil,
	)
	memberA.host = testkit.NewAuthorityDeliveryOperatorCLIFixture(
		t, memberA.runtime, nil, memberA.delivery,
	)
	memberB.host = testkit.NewAuthorityDeliveryOperatorCLIFixture(
		t, memberB.runtime, nil, memberB.delivery,
	)

	created, err := authorityHost.Client.CreateRealmAuthority(
		ctx, connect.NewRequest(&protocol.CreateRealmAuthorityRequest{
			Version: domain.ContractVersion, RequestId: "membership-integration-genesis",
			RealmClass: domain.RealmClassProduction,
		}),
	)
	require.NoError(t, err)
	realmID := created.Msg.GetAuthority().GetRealmId()
	attestationA := prepareProtectedMembershipMember(t, ctx, memberA)
	attestationB := prepareProtectedMembershipMember(t, ctx, memberB)

	initialResource := domain.InitialGenerationDeliveryResource(
		realmID, "membership-integration-initial",
	)
	authorityHost.GrantExact(
		t, []identityaccess.Action{domain.ActionIssueDelivery},
		domain.ResourceKindGenerationDelivery, initialResource, false,
	)
	issued, err := authorityHost.Client.IssueInitialGeneration(
		ctx, connect.NewRequest(&protocol.IssueInitialGenerationRequest{
			Version: domain.ContractVersion, RequestId: "membership-integration-initial",
			RealmId: realmID, ChannelClass: string(identityapi.CapabilityRealmDiscovery),
			Permissions: uint32(identityapi.CapabilityPublish |
				identityapi.CapabilitySubscribe | identityapi.CapabilityStoreFetch),
			RecipientAttestation: attestationA,
			ValidForSeconds:      uint64(time.Hour / time.Second),
		}),
	)
	require.NoError(t, err)
	installAndAcknowledgeMembershipDelivery(
		t, ctx, authorityHost, memberA, realmID, issued.Msg.GetOperationId(),
		issued.Msg.GetDeliveryId(), issued.Msg.GetSealed(),
	)
	channelID := issued.Msg.GetChannelId()
	var channel [16]byte
	copy(channel[:], channelID)
	channelResource := domain.ChannelResource(realmID, channel)
	authorityHost.GrantExact(
		t, []identityaccess.Action{domain.ActionChangeMembership},
		domain.ResourceKindChannel, channelResource, false,
	)

	added, err := authorityHost.Client.ChangeChannelMembership(
		ctx, connect.NewRequest(&protocol.ChangeChannelMembershipRequest{
			Version: domain.ContractVersion, RequestId: "membership-integration-add",
			RealmId: realmID, ChannelId: channelID, Change: domain.MembershipChangeAdd,
			TargetPrincipal: memberB.principal,
			RecipientAttestations: []*protocol.GenerationDeliveryAttestation{
				attestationA, attestationB,
			},
			ValidForSeconds: uint64(time.Hour / time.Second),
			DrainForSeconds: uint64((15 * time.Minute) / time.Second),
		}),
	)
	require.NoError(t, err)
	require.Equal(t, domain.MemberStateCandidate, added.Msg.GetMemberState())
	require.Len(t, added.Msg.GetDeliveries(), 2)
	addReceipts := installMembershipRotationDeliveries(
		t, ctx, authorityHost, realmID, added.Msg,
		map[string]*protectedMembershipMember{
			memberA.principal: memberA,
			memberB.principal: memberB,
		},
	)
	addActivation := commitProtectedMembership(
		t, ctx, authorityHost, realmID, added.Msg.GetOperationId(),
	)
	for _, member := range []*protectedMembershipMember{memberA, memberB} {
		active := activateProtectedMembership(
			t, ctx, member, realmID, added.Msg.GetOperationId(),
			addActivation.Msg.GetActivation(),
		)
		addReceipts[member.principal] = active
		acknowledgeProtectedMembership(
			t, ctx, authorityHost, realmID, added.Msg.GetOperationId(), active, true,
		)
	}
	require.Equal(t, uint32(2), memberB.capabilities.GenerationReadiness(channel).CurrentGeneration)
	require.Zero(t, memberB.capabilities.GenerationReadiness(channel).PreviousGeneration)
	authorityState, found, err := store.Load(ctx)
	require.NoError(t, err)
	require.True(t, found)
	var removedGrant identityapi.CapabilityGrant
	for _, record := range authorityState.Channels[0].CurrentGrants {
		if record.SubjectPrincipal == memberB.principal {
			removedGrant = restoreIntegrationGrant(t, record)
		}
	}
	require.Equal(t, memberB.principal, removedGrant.SubjectPrincipal)

	removed, err := authorityHost.Client.ChangeChannelMembership(
		ctx, connect.NewRequest(&protocol.ChangeChannelMembershipRequest{
			Version: domain.ContractVersion, RequestId: "membership-integration-remove",
			RealmId: realmID, ChannelId: channelID, Change: domain.MembershipChangeRemove,
			TargetPrincipal: memberB.principal,
			RecipientAttestations: []*protocol.GenerationDeliveryAttestation{
				attestationA,
			},
			ValidForSeconds: uint64(time.Hour / time.Second),
			DrainForSeconds: uint64((15 * time.Minute) / time.Second),
		}),
	)
	require.NoError(t, err)
	require.Len(t, removed.Msg.GetDeliveries(), 1)
	require.Equal(t, memberA.principal, removed.Msg.GetDeliveries()[0].GetRecipientPrincipal())
	removeReceipts := installMembershipRotationDeliveries(
		t, ctx, authorityHost, realmID, removed.Msg,
		map[string]*protectedMembershipMember{memberA.principal: memberA},
	)
	removeActivation := commitProtectedMembership(
		t, ctx, authorityHost, realmID, removed.Msg.GetOperationId(),
	)
	activeA := activateProtectedMembership(
		t, ctx, memberA, realmID, removed.Msg.GetOperationId(),
		removeActivation.Msg.GetActivation(),
	)
	removeReceipts[memberA.principal] = activeA
	pending := acknowledgeProtectedMembership(
		t, ctx, authorityHost, realmID, removed.Msg.GetOperationId(), activeA, true,
	)
	require.Equal(t, domain.DeliveryPhaseActivationCommitted, pending.Msg.GetPhase())
	require.Error(t, memberA.capabilities.AuthorizeCapabilitySender(
		identityapi.CapabilitySenderUse{
			GrantID: removedGrant.GrantID, ChannelID: removedGrant.ChannelID,
			Generation: removedGrant.Generation, Subject: memberB.principal,
			Permission: identityapi.CapabilityPublish, Scope: removedGrant.Scope,
			At: now, ObservedAt: now,
		},
	), "stale removed traffic must fail before replay admission")

	authorityHost.GrantExact(
		t, []identityaccess.Action{domain.ActionSubmitFenceEvidence},
		domain.ResourceKindChannel, channelResource, false,
	)
	evidence := protectedFenceEvidence(
		realmID, removed.Msg.GetOperationId(), memberB.principal,
		authorityHost.Principal, now,
	)
	forged := proto.Clone(evidence).(*protocol.DeploymentFenceEvidence)
	forged.Controls[0].Actor = memberB.principal
	_, err = authorityHost.Client.SubmitDeploymentFenceEvidence(
		ctx, connect.NewRequest(&protocol.SubmitDeploymentFenceEvidenceRequest{
			Version: domain.ContractVersion, RealmId: realmID, ChannelId: channelID,
			OperationId: removed.Msg.GetOperationId(), Evidence: forged,
		}),
	)
	require.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
	completed, err := authorityHost.Client.SubmitDeploymentFenceEvidence(
		ctx, connect.NewRequest(&protocol.SubmitDeploymentFenceEvidenceRequest{
			Version: domain.ContractVersion, RealmId: realmID, ChannelId: channelID,
			OperationId: removed.Msg.GetOperationId(), Evidence: evidence,
		}),
	)
	require.NoError(t, err)
	require.Equal(t, domain.DeliveryPhaseCompleted, completed.Msg.GetPhase())
	require.NotEmpty(t, completed.Msg.GetEvidenceDigest())

	rejoined, err := authorityHost.Client.ChangeChannelMembership(
		ctx, connect.NewRequest(&protocol.ChangeChannelMembershipRequest{
			Version: domain.ContractVersion, RequestId: "membership-integration-rejoin",
			RealmId: realmID, ChannelId: channelID, Change: domain.MembershipChangeAdd,
			TargetPrincipal: memberB.principal,
			RecipientAttestations: []*protocol.GenerationDeliveryAttestation{
				attestationA, attestationB,
			},
			ValidForSeconds: uint64(time.Hour / time.Second),
			DrainForSeconds: uint64((15 * time.Minute) / time.Second),
		}),
	)
	require.NoError(t, err)
	require.Equal(t, uint32(4), rejoined.Msg.GetPendingGeneration())
	installMembershipRotationDeliveries(
		t, ctx, authorityHost, realmID, rejoined.Msg,
		map[string]*protectedMembershipMember{
			memberA.principal: memberA, memberB.principal: memberB,
		},
	)
	rejoinActivation := commitProtectedMembership(
		t, ctx, authorityHost, realmID, rejoined.Msg.GetOperationId(),
	)
	for _, member := range []*protectedMembershipMember{memberA, memberB} {
		active := activateProtectedMembership(
			t, ctx, member, realmID, rejoined.Msg.GetOperationId(),
			rejoinActivation.Msg.GetActivation(),
		)
		acknowledgeProtectedMembership(
			t, ctx, authorityHost, realmID, rejoined.Msg.GetOperationId(), active, true,
		)
	}
	require.Equal(
		t, uint32(4),
		memberB.capabilities.GenerationReadiness(channel).CurrentGeneration,
	)
	require.Zero(t, memberB.capabilities.GenerationReadiness(channel).PreviousGeneration)
}

type protectedMembershipMember struct {
	principal    string
	capabilities *identitycapability.Service
	delivery     *channeldelivery.Service
	runtime      *runtimeinfra.Node
	host         testkit.OperatorCLIFixture
}

func newProtectedMembershipMember(
	t *testing.T,
	root, name string,
	keyByte byte,
	authoritySigner *integrationSigner,
	clock func() time.Time,
) *protectedMembershipMember {
	t.Helper()
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{keyByte}, ed25519.SeedSize))
	principal, err := identityprincipal.FromEd25519PublicKey(
		private.Public().(ed25519.PublicKey),
	)
	require.NoError(t, err)
	trust, err := identitytrust.NewRegistry([]identitytrust.Entry{{
		Principal: authoritySigner.principal,
		PublicKey: authoritySigner.private.Public().(ed25519.PublicKey),
		Purposes:  []identitytrust.Purpose{identitytrust.PurposeChannelIssue},
	}})
	require.NoError(t, err)
	capabilities, err := identitycapability.NewService(
		filepath.Join(root, "member-"+name, "capabilities.db"),
		bytes.Repeat([]byte{keyByte + 1}, 32), principal.String(), trust,
		integrationCapabilityPolicy{}, clock,
	)
	require.NoError(t, err)
	delivery, err := channeldelivery.New(
		capabilities, private, principal.String(), clock,
	)
	require.NoError(t, err)
	runtime := testkit.StartRuntime(t, runtimeinfra.Config{
		Name: "membership-member-" + name,
		Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.DataConfig{Dir: filepath.Join(root, "member-"+name+"-node")},
	}).Runtime
	return &protectedMembershipMember{
		principal: principal.String(), capabilities: capabilities,
		delivery: delivery, runtime: runtime,
	}
}

func prepareProtectedMembershipMember(
	t *testing.T,
	ctx context.Context,
	member *protectedMembershipMember,
) *protocol.GenerationDeliveryAttestation {
	t.Helper()
	member.host.GrantExact(
		t, []identityaccess.Action{"realm.channel.delivery.prepare"},
		identityaccess.ResourceKind("principal"), member.principal, false,
	)
	prepared, err := member.host.Client.PrepareGenerationDelivery(
		ctx, connect.NewRequest(&protocol.PrepareGenerationDeliveryRequest{
			Version:          channeldelivery.ContractVersion,
			SubjectPrincipal: member.principal,
			ValidForSeconds:  uint64(time.Hour / time.Second),
		}),
	)
	require.NoError(t, err)
	return prepared.Msg.GetAttestation()
}

func installAndAcknowledgeMembershipDelivery(
	t *testing.T,
	ctx context.Context,
	authorityHost testkit.OperatorCLIFixture,
	member *protectedMembershipMember,
	realmID, operationID, deliveryID string,
	sealed *protocol.SealedGenerationDelivery,
) *protocol.GenerationDeliveryReceipt {
	t.Helper()
	resource, valid := domain.GenerationDeliveryResource(realmID, operationID, deliveryID)
	require.True(t, valid)
	member.host.GrantExact(
		t, []identityaccess.Action{"realm.channel.delivery.install"},
		domain.ResourceKindGenerationDelivery, resource, false,
	)
	installed, err := member.host.Client.InstallGenerationDelivery(
		ctx, connect.NewRequest(&protocol.InstallGenerationDeliveryRequest{
			Version: channeldelivery.ContractVersion, Sealed: sealed,
		}),
	)
	require.NoError(t, err)
	authorityHost.GrantExact(
		t, []identityaccess.Action{domain.ActionAcknowledgeDelivery},
		domain.ResourceKindGenerationDelivery, resource, false,
	)
	_, err = authorityHost.Client.AcknowledgeInitialGeneration(
		ctx, connect.NewRequest(&protocol.AcknowledgeInitialGenerationRequest{
			Version: domain.ContractVersion, RealmId: realmID,
			OperationId: operationID, Receipt: installed.Msg.GetReceipt(),
		}),
	)
	require.NoError(t, err)
	return installed.Msg.GetReceipt()
}

func installMembershipRotationDeliveries(
	t *testing.T,
	ctx context.Context,
	authorityHost testkit.OperatorCLIFixture,
	realmID string,
	rotation *protocol.RotateChannelResponse,
	members map[string]*protectedMembershipMember,
) map[string]*protocol.GenerationDeliveryReceipt {
	t.Helper()
	result := make(map[string]*protocol.GenerationDeliveryReceipt, len(rotation.GetDeliveries()))
	for _, delivery := range rotation.GetDeliveries() {
		member := members[delivery.GetRecipientPrincipal()]
		require.NotNil(t, member)
		result[member.principal] = installAndAcknowledgeMembershipDelivery(
			t, ctx, authorityHost, member, realmID, rotation.GetOperationId(),
			delivery.GetDeliveryId(), delivery.GetSealed(),
		)
	}
	return result
}

func commitProtectedMembership(
	t *testing.T,
	ctx context.Context,
	authorityHost testkit.OperatorCLIFixture,
	realmID, operationID string,
) *connect.Response[protocol.CommitChannelActivationResponse] {
	t.Helper()
	resource := domain.OperationResource(realmID, operationID)
	authorityHost.GrantExact(
		t, []identityaccess.Action{domain.ActionCommitActivation},
		domain.ResourceKindOperation, resource, false,
	)
	result, err := authorityHost.Client.CommitChannelActivation(
		ctx, connect.NewRequest(&protocol.CommitChannelActivationRequest{
			Version: domain.ContractVersion, RealmId: realmID, OperationId: operationID,
		}),
	)
	require.NoError(t, err)
	return result
}

func activateProtectedMembership(
	t *testing.T,
	ctx context.Context,
	member *protectedMembershipMember,
	realmID, operationID string,
	activation *protocol.GenerationActivation,
) *protocol.GenerationDeliveryReceipt {
	t.Helper()
	resource := domain.OperationResource(realmID, operationID)
	member.host.GrantExact(
		t, []identityaccess.Action{"realm.channel.generation.activate"},
		domain.ResourceKindOperation, resource, false,
	)
	result, err := member.host.Client.ActivateGeneration(
		ctx, connect.NewRequest(&protocol.ActivateGenerationRequest{
			Version: domain.ContractVersion, Activation: activation,
		}),
	)
	require.NoError(t, err)
	return result.Msg.GetReceipt()
}

func acknowledgeProtectedMembership(
	t *testing.T,
	ctx context.Context,
	authorityHost testkit.OperatorCLIFixture,
	realmID, operationID string,
	receipt *protocol.GenerationDeliveryReceipt,
	approved bool,
) *connect.Response[protocol.AcknowledgeChannelActivationResponse] {
	t.Helper()
	resource, valid := domain.GenerationDeliveryResource(
		realmID, operationID, receipt.GetDeliveryId(),
	)
	require.True(t, valid)
	authorityHost.GrantExact(
		t, []identityaccess.Action{domain.ActionAcknowledgeActivation},
		domain.ResourceKindGenerationDelivery, resource, false,
	)
	result, err := authorityHost.Client.AcknowledgeChannelActivation(
		ctx, connect.NewRequest(&protocol.AcknowledgeChannelActivationRequest{
			Version: domain.ContractVersion, RealmId: realmID, OperationId: operationID,
			ApprovedHost: approved, Receipt: receipt,
		}),
	)
	require.NoError(t, err)
	return result
}

func protectedFenceEvidence(
	realmID, operationID, target, actor string,
	now time.Time,
) *protocol.DeploymentFenceEvidence {
	digest := "sha256:" + strings.Repeat("a", 64)
	return &protocol.DeploymentFenceEvidence{
		Version: domain.ContractVersion, RealmId: realmID,
		OperationId: operationID, TargetPrincipal: target,
		ManifestDigest: digest, RequestId: "membership-integration-fence",
		Reason: "membership_removed", ObservedAt: timestamppb.New(now),
		Controls: []*protocol.DeploymentFenceControl{
			{Kind: "target_ingress_blocked", Actor: actor, ReceiptDigest: digest},
			{Kind: "discovery_withdrawn", Actor: actor, ReceiptDigest: digest},
			{Kind: "peer_id_denied", Actor: actor, ReceiptDigest: digest},
		},
	}
}

func restoreIntegrationGrant(
	t *testing.T,
	record domain.CapabilityGrantRecord,
) identityapi.CapabilityGrant {
	t.Helper()
	secret, ok := identityapi.NewCapabilitySecret(record.Secret)
	require.True(t, ok)
	return identityapi.CapabilityGrant{
		Version: record.Version, ChannelID: record.ChannelID,
		Generation: record.Generation, Secret: secret, GrantID: record.GrantID,
		IssuerPrincipal:  record.IssuerPrincipal,
		SubjectPrincipal: record.SubjectPrincipal,
		Permissions:      record.Permissions, Scope: record.Scope,
		NotBefore: record.NotBefore, NotAfter: record.NotAfter,
		Signature: append([]byte(nil), record.Signature...),
	}
}
