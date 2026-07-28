package authority

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
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

func TestMigrateLocalV2ReconcilesEveryMemberAndFencesOrdinaryMutation(t *testing.T) {
	ctx := context.Background()
	legacy := newLocalV2TestInput(t)
	service, store, repository := newLocalV2MigrationService(legacy)

	result, err := service.MigrateLocalV2(
		ctx, migrateLocalV2Command(), legacy.request,
	)
	require.NoError(t, err)
	require.True(t, ValidRealmID(result.RealmID))
	require.Equal(t, uint64(1), result.AuthoritySequence)
	require.Equal(t, PhaseMigrationRotationRequired, result.Phase)
	require.Equal(t, ReadinessDegraded, result.Readiness)
	require.True(t, result.FreshRotationRequired)
	require.Equal(t, legacy.discoveryID, result.DiscoveryChannelID)
	require.Equal(t, legacy.dataID, result.DataChannelID)
	require.Equal(t, legacy.request.OldManagerFence.EvidenceDigest, result.OldManagerFenceDigest)
	require.Equal(t, result.CheckpointDigest, repository.head.Digest)
	require.Equal(t, legacy.signer.principal, store.state.AuthorityPrincipal)
	require.Len(t, store.state.Members, 1)
	require.Len(t, store.state.Channels, 2)
	require.Len(t, store.state.Migration.RequiredRotationChannelIDs, 2)
	require.Empty(t, store.state.Migration.RotatedChannelIDs)
	require.Equal(t, ActionMigrateLocalV2, store.state.AuditLog[0].Action)
	require.Equal(t, store.state.Migration.CommitEvidenceDigest, store.state.AuditLog[0].EvidenceDigest)
	require.Equal(t, store.state.AuditLog[0].Hash, store.state.Checkpoint.AuditHead)

	_, err = service.CreateOrReopen(ctx, Command{
		Actor: "operator", Effective: "operator", Action: ActionCreate,
		ResourceKind: ResourceKindAuthorityInstance, ResourceID: PrimaryAuthorityInstance,
	}, CreateRequest{
		Version: ContractVersion, RequestID: "ordinary-mutation",
		RealmClass: RealmClassProduction,
	})
	require.ErrorIs(t, err, ErrRecoveryRequired)

	restarted := New(Config{
		Store: store, Signer: legacy.signer, Repository: repository,
		Clock: legacy.clock, Policy: allowPolicy{},
	})
	require.Equal(t, PhaseMigrationRotationRequired, restarted.Readiness().Phase)
	require.Equal(t, ReasonMigrationRotationRequired, restarted.Readiness().Reason)
	replayed, err := restarted.MigrateLocalV2(ctx, migrateLocalV2Command(), legacy.request)
	require.NoError(t, err)
	require.Equal(t, result, replayed)
}

func TestMigrateLocalV2RejectsUnknownDowngradeAndMemberMismatch(t *testing.T) {
	tests := []struct {
		name   string
		tamper func(*localV2TestInput)
	}{
		{
			name: "unknown authority field",
			tamper: func(input *localV2TestInput) {
				raw := bytes.TrimSpace(input.request.AuthorityState)
				input.request.AuthorityState = append(
					append([]byte(nil), raw[:len(raw)-1]...),
					[]byte(`,"unknown":true}`)...,
				)
			},
		},
		{
			name: "downgrade authority version",
			tamper: func(input *localV2TestInput) {
				var state map[string]any
				require.NoError(t, json.Unmarshal(input.request.AuthorityState, &state))
				state["version"] = "ardents.local-realm/v1"
				input.request.AuthorityState, _ = json.Marshal(state)
			},
		},
		{
			name: "unknown node field",
			tamper: func(input *localV2TestInput) {
				raw := bytes.TrimSpace(input.request.Members[0].NodeState)
				input.request.Members[0].NodeState = append(
					append([]byte(nil), raw[:len(raw)-1]...),
					[]byte(`,"unknown":true}`)...,
				)
			},
		},
		{
			name: "missing member evidence",
			tamper: func(input *localV2TestInput) {
				input.request.Members = nil
			},
		},
		{
			name: "capability store grant mismatch",
			tamper: func(input *localV2TestInput) {
				input.request.Members[0].ReceiverGrants[0].Signature[0] ^= 1
			},
		},
		{
			name: "old manager not fenced",
			tamper: func(input *localV2TestInput) {
				input.request.OldManagerFence.SharedAuthorityRemoved = false
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := newLocalV2TestInput(t)
			test.tamper(&input)
			service, store, repository := newLocalV2MigrationService(input)
			_, err := service.MigrateLocalV2(
				context.Background(), migrateLocalV2Command(), input.request,
			)
			require.ErrorIs(t, err, ErrInvalidArgument)
			require.False(t, store.found)
			require.False(t, repository.found)
		})
	}
}

