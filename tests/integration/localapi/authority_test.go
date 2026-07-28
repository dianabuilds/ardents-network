//go:build integration

package localapi_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	cryptorand "crypto/rand"
	"encoding/json"
	"errors"
	"path/filepath"
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
)

func TestRealmAuthorityGenesisInspectAndRestartThroughProtectedOperatorInterface(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer: testkit.LayerIntegration, Domain: "realm-authority", ScenarioID: "CGA-01",
		Suite: "integration", Tags: []string{"integration", "authority", "security", "restart"},
		Speed: "fast", Environment: "local",
	})
	ctx := context.Background()
	root := t.TempDir()
	key := bytes.Repeat([]byte{0x81}, domain.AuthorityStoreKeyBytes)
	storePath := filepath.Join(root, "authority", "realm-authority.db")
	checkpointPath := filepath.Join(root, "independent-checkpoints")
	require.NoError(t, storage.EnsurePrivateDir(filepath.Dir(storePath)))
	require.NoError(t, storage.EnsurePrivateDir(checkpointPath))
	store, err := domain.OpenFileStore(ctx, storePath, key)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	repository, err := domain.NewFileCheckpointRepository(checkpointPath)
	require.NoError(t, err)
	signer := integrationAuthoritySigner(t, 0x82)
	clock := func() time.Time { return time.Date(2026, 7, 27, 13, 0, 0, 0, time.UTC) }
	service := domain.New(domain.Config{
		Store: store, Signer: signer, Repository: repository,
		Random: bytes.NewReader(bytes.Repeat([]byte{0x83}, 1024)),
		Clock:  clock, Policy: integrationAuthorityPolicy{},
	})

	runtime := testkit.StartRuntime(t, runtimeinfra.Config{
		Name: "authority-node", Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.DataConfig{Dir: filepath.Join(root, "node")},
	}).Runtime
	fixture := testkit.NewAuthorityOperatorCLIFixture(t, runtime, service)
	unknown := &protocol.CreateRealmAuthorityRequest{
		Version: domain.ContractVersion, RequestId: "unknown-field",
		RealmClass: domain.RealmClassProduction,
	}
	unknown.ProtoReflect().SetUnknown([]byte{0x98, 0x06, 0x01})
	_, err = fixture.Client.CreateRealmAuthority(ctx, connect.NewRequest(unknown))
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	require.NotContains(t, err.Error(), storePath)
	require.NotContains(t, err.Error(), signer.principal)

	_, err = fixture.Client.CreateRealmAuthority(ctx, connect.NewRequest(&protocol.CreateRealmAuthorityRequest{
		Version: domain.ContractVersion + 1, RequestId: "unsupported-version",
		RealmClass: domain.RealmClassProduction,
	}))
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	require.Equal(t, "authority_unsupported_version", authorityErrorCode(t, err))

	create, err := fixture.Client.CreateRealmAuthority(ctx, connect.NewRequest(&protocol.CreateRealmAuthorityRequest{
		Version: domain.ContractVersion, RequestId: "operator-request-001",
		RealmClass: domain.RealmClassProduction,
	}))
	require.NoError(t, err)
	realmID := create.Msg.GetAuthority().GetRealmId()
	require.True(t, domain.ValidRealmID(realmID))
	require.Equal(t, domain.PhaseReady, create.Msg.GetAuthority().GetPhase())
	require.Equal(t, uint64(1), create.Msg.GetAuthority().GetAuthoritySequence())
	require.Zero(t, create.Msg.GetAuthority().GetCurrentGeneration())
	require.Equal(
		t, time.Date(2026, 7, 28, 13, 0, 0, 0, time.UTC),
		create.Msg.GetAuthority().GetOperationDeadline().AsTime(),
	)
	persisted, found, err := store.Load(ctx)
	require.NoError(t, err)
	require.True(t, found)
	require.Len(t, persisted.AuditLog, 1)
	require.Len(t, persisted.AuditOutbox, 1)

	_, err = fixture.Client.InspectRealmAuthority(ctx, connect.NewRequest(&protocol.InspectRealmAuthorityRequest{
		Version: domain.ContractVersion, RealmId: realmID,
	}))
	require.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
	fixture.GrantExact(t, []identityaccess.Action{domain.ActionInspect}, domain.ResourceKindRealm, realmID, false)
	inspected, err := fixture.Client.InspectRealmAuthority(ctx, connect.NewRequest(&protocol.InspectRealmAuthorityRequest{
		Version: domain.ContractVersion, RealmId: realmID,
	}))
	require.NoError(t, err)
	require.Equal(t, create.Msg.GetAuthority(), inspected.Msg.GetAuthority())
	encoded, err := json.Marshal(inspected.Msg)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), signer.principal)
	require.NotContains(t, string(encoded), string(signer.private))
	require.NotContains(t, string(encoded), "public_key")

	conflict, err := fixture.Client.CreateRealmAuthority(ctx, connect.NewRequest(&protocol.CreateRealmAuthorityRequest{
		Version: domain.ContractVersion, RequestId: "operator-request-001", RealmClass: "changed",
	}))
	require.Nil(t, conflict)
	require.Equal(t, connect.CodeAlreadyExists, connect.CodeOf(err))

	require.NoError(t, store.Close())
	reopenedStore, err := domain.OpenFileStore(ctx, storePath, key)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, reopenedStore.Close()) })
	restartAudit := &integrationAuthorityAudit{}
	restarted := domain.New(domain.Config{
		Store: reopenedStore, Signer: signer, Repository: repository,
		Random: bytes.NewReader(bytes.Repeat([]byte{0x84}, 1024)),
		Clock:  clock, Policy: integrationAuthorityPolicy{}, Audit: restartAudit,
	})
	require.Len(t, restartAudit.records, 1)
	persisted, found, err = reopenedStore.Load(ctx)
	require.NoError(t, err)
	require.True(t, found)
	require.Len(t, persisted.AuditLog, 1)
	require.Empty(t, persisted.AuditOutbox)
	restartFixture := testkit.NewAuthorityOperatorCLIFixture(t, runtime, restarted)
	replayed, err := restartFixture.Client.CreateRealmAuthority(ctx, connect.NewRequest(&protocol.CreateRealmAuthorityRequest{
		Version: domain.ContractVersion, RequestId: "operator-request-001",
		RealmClass: domain.RealmClassProduction,
	}))
	require.NoError(t, err)
	require.Equal(t, create.Msg.GetOperationId(), replayed.Msg.GetOperationId())
	require.Equal(t, realmID, replayed.Msg.GetAuthority().GetRealmId())
	require.Equal(t, create.Msg.GetAuthority().GetCheckpointDigest(), replayed.Msg.GetAuthority().GetCheckpointDigest())
}