func TestMigrateLocalV2RequiresMatchingSignerAndEmptyRepository(t *testing.T) {
	ctx := context.Background()
	t.Run("signer mismatch", func(t *testing.T) {
		input := newLocalV2TestInput(t)
		service, store, repository := newLocalV2MigrationService(input)
		service.signer = newTestSigner(t, 0x92)
		_, err := service.MigrateLocalV2(ctx, migrateLocalV2Command(), input.request)
		require.ErrorIs(t, err, ErrPermissionDenied)
		require.False(t, store.found)
		require.False(t, repository.found)
	})
	t.Run("repository occupied", func(t *testing.T) {
		input := newLocalV2TestInput(t)
		service, store, repository := newLocalV2MigrationService(input)
		repository.found = true
		repository.head = SignedCheckpoint{Digest: "occupied"}
		_, err := service.MigrateLocalV2(ctx, migrateLocalV2Command(), input.request)
		require.ErrorIs(t, err, ErrConflict)
		require.False(t, store.found)
		require.Equal(t, "occupied", repository.head.Digest)
	})
}

func TestLocalV2MigrationRequiresFreshRotationCASForBothChannels(t *testing.T) {
	ctx := context.Background()
	legacy := newLocalV2TestInput(t)
	service, store, repository := newLocalV2MigrationService(legacy)
	result, err := service.MigrateLocalV2(ctx, migrateLocalV2Command(), legacy.request)
	require.NoError(t, err)

	trust, err := identitytrust.NewRegistry([]identitytrust.Entry{{
		Principal: legacy.signer.principal,
		PublicKey: legacy.signer.private.Public().(ed25519.PublicKey),
		Purposes:  []identitytrust.Purpose{identitytrust.PurposeChannelIssue},
	}})
	require.NoError(t, err)
	member, err := identitycapability.NewService(
		filepath.Join(t.TempDir(), "member-capabilities.db"),
		bytes.Repeat([]byte{0x93}, 32), legacy.memberPrincipal,
		trust, authorityCapabilityPolicy{}, legacy.clock,
	)
	require.NoError(t, err)
	for _, grant := range legacy.request.Members[0].ReceiverGrants {
		_, err := member.ImportGrant(grant)
		require.NoError(t, err)
	}
	attestation, err := member.AttestDeliveryPublicKey(
		legacy.memberPrivate, legacy.clock().Add(time.Hour),
	)
	require.NoError(t, err)
	rotationRandom := make([]byte, 1024)
	for index := range rotationRandom {
		rotationRandom[index] = byte(index%251 + 1)
	}
	service.random = bytes.NewReader(rotationRandom)

	for index, channelID := range [][16]byte{result.DiscoveryChannelID, result.DataChannelID} {
		completeMigrationChannelRotation(
			t, ctx, service, member, attestation, result.RealmID,
			channelID, "migration-rotation-"+string(rune('1'+index)),
		)
		if index == 0 {
			require.Equal(t, PhaseMigrationRotationRequired, service.Readiness().Phase)
			require.Len(t, store.state.Migration.RotatedChannelIDs, 1)
		}
	}
	require.Equal(t, PhaseReady, service.Readiness().Phase)
	require.Equal(t, ReadinessReady, service.Readiness().Readiness)
	require.False(t, service.migrationPending)
	require.Len(t, store.state.Migration.RotatedChannelIDs, 2)
	require.Greater(t, repository.head.AuthoritySequence, uint64(1))
	require.Equal(t, store.state.Checkpoint, repository.head)
	require.Equal(t, legacy.request.OldManagerFence.EvidenceDigest, store.state.Migration.OldManagerFenceDigest)

	replayed, err := service.MigrateLocalV2(ctx, migrateLocalV2Command(), legacy.request)
	require.NoError(t, err)
	require.False(t, replayed.FreshRotationRequired)
	require.Equal(t, PhaseReady, replayed.Phase)
}

type localV2TestInput struct {
	request         MigrateLocalV2Request
	signer          *testSigner
	memberPrivate   ed25519.PrivateKey
	memberPrincipal string
	discoveryID     [16]byte
	dataID          [16]byte
	clock           func() time.Time
}

func newLocalV2TestInput(t *testing.T) localV2TestInput {
	t.Helper()
	signer := newTestSigner(t, 0x81)
	memberPrivate := newTestSigner(t, 0x82).private
	memberID, err := identityprincipal.FromEd25519PublicKey(
		memberPrivate.Public().(ed25519.PublicKey),
	)
	require.NoError(t, err)
	clock := func() time.Time {
		return time.Date(2026, 7, 28, 18, 0, 0, 0, time.UTC)
	}
	var discoveryID, dataID [16]byte
	copy(discoveryID[:], bytes.Repeat([]byte{0x83}, 16))
	copy(dataID[:], bytes.Repeat([]byte{0x84}, 16))
	discoverySecret, ok := identityapi.NewCapabilitySecret(bytes.Repeat([]byte{0x85}, 32))
	require.True(t, ok)
	dataSecret, ok := identityapi.NewCapabilitySecret(bytes.Repeat([]byte{0x86}, 32))
	require.True(t, ok)
	discoveryGrantID := bytes.Repeat([]byte{0x87}, 16)
	dataGrantID := bytes.Repeat([]byte{0x88}, 16)
	node := localV2NodeState{
		Version: localV2NodeVersion, Subject: memberID.String(), Issuer: signer.principal,
		Discovery: localV2GrantState{
			ID:        base64.StdEncoding.EncodeToString(discoveryGrantID),
			NotBefore: clock().Add(-time.Minute), NotAfter: clock().Add(30 * 24 * time.Hour),
		},
		Data: localV2GrantState{
			ID:        base64.StdEncoding.EncodeToString(dataGrantID),
			NotBefore: clock().Add(-time.Minute), NotAfter: clock().Add(30 * 24 * time.Hour),
		},
	}
	authorityState := localV2AuthorityState{
		Version:       localV2AuthorityVersion,
		IssuerPrivate: base64.StdEncoding.EncodeToString(signer.private),
		Discovery: localV2ChannelState{
			ID:     base64.StdEncoding.EncodeToString(discoveryID[:]),
			Secret: base64.StdEncoding.EncodeToString(discoverySecret.Bytes()), Generation: 1,
		},
		Data: localV2ChannelState{
			ID:     base64.StdEncoding.EncodeToString(dataID[:]),
			Secret: base64.StdEncoding.EncodeToString(dataSecret.Bytes()), Generation: 1,
		},
		Members: map[string]localV2NodeState{memberID.String(): node},
	}
	authorityRaw, err := json.Marshal(authorityState)
	require.NoError(t, err)
	nodeRaw, err := json.Marshal(node)
	require.NoError(t, err)
	var discoveryGrantIDFixed, dataGrantIDFixed [16]byte
	copy(discoveryGrantIDFixed[:], discoveryGrantID)
	copy(dataGrantIDFixed[:], dataGrantID)
	discoveryGrant, err := identitycapability.SignGrant(identityapi.CapabilityGrant{
		Version: ContractVersion, ChannelID: discoveryID, Generation: 1,
		Secret: discoverySecret, GrantID: discoveryGrantIDFixed,
		IssuerPrincipal: signer.principal, SubjectPrincipal: memberID.String(),
		Permissions: identityapi.CapabilityPublish | identityapi.CapabilitySubscribe |
			identityapi.CapabilityStoreFetch,
		Scope:     identityapi.CapabilityRealmDiscovery,
		NotBefore: node.Discovery.NotBefore, NotAfter: node.Discovery.NotAfter,
	}, signer.private)
	require.NoError(t, err)
	dataGrant, err := identitycapability.SignGrant(identityapi.CapabilityGrant{
		Version: ContractVersion, ChannelID: dataID, Generation: 1,
		Secret: dataSecret, GrantID: dataGrantIDFixed,
		IssuerPrincipal: signer.principal, SubjectPrincipal: memberID.String(),
		Permissions: identityapi.CapabilityPublish | identityapi.CapabilitySubscribe |
			identityapi.CapabilityStoreFetch,
		Scope:     identityapi.CapabilityDataExchange,
		NotBefore: node.Data.NotBefore, NotAfter: node.Data.NotAfter,
	}, signer.private)
	require.NoError(t, err)
	return localV2TestInput{
		request: MigrateLocalV2Request{
			Version: ContractVersion, RequestID: "local-v2-migration",
			AuthorityState: authorityRaw,
			Members: []LocalV2MemberEvidence{{
				NodeState: nodeRaw,
				ReceiverGrants: []identityapi.CapabilityGrant{
					discoveryGrant, dataGrant,
				},
			}},
			OldManagerFence: LocalV2ManagerFence{
				OldProcessStopped: true, SharedAuthorityRemoved: true,
				EvidenceDigest: "sha256:" + string(bytes.Repeat([]byte{'a'}, 64)),
			},
		},
		signer: signer, memberPrivate: memberPrivate,
		memberPrincipal: memberID.String(),
		discoveryID:     discoveryID, dataID: dataID, clock: clock,
	}
}