func TestInitialGenerationDeliveryThroughProtectedOperatorInterface(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer: testkit.LayerIntegration, Domain: "realm-authority", ScenarioID: "CGA-02",
		Suite: "integration", Tags: []string{"integration", "authority", "security", "delivery"},
		Speed: "fast", Environment: "local",
	})
	ctx := context.Background()
	root := t.TempDir()
	now := time.Now().UTC().Truncate(time.Second)
	clock := func() time.Time { return now }
	storePath := filepath.Join(root, "authority", "realm-authority.db")
	checkpointPath := filepath.Join(root, "independent-checkpoints")
	require.NoError(t, storage.EnsurePrivateDir(filepath.Dir(storePath)))
	require.NoError(t, storage.EnsurePrivateDir(checkpointPath))
	store, err := domain.OpenFileStore(
		ctx, storePath, bytes.Repeat([]byte{0xa1}, domain.AuthorityStoreKeyBytes),
	)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	repository, err := domain.NewFileCheckpointRepository(checkpointPath)
	require.NoError(t, err)
	authoritySigner := integrationAuthoritySigner(t, 0xa2)
	authorityService := domain.New(domain.Config{
		Store: store, Signer: authoritySigner, Repository: repository,
		Random: bytes.NewReader(bytes.Repeat([]byte{0xa3}, 2048)),
		Clock:  clock, Policy: integrationAuthorityPolicy{},
	})

	memberPrivate := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0xa4}, ed25519.SeedSize))
	memberPrincipal, err := identityprincipal.FromEd25519PublicKey(
		memberPrivate.Public().(ed25519.PublicKey),
	)
	require.NoError(t, err)
	trust, err := identitytrust.NewRegistry([]identitytrust.Entry{{
		Principal: authoritySigner.principal,
		PublicKey: authoritySigner.private.Public().(ed25519.PublicKey),
		Purposes:  []identitytrust.Purpose{identitytrust.PurposeChannelIssue},
	}})
	require.NoError(t, err)
	memberCapabilities, err := identitycapability.NewService(
		filepath.Join(root, "member", "capabilities.db"),
		bytes.Repeat([]byte{0xa5}, 32), memberPrincipal.String(), trust,
		integrationCapabilityPolicy{}, clock,
	)
	require.NoError(t, err)
	memberDelivery, err := channeldelivery.New(
		memberCapabilities, memberPrivate, memberPrincipal.String(), clock,
	)
	require.NoError(t, err)

	runtime := testkit.StartRuntime(t, runtimeinfra.Config{
		Name: "authority-delivery-node",
		Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.DataConfig{Dir: filepath.Join(root, "node")},
	}).Runtime
	fixture := testkit.NewAuthorityDeliveryOperatorCLIFixture(
		t, runtime, authorityService, memberDelivery,
	)
	create, err := fixture.Client.CreateRealmAuthority(
		ctx, connect.NewRequest(&protocol.CreateRealmAuthorityRequest{
			Version: domain.ContractVersion, RequestId: "delivery-genesis-001",
			RealmClass: domain.RealmClassProduction,
		}),
	)
	require.NoError(t, err)
	realmID := create.Msg.GetAuthority().GetRealmId()

	fixture.GrantExact(
		t, []identityaccess.Action{"realm.channel.delivery.prepare"},
		identityaccess.ResourceKind("principal"), memberPrincipal.String(), false,
	)
	prepared, err := fixture.Client.PrepareGenerationDelivery(
		ctx, connect.NewRequest(&protocol.PrepareGenerationDeliveryRequest{
			Version: channeldelivery.ContractVersion, SubjectPrincipal: memberPrincipal.String(),
			ValidForSeconds: uint64(time.Hour / time.Second),
		}),
	)
	require.NoError(t, err)
	require.NotNil(t, prepared.Msg.GetAttestation())

	requestID := "initial-delivery-001"
	initialResource := domain.InitialGenerationDeliveryResource(realmID, requestID)
	fixture.GrantExact(
		t, []identityaccess.Action{domain.ActionIssueDelivery},
		domain.ResourceKindGenerationDelivery, initialResource, false,
	)
	issued, err := fixture.Client.IssueInitialGeneration(
		ctx, connect.NewRequest(&protocol.IssueInitialGenerationRequest{
			Version: domain.ContractVersion, RequestId: requestID, RealmId: realmID,
			ChannelClass: string(identityapi.CapabilityRealmDiscovery),
			Permissions: uint32(identityapi.CapabilityPublish | identityapi.CapabilitySubscribe |
				identityapi.CapabilityStoreFetch),
			RecipientAttestation: prepared.Msg.GetAttestation(),
			ValidForSeconds:      uint64(time.Hour / time.Second),
		}),
	)
	require.NoError(t, err)
	require.Equal(t, uint64(2), issued.Msg.GetAuthoritySequence())
	require.Equal(t, uint32(1), issued.Msg.GetGeneration())

	deliveryResource, valid := domain.GenerationDeliveryResource(
		realmID, issued.Msg.GetOperationId(), issued.Msg.GetDeliveryId(),
	)
	require.True(t, valid)
	fixture.GrantExact(
		t, []identityaccess.Action{
			"realm.channel.delivery.install",
			domain.ActionAcknowledgeDelivery,
		},
		domain.ResourceKindGenerationDelivery, deliveryResource, false,
	)
	installed, err := fixture.Client.InstallGenerationDelivery(
		ctx, connect.NewRequest(&protocol.InstallGenerationDeliveryRequest{
			Version: channeldelivery.ContractVersion, Sealed: issued.Msg.GetSealed(),
		}),
	)
	require.NoError(t, err)
	require.Equal(t, "installed", installed.Msg.GetReceipt().GetPhase())

	acknowledged, err := fixture.Client.AcknowledgeInitialGeneration(
		ctx, connect.NewRequest(&protocol.AcknowledgeInitialGenerationRequest{
			Version: domain.ContractVersion, RealmId: realmID,
			OperationId: issued.Msg.GetOperationId(), Receipt: installed.Msg.GetReceipt(),
		}),
	)
	require.NoError(t, err)
	require.Equal(t, uint64(3), acknowledged.Msg.GetAuthoritySequence())
	require.Equal(t, domain.DeliveryPhaseInstalled, acknowledged.Msg.GetPhase())
	require.Equal(t, uint32(1), authorityService.Readiness().CurrentGeneration)
}