func newLocalV2MigrationService(
	input localV2TestInput,
) (*Service, *memoryStore, *memoryRepository) {
	store := &memoryStore{}
	repository := &memoryRepository{}
	service := New(Config{
		Store: store, Signer: input.signer, Repository: repository,
		Random: bytes.NewReader(bytes.Repeat([]byte{0x91}, 1024)),
		Clock:  input.clock, Policy: allowPolicy{},
	})
	return service, store, repository
}

func migrateLocalV2Command() Command {
	return Command{
		Actor: "operator", Effective: "operator", Action: ActionMigrateLocalV2,
		ResourceKind: ResourceKindAuthorityInstance, ResourceID: PrimaryAuthorityInstance,
	}
}

func completeMigrationChannelRotation(
	t *testing.T,
	ctx context.Context,
	service *Service,
	member *identitycapability.Service,
	attestation identityapi.CapabilityDeliveryAttestation,
	realmID string,
	channelID [16]byte,
	requestID string,
) {
	t.Helper()
	resource := ChannelResource(realmID, channelID)
	rotated, err := service.RotateChannel(ctx, Command{
		Actor: "operator", Effective: "operator", Action: ActionRotateGeneration,
		ResourceKind: ResourceKindChannel, ResourceID: resource,
	}, RotationRequest{
		Version: ContractVersion, RequestID: requestID, RealmID: realmID,
		ChannelID:             channelID,
		RecipientAttestations: []identityapi.CapabilityDeliveryAttestation{attestation},
		ValidFor:              time.Hour, DrainFor: 15 * time.Minute,
	})
	require.NoError(t, err)
	require.Len(t, rotated.Deliveries, 1)
	installed, err := member.InstallGenerationDelivery(rotated.Deliveries[0].Sealed)
	require.NoError(
		t, err, "detail: %v / %v",
		errors.Unwrap(err), errors.Unwrap(errors.Unwrap(err)),
	)
	deliveryResource, ok := GenerationDeliveryResource(
		realmID, rotated.OperationID, rotated.Deliveries[0].DeliveryID,
	)
	require.True(t, ok)
	_, err = service.AcknowledgeInitialGeneration(ctx, Command{
		Actor: "operator", Effective: "operator", Action: ActionAcknowledgeDelivery,
		ResourceKind: ResourceKindGenerationDelivery, ResourceID: deliveryResource,
	}, InitialGenerationAcknowledgeRequest{
		Version: ContractVersion, RealmID: realmID, Receipt: installed,
	})
	require.NoError(t, err)
	committed, err := service.CommitChannelActivation(ctx, Command{
		Actor: "operator", Effective: "operator", Action: ActionCommitActivation,
		ResourceKind: ResourceKindOperation,
		ResourceID:   OperationResource(realmID, rotated.OperationID),
	}, ActivationCommitRequest{
		Version: ContractVersion, RealmID: realmID, OperationID: rotated.OperationID,
	})
	require.NoError(t, err)
	active, err := member.ActivateGeneration(committed.Activation)
	require.NoError(t, err)
	active, err = member.ConfirmGenerationRuntimeAdoption(committed.Activation)
	require.NoError(t, err)
	_, err = service.AcknowledgeChannelActivation(ctx, Command{
		Actor: "operator", Effective: "operator", Action: ActionAcknowledgeActivation,
		ResourceKind: ResourceKindGenerationDelivery, ResourceID: deliveryResource,
	}, ActivationAcknowledgeRequest{
		Version: ContractVersion, RealmID: realmID,
		OperationID: rotated.OperationID, ApprovedHost: true, Receipt: active,
	})
	require.NoError(t, err)
}