func TestChannelRotationActivationAcrossProtectedAuthorityAndMemberHosts(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer: testkit.LayerIntegration, Domain: "realm-authority", ScenarioID: "CGA-03",
		Suite: "integration", Tags: []string{"integration", "authority", "security", "rotation", "multi-host"},
		Speed: "fast", Environment: "local",
	})
	ctx := context.Background()
	root := t.TempDir()
	now := time.Now().UTC().Truncate(time.Second)
	clock := func() time.Time { return now }
	storePath := filepath.Join(root, "authority", "realm-authority.db")
	checkpointPath := filepath.Join(root, "independent-checkpoints")
	require.NoError(t, storage.EnsurePrivateDir(filepath.Dir(storePath)))
	require.NoError(t, storage.EnsurePrivateDir(checkpointPath))
	store, err := domain.OpenFileStore(
		ctx, storePath, bytes.Repeat([]byte{0xb1}, domain.AuthorityStoreKeyBytes),
	)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	repository, err := domain.NewFileCheckpointRepository(checkpointPath)
	require.NoError(t, err)
	authoritySigner := integrationAuthoritySigner(t, 0xb2)
	authorityService := domain.New(domain.Config{
		Store: store, Signer: authoritySigner, Repository: repository,
		Random: cryptorand.Reader, Clock: clock, Policy: integrationAuthorityPolicy{},
	})

	memberPrivate := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0xb5}, ed25519.SeedSize))
	memberPrincipal, err := identityprincipal.FromEd25519PublicKey(
		memberPrivate.Public().(ed25519.PublicKey),
	)
	require.NoError(t, err)
	trust, err := identitytrust.NewRegistry([]identitytrust.Entry{{
		Principal: authoritySigner.principal,
		PublicKey: authoritySigner.private.Public().(ed25519.PublicKey),
		Purposes:  []identitytrust.Purpose{identitytrust.PurposeChannelIssue},
	}})
	require.NoError(t, err)
	memberCapabilities, err := identitycapability.NewService(
		filepath.Join(root, "member", "capabilities.db"),
		bytes.Repeat([]byte{0xb6}, 32), memberPrincipal.String(), trust,
		integrationCapabilityPolicy{}, clock,
	)
	require.NoError(t, err)
	memberDelivery, err := channeldelivery.New(
		memberCapabilities, memberPrivate, memberPrincipal.String(), clock,
	)
	require.NoError(t, err)

	authorityRuntime := testkit.StartRuntime(t, runtimeinfra.Config{
		Name: "rotation-authority-host",
		Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.DataConfig{Dir: filepath.Join(root, "authority-node")},
	}).Runtime
	memberRuntime := testkit.StartRuntime(t, runtimeinfra.Config{
		Name: "rotation-member-host",
		Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.DataConfig{Dir: filepath.Join(root, "member-node")},
	}).Runtime
	authorityHost := testkit.NewAuthorityDeliveryOperatorCLIFixture(
		t, authorityRuntime, authorityService, nil,
	)
	memberHost := testkit.NewAuthorityDeliveryOperatorCLIFixture(
		t, memberRuntime, nil, memberDelivery,
	)
	created, err := authorityHost.Client.CreateRealmAuthority(
		ctx, connect.NewRequest(&protocol.CreateRealmAuthorityRequest{
			Version: domain.ContractVersion, RequestId: "rotation-genesis",
			RealmClass: domain.RealmClassProduction,
		}),
	)
	require.NoError(t, err)
	realmID := created.Msg.GetAuthority().GetRealmId()

	memberHost.GrantExact(
		t, []identityaccess.Action{"realm.channel.delivery.prepare"},
		identityaccess.ResourceKind("principal"), memberPrincipal.String(), false,
	)
	prepared, err := memberHost.Client.PrepareGenerationDelivery(
		ctx, connect.NewRequest(&protocol.PrepareGenerationDeliveryRequest{
			Version: channeldelivery.ContractVersion, SubjectPrincipal: memberPrincipal.String(),
			ValidForSeconds: uint64(time.Hour / time.Second),
		}),
	)
	require.NoError(t, err)
	initialResource := domain.InitialGenerationDeliveryResource(realmID, "rotation-initial")
	authorityHost.GrantExact(
		t, []identityaccess.Action{domain.ActionIssueDelivery},
		domain.ResourceKindGenerationDelivery, initialResource, false,
	)
	issued, err := authorityHost.Client.IssueInitialGeneration(
		ctx, connect.NewRequest(&protocol.IssueInitialGenerationRequest{
			Version: domain.ContractVersion, RequestId: "rotation-initial", RealmId: realmID,
			ChannelClass: string(identityapi.CapabilityRealmDiscovery),
			Permissions: uint32(identityapi.CapabilityPublish | identityapi.CapabilitySubscribe |
				identityapi.CapabilityStoreFetch),
			RecipientAttestation: prepared.Msg.GetAttestation(),
			ValidForSeconds:      uint64(time.Hour / time.Second),
		}),
	)
	require.NoError(t, err)
	initialDeliveryResource, valid := domain.GenerationDeliveryResource(
		realmID, issued.Msg.GetOperationId(), issued.Msg.GetDeliveryId(),
	)
	require.True(t, valid)
	memberHost.GrantExact(
		t, []identityaccess.Action{"realm.channel.delivery.install"},
		domain.ResourceKindGenerationDelivery, initialDeliveryResource, false,
	)
	installed, err := memberHost.Client.InstallGenerationDelivery(
		ctx, connect.NewRequest(&protocol.InstallGenerationDeliveryRequest{
			Version: channeldelivery.ContractVersion, Sealed: issued.Msg.GetSealed(),
		}),
	)
	require.NoError(t, err)
	authorityHost.GrantExact(
		t, []identityaccess.Action{domain.ActionAcknowledgeDelivery},
		domain.ResourceKindGenerationDelivery, initialDeliveryResource, false,
	)
	_, err = authorityHost.Client.AcknowledgeInitialGeneration(
		ctx, connect.NewRequest(&protocol.AcknowledgeInitialGenerationRequest{
			Version: domain.ContractVersion, RealmId: realmID,
			OperationId: issued.Msg.GetOperationId(), Receipt: installed.Msg.GetReceipt(),
		}),
	)
	require.NoError(t, err)

	channelID := issued.Msg.GetChannelId()
	var channel [16]byte
	copy(channel[:], channelID)
	channelResource := domain.ChannelResource(realmID, channel)
	authorityHost.GrantExact(
		t, []identityaccess.Action{domain.ActionRotateGeneration},
		domain.ResourceKindChannel, channelResource, false,
	)
	rotated, err := authorityHost.Client.RotateChannel(
		ctx, connect.NewRequest(&protocol.RotateChannelRequest{
			Version: domain.ContractVersion, RequestId: "rotation-001", RealmId: realmID,
			ChannelId: channelID,
			RecipientAttestations: []*protocol.GenerationDeliveryAttestation{
				prepared.Msg.GetAttestation(),
			},
			ValidForSeconds: uint64(time.Hour / time.Second),
			DrainForSeconds: uint64((15 * time.Minute) / time.Second),
		}),
	)
	require.NoError(t, err)
	require.Equal(t, uint32(1), rotated.Msg.GetPreviousGeneration())
	require.Equal(t, uint32(2), rotated.Msg.GetPendingGeneration())
	require.Len(t, rotated.Msg.GetDeliveries(), 1)
	require.True(t, memberCapabilities.GenerationReadiness(channel).Ready)

	rotationDelivery := rotated.Msg.GetDeliveries()[0]
	rotationDeliveryResource, valid := domain.GenerationDeliveryResource(
		realmID, rotated.Msg.GetOperationId(), rotationDelivery.GetDeliveryId(),
	)
	require.True(t, valid)
	memberHost.GrantExact(
		t, []identityaccess.Action{"realm.channel.delivery.install"},
		domain.ResourceKindGenerationDelivery, rotationDeliveryResource, false,
	)
	pending, err := memberHost.Client.InstallGenerationDelivery(
		ctx, connect.NewRequest(&protocol.InstallGenerationDeliveryRequest{
			Version: channeldelivery.ContractVersion, Sealed: rotationDelivery.GetSealed(),
		}),
	)
	require.NoError(t, err)
	require.False(t, memberCapabilities.GenerationReadiness(channel).Ready)
	require.Equal(t, uint32(2), memberCapabilities.GenerationReadiness(channel).PendingGeneration)
	authorityHost.GrantExact(
		t, []identityaccess.Action{domain.ActionAcknowledgeDelivery},
		domain.ResourceKindGenerationDelivery, rotationDeliveryResource, false,
	)
	_, err = authorityHost.Client.AcknowledgeInitialGeneration(
		ctx, connect.NewRequest(&protocol.AcknowledgeInitialGenerationRequest{
			Version: domain.ContractVersion, RealmId: realmID,
			OperationId: rotated.Msg.GetOperationId(), Receipt: pending.Msg.GetReceipt(),
		}),
	)
	require.NoError(t, err)

	operationResource := domain.OperationResource(realmID, rotated.Msg.GetOperationId())
	authorityHost.GrantExact(
		t, []identityaccess.Action{domain.ActionCommitActivation},
		domain.ResourceKindOperation, operationResource, false,
	)
	committed, err := authorityHost.Client.CommitChannelActivation(
		ctx, connect.NewRequest(&protocol.CommitChannelActivationRequest{
			Version: domain.ContractVersion, RealmId: realmID,
			OperationId: rotated.Msg.GetOperationId(),
		}),
	)
	require.NoError(t, err)
	require.False(t, memberCapabilities.GenerationReadiness(channel).Ready)
	memberHost.GrantExact(
		t, []identityaccess.Action{"realm.channel.generation.activate"},
		domain.ResourceKindOperation, operationResource, false,
	)
	active, err := memberHost.Client.ActivateGeneration(
		ctx, connect.NewRequest(&protocol.ActivateGenerationRequest{
			Version: domain.ContractVersion, Activation: committed.Msg.GetActivation(),
		}),
	)
	require.NoError(t, err)
	readiness := memberCapabilities.GenerationReadiness(channel)
	require.True(t, readiness.Ready)
	require.Equal(t, uint32(2), readiness.CurrentGeneration)
	require.Equal(t, uint32(1), readiness.PreviousGeneration)

	_, err = authorityHost.Client.AcknowledgeChannelActivation(
		ctx, connect.NewRequest(&protocol.AcknowledgeChannelActivationRequest{
			Version: domain.ContractVersion, RealmId: realmID,
			OperationId: rotated.Msg.GetOperationId(), ApprovedHost: true,
			Receipt: active.Msg.GetReceipt(),
		}),
	)
	require.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
	authorityHost.GrantExact(
		t, []identityaccess.Action{domain.ActionAcknowledgeActivation},
		domain.ResourceKindGenerationDelivery, rotationDeliveryResource, false,
	)
	_, err = authorityHost.Client.AcknowledgeChannelActivation(
		ctx, connect.NewRequest(&protocol.AcknowledgeChannelActivationRequest{
			Version: domain.ContractVersion, RealmId: realmID,
			OperationId: rotated.Msg.GetOperationId(), ApprovedHost: false,
			Receipt: active.Msg.GetReceipt(),
		}),
	)
	require.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
	completed, err := authorityHost.Client.AcknowledgeChannelActivation(
		ctx, connect.NewRequest(&protocol.AcknowledgeChannelActivationRequest{
			Version: domain.ContractVersion, RealmId: realmID,
			OperationId: rotated.Msg.GetOperationId(), ApprovedHost: true,
			Receipt: active.Msg.GetReceipt(),
		}),
	)
	require.NoError(t, err)
	require.Equal(t, domain.DeliveryPhaseCompleted, completed.Msg.GetPhase())
	require.Equal(t, uint32(2), completed.Msg.GetCurrentGeneration())
	require.Equal(t, uint32(1), completed.Msg.GetPreviousGeneration())

	authorityHost.GrantExact(
		t, []identityaccess.Action{domain.ActionInspect},
		domain.ResourceKindChannel, channelResource, false,
	)
	channelStatus, err := authorityHost.Client.InspectChannel(
		ctx, connect.NewRequest(&protocol.InspectChannelRequest{
			Version: domain.ContractVersion, RealmId: realmID, ChannelId: channelID,
		}),
	)
	require.NoError(t, err)
	require.Equal(
		t, string(identityapi.CapabilityRealmDiscovery),
		channelStatus.Msg.GetChannel().GetChannelClass(),
	)
	require.Equal(t, domain.ReasonChannelGrantRenewalDue, channelStatus.Msg.GetChannel().GetReason())

	renewed, err := authorityHost.Client.RenewChannelGrants(
		ctx, connect.NewRequest(&protocol.RenewChannelGrantsRequest{
			Version: domain.ContractVersion, RequestId: "renewal-integration-001",
			RealmId: realmID, ChannelId: channelID,
			RecipientAttestations: []*protocol.GenerationDeliveryAttestation{
				prepared.Msg.GetAttestation(),
			},
			DrainForSeconds: uint64((15 * time.Minute) / time.Second),
		}),
	)
	require.NoError(t, err)
	require.True(t, renewed.Msg.GetRenewal())
	require.Equal(
		t, string(identityapi.CapabilityRealmDiscovery), renewed.Msg.GetChannelClass(),
	)
	require.Equal(t, uint32(3), renewed.Msg.GetPendingGeneration())
	require.Len(t, renewed.Msg.GetDeliveries(), 1)
	renewalDelivery := renewed.Msg.GetDeliveries()[0]
	renewalDeliveryResource, valid := domain.GenerationDeliveryResource(
		realmID, renewed.Msg.GetOperationId(), renewalDelivery.GetDeliveryId(),
	)
	require.True(t, valid)
	memberHost.GrantExact(
		t, []identityaccess.Action{"realm.channel.delivery.install"},
		domain.ResourceKindGenerationDelivery, renewalDeliveryResource, false,
	)
	renewalPending, err := memberHost.Client.InstallGenerationDelivery(
		ctx, connect.NewRequest(&protocol.InstallGenerationDeliveryRequest{
			Version: channeldelivery.ContractVersion, Sealed: renewalDelivery.GetSealed(),
		}),
	)
	require.NoError(t, err)
	require.False(t, memberCapabilities.GenerationReadiness(channel).Ready)
	require.Equal(t, uint32(3), memberCapabilities.GenerationReadiness(channel).PendingGeneration)
	authorityHost.GrantExact(
		t, []identityaccess.Action{domain.ActionAcknowledgeDelivery},
		domain.ResourceKindGenerationDelivery, renewalDeliveryResource, false,
	)
	_, err = authorityHost.Client.AcknowledgeInitialGeneration(
		ctx, connect.NewRequest(&protocol.AcknowledgeInitialGenerationRequest{
			Version: domain.ContractVersion, RealmId: realmID,
			OperationId: renewed.Msg.GetOperationId(), Receipt: renewalPending.Msg.GetReceipt(),
		}),
	)
	require.NoError(t, err)
	renewalOperationResource := domain.OperationResource(realmID, renewed.Msg.GetOperationId())
	authorityHost.GrantExact(
		t, []identityaccess.Action{domain.ActionCommitActivation},
		domain.ResourceKindOperation, renewalOperationResource, false,
	)
	renewalCommitted, err := authorityHost.Client.CommitChannelActivation(
		ctx, connect.NewRequest(&protocol.CommitChannelActivationRequest{
			Version: domain.ContractVersion, RealmId: realmID,
			OperationId: renewed.Msg.GetOperationId(),
		}),
	)
	require.NoError(t, err)
	memberHost.GrantExact(
		t, []identityaccess.Action{"realm.channel.generation.activate"},
		domain.ResourceKindOperation, renewalOperationResource, false,
	)
	renewalActive, err := memberHost.Client.ActivateGeneration(
		ctx, connect.NewRequest(&protocol.ActivateGenerationRequest{
			Version: domain.ContractVersion, Activation: renewalCommitted.Msg.GetActivation(),
		}),
	)
	require.NoError(t, err)
	require.True(t, memberCapabilities.GenerationReadiness(channel).Ready)
	require.Equal(t, uint32(3), memberCapabilities.GenerationReadiness(channel).CurrentGeneration)
	authorityHost.GrantExact(
		t, []identityaccess.Action{domain.ActionAcknowledgeActivation},
		domain.ResourceKindGenerationDelivery, renewalDeliveryResource, false,
	)
	renewalCompleted, err := authorityHost.Client.AcknowledgeChannelActivation(
		ctx, connect.NewRequest(&protocol.AcknowledgeChannelActivationRequest{
			Version: domain.ContractVersion, RealmId: realmID,
			OperationId: renewed.Msg.GetOperationId(), ApprovedHost: true,
			Receipt: renewalActive.Msg.GetReceipt(),
		}),
	)
	require.NoError(t, err)
	require.Equal(t, domain.DeliveryPhaseCompleted, renewalCompleted.Msg.GetPhase())
	require.Equal(t, uint32(3), renewalCompleted.Msg.GetCurrentGeneration())
}

func TestRealmAuthorityCrashBoundariesResumeRealPersistence(t *testing.T) {
	for _, boundary := range []domain.CrashBoundary{
		domain.CrashAfterLedgerCommit,
		domain.CrashAfterCheckpointCreate,
	} {
		t.Run(string(boundary), func(t *testing.T) {
			ctx := context.Background()
			root := t.TempDir()
			key := bytes.Repeat([]byte{0x91}, domain.AuthorityStoreKeyBytes)
			storePath := filepath.Join(root, "authority", "realm-authority.db")
			checkpointPath := filepath.Join(root, "independent-checkpoints")
			require.NoError(t, storage.EnsurePrivateDir(filepath.Dir(storePath)))
			require.NoError(t, storage.EnsurePrivateDir(checkpointPath))
			store, err := domain.OpenFileStore(ctx, storePath, key)
			require.NoError(t, err)
			repository, err := domain.NewFileCheckpointRepository(checkpointPath)
			require.NoError(t, err)
			signer := integrationAuthoritySigner(t, 0x92)
			crashed := false
			service := domain.New(domain.Config{
				Store: store, Signer: signer, Repository: repository,
				Random: bytes.NewReader(bytes.Repeat([]byte{0x93}, 1024)),
				Clock:  func() time.Time { return time.Date(2026, 7, 27, 14, 0, 0, 0, time.UTC) },
				Policy: integrationAuthorityPolicy{},
				Crash: func(at domain.CrashBoundary) error {
					if !crashed && at == boundary {
						crashed = true
						return errors.New("injected process crash")
					}
					return nil
				},
			})
			command := domain.Command{
				Actor: "operator", Effective: "operator", Action: domain.ActionCreate,
				ResourceKind: domain.ResourceKindAuthorityInstance, ResourceID: domain.PrimaryAuthorityInstance,
			}
			request := domain.CreateRequest{
				Version: domain.ContractVersion, RequestID: "crash-request-001",
				RealmClass: domain.RealmClassProduction,
			}
			_, err = service.CreateOrReopen(ctx, command, request)
			require.ErrorIs(t, err, domain.ErrUnavailable)
			before, found, err := store.Load(ctx)
			require.NoError(t, err)
			require.True(t, found)
			require.Equal(t, domain.PhaseCheckpointing, before.Phase)
			require.Len(t, before.Operations, 1)
			operationID := before.Operations[0].ID
			require.NoError(t, store.Close())

			reopened, err := domain.OpenFileStore(ctx, storePath, key)
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, reopened.Close()) })
			restarted := domain.New(domain.Config{
				Store: reopened, Signer: signer, Repository: repository,
				Random: bytes.NewReader(bytes.Repeat([]byte{0x94}, 1024)),
				Clock:  func() time.Time { return time.Date(2026, 7, 27, 14, 0, 0, 0, time.UTC) },
				Policy: integrationAuthorityPolicy{},
			})
			result, err := restarted.CreateOrReopen(ctx, command, request)
			require.NoError(t, err)
			require.Equal(t, operationID, result.OperationID)
			require.Equal(t, domain.PhaseReady, result.Phase)
			head, found, err := repository.ReadHead(ctx, result.RealmID)
			require.NoError(t, err)
			require.True(t, found)
			require.Equal(t, uint64(1), head.AuthoritySequence)
			require.Equal(t, result.CheckpointDigest, head.Digest)
		})
	}
}

type integrationAuthorityPolicy struct{}

func (integrationAuthorityPolicy) AdmitRealmGenesis(context.Context, domain.Command) error {
	return nil
}
func (integrationAuthorityPolicy) AdmitInitialGeneration(context.Context, domain.Command) error {
	return nil
}
func (integrationAuthorityPolicy) AdmitChannelRotation(context.Context, domain.Command) error {
	return nil
}
func (integrationAuthorityPolicy) AdmitChannelMembership(context.Context, domain.Command) error {
	return nil
}
func (integrationAuthorityPolicy) AdmitChannelClass(context.Context, domain.Command, identityapi.CapabilityScope) error {
	return nil
}
func (integrationAuthorityPolicy) AdmitAuthorityRecovery(context.Context, domain.Command) error {
	return nil
}

type integrationCapabilityPolicy struct{}

func (integrationCapabilityPolicy) AllowCapabilityUse(identityapi.CapabilityUse) error {
	return nil
}

type integrationAuthorityAudit struct{ records []domain.AuditRecord }

func (a *integrationAuthorityAudit) RecordAuthorityAudit(_ context.Context, record domain.AuditRecord) error {
	a.records = append(a.records, record)
	return nil
}

type integrationSigner struct {
	private   ed25519.PrivateKey
	principal string
}

func integrationAuthoritySigner(t *testing.T, seed byte) *integrationSigner {
	t.Helper()
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{seed}, ed25519.SeedSize))
	principal, err := identityprincipal.FromEd25519PublicKey(private.Public().(ed25519.PublicKey))
	require.NoError(t, err)
	return &integrationSigner{private: private, principal: principal.String()}
}

func (s *integrationSigner) Principal(context.Context) (string, error) { return s.principal, nil }
func (s *integrationSigner) PublicKey(context.Context) (ed25519.PublicKey, error) {
	return append(ed25519.PublicKey(nil), s.private.Public().(ed25519.PublicKey)...), nil
}
func (s *integrationSigner) Sign(_ context.Context, message []byte) ([]byte, error) {
	return ed25519.Sign(s.private, message), nil
}

func authorityErrorCode(t *testing.T, err error) string {
	t.Helper()
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	for _, detail := range connectErr.Details() {
		value, detailErr := detail.Value()
		require.NoError(t, detailErr)
		if public, ok := value.(*protocol.Error); ok {
			return public.GetCode()
		}
	}
	require.FailNow(t, "connect error has no stable public Error detail")
	return ""
}
