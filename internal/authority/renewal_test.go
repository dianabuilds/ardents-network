package authority

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"path/filepath"
	"testing"
	"time"

	identityapi "ardents/internal/identity"
	"ardents/internal/messaging"

	"github.com/stretchr/testify/require"
)

func TestAuthorityCreatesStrictlySeparatedChannelClasses(t *testing.T) {
	ctx := context.Background()
	fixture := newServiceFixture(t)
	fixture.service.random = rand.Reader
	genesis, err := fixture.service.CreateOrReopen(
		ctx, fixture.createCommand(),
		CreateRequest{
			Version: ContractVersion, RequestID: "class-separation-genesis",
			RealmClass: RealmClassProduction,
		},
	)
	require.NoError(t, err)
	_, principal, attestation := newMembershipMember(t, fixture, 0x91)
	classes := []identityapi.CapabilityScope{
		identityapi.CapabilityRealmDiscovery,
		identityapi.CapabilityDataExchange,
		identityapi.CapabilityApplication,
		identityapi.CapabilityControl,
	}
	results := make([]InitialGenerationResult, 0, len(classes))
	for index, class := range classes {
		requestID := "class-separation-" + string(rune('a'+index))
		result, issueErr := fixture.service.IssueInitialGeneration(
			ctx,
			Command{
				Actor: "operator", Effective: "operator", Action: ActionIssueDelivery,
				ResourceKind: ResourceKindGenerationDelivery,
				ResourceID:   InitialGenerationDeliveryResource(genesis.RealmID, requestID),
			},
			InitialGenerationRequest{
				Version: ContractVersion, RequestID: requestID, RealmID: genesis.RealmID,
				ChannelClass: class,
				Permissions: identityapi.CapabilityPublish |
					identityapi.CapabilitySubscribe | identityapi.CapabilityStoreFetch,
				RecipientAttestation: attestation, ValidFor: GrantValidity,
			},
		)
		require.NoError(t, issueErr)
		results = append(results, result)
	}
	require.Len(t, fixture.store.state.Channels, len(classes))
	require.Len(t, fixture.store.state.Members, 1)
	require.Equal(t, principal, fixture.store.state.Members[0].Principal)
	ids := map[[16]byte]struct{}{}
	secrets := map[string]struct{}{}
	selectors := map[string]struct{}{}
	envelopeKeys := map[string]struct{}{}
	replay, err := messaging.NewDurableReplayLedger(
		filepath.Join(t.TempDir(), "class-replay.db"), bytes.Repeat([]byte{0x99}, 32),
		8, 32,
	)
	require.NoError(t, err)
	for index, channel := range fixture.store.state.Channels {
		require.Equal(t, string(classes[index]), channel.Class)
		require.Equal(t, uint32(1), channel.CurrentGeneration)
		require.NotEqual(t, [16]byte{}, channel.ID)
		_, duplicateID := ids[channel.ID]
		require.False(t, duplicateID)
		ids[channel.ID] = struct{}{}
		secret := string(channel.CurrentGrants[0].Secret)
		_, duplicateSecret := secrets[secret]
		require.False(t, duplicateSecret)
		secrets[secret] = struct{}{}
		grant, ok := channel.CurrentGrants[0].restore()
		require.True(t, ok)
		ref := identityapi.CapabilityRef("cap_class_" + string(rune('a'+index)))
		material, deriveErr := messaging.Derive(identityapi.ResolvedCapability{
			Ref: ref, ChannelID: grant.ChannelID, Generation: grant.Generation,
			GrantID: grant.GrantID, Subject: grant.SubjectPrincipal,
			Permissions: grant.Permissions, Scope: grant.Scope, Secret: grant.Secret,
		})
		require.NoError(t, deriveErr)
		_, duplicateSelector := selectors[material.ContentTopic]
		require.False(t, duplicateSelector)
		selectors[material.ContentTopic] = struct{}{}
		envelopeKey := string(material.EnvelopeKey())
		_, duplicateKey := envelopeKeys[envelopeKey]
		require.False(t, duplicateKey)
		envelopeKeys[envelopeKey] = struct{}{}
		require.NoError(t, replay.Admit(messaging.ReplayUse{
			CapabilityRef: ref, Generation: 1, MessageID: [16]byte{0x01},
			ExpiresAt: fixture.clock().Add(time.Hour), Now: fixture.clock(),
		}))
		status, statusErr := fixture.service.InspectChannel(
			ctx,
			Command{
				Actor: "operator", Effective: "operator", Action: ActionInspect,
				ResourceKind: ResourceKindChannel,
				ResourceID:   ChannelResource(genesis.RealmID, results[index].ChannelID),
			},
			InspectChannelRequest{
				Version: ContractVersion, RealmID: genesis.RealmID,
				ChannelID: results[index].ChannelID,
			},
		)
		require.NoError(t, statusErr)
		require.Equal(t, classes[index], status.ChannelClass)
		require.Equal(t, ReadinessReady, status.Readiness)
	}
	for _, audit := range fixture.store.state.AuditLog[1:] {
		require.NotEmpty(t, audit.ChannelClass)
		require.Equal(t, uint32(1), audit.Generation)
	}
	rotated, err := fixture.service.RotateChannel(
		ctx, renewalCommand(genesis.RealmID, results[0].ChannelID),
		RotationRequest{
			Version: ContractVersion, RequestID: "class-separation-rotate-discovery",
			RealmID: genesis.RealmID, ChannelID: results[0].ChannelID,
			RecipientAttestations: []identityapi.CapabilityDeliveryAttestation{attestation},
			ValidFor:              time.Hour, DrainFor: 15 * time.Minute,
		},
	)
	require.NoError(t, err)
	require.Equal(t, uint32(2), rotated.PendingGeneration)
	require.Equal(t, uint32(1), fixture.store.state.Channels[0].PendingGenerationCount)
	for _, sibling := range fixture.store.state.Channels[1:] {
		require.Equal(t, uint32(1), sibling.CurrentGeneration)
		require.Zero(t, sibling.PendingGenerationCount)
	}
}

func TestApplicationChannelConsumesProductPolicyWithoutConversationSemantics(t *testing.T) {
	ctx := context.Background()
	fixture := newServiceFixture(t)
	genesis, err := fixture.service.CreateOrReopen(
		ctx, fixture.createCommand(),
		CreateRequest{
			Version: ContractVersion, RequestID: "application-policy-genesis",
			RealmClass: RealmClassProduction,
		},
	)
	require.NoError(t, err)
	_, _, attestation := newMembershipMember(t, fixture, 0x92)
	fixture.service = New(Config{
		Store: fixture.store, Signer: fixture.signer, Repository: fixture.repository,
		Random: rand.Reader, Clock: fixture.clock, Policy: denyApplicationClassPolicy{},
	})
	issue := func(
		requestID string,
		class identityapi.CapabilityScope,
		recipient identityapi.CapabilityDeliveryAttestation,
	) (InitialGenerationResult, error) {
		return fixture.service.IssueInitialGeneration(
			ctx,
			Command{
				Actor: "operator", Effective: "operator", Action: ActionIssueDelivery,
				ResourceKind: ResourceKindGenerationDelivery,
				ResourceID:   InitialGenerationDeliveryResource(genesis.RealmID, requestID),
			},
			InitialGenerationRequest{
				Version: ContractVersion, RequestID: requestID, RealmID: genesis.RealmID,
				ChannelClass:         class,
				Permissions:          identityapi.CapabilityPublish | identityapi.CapabilitySubscribe,
				RecipientAttestation: recipient, ValidFor: GrantValidity,
			},
		)
	}
	forged := attestation
	forged.Signature = nil
	_, err = issue(
		"application-policy-denied", identityapi.CapabilityApplication, forged,
	)
	require.ErrorIs(t, err, ErrPermissionDenied)
	require.Empty(t, fixture.store.state.Channels)
	allowed, err := issue(
		"application-policy-discovery", identityapi.CapabilityRealmDiscovery, attestation,
	)
	require.NoError(t, err)
	require.NotEqual(t, [16]byte{}, allowed.ChannelID)
	require.Len(t, fixture.store.state.Channels, 1)
}

func TestRenewalUsesFreshGenerationAndIsIdempotentInsideThreshold(t *testing.T) {
	ctx := context.Background()
	fixture := newServiceFixture(t)
	fixture.service.random = rand.Reader
	genesis, err := fixture.service.CreateOrReopen(
		ctx, fixture.createCommand(),
		CreateRequest{
			Version: ContractVersion, RequestID: "renewal-genesis",
			RealmClass: RealmClassProduction,
		},
	)
	require.NoError(t, err)
	member, principal, attestation := newMembershipMember(t, fixture, 0x93)
	initial := issueAndInstallMembershipInitial(
		t, ctx, fixture, genesis.RealmID, member, attestation,
	)
	current := fixture.store.state.Channels[0].CurrentGrants[0]
	result, err := fixture.service.RenewChannelGrants(
		ctx, renewalCommand(genesis.RealmID, initial.ChannelID),
		RenewalRequest{
			Version: ContractVersion, RequestID: "renewal-one",
			RealmID: genesis.RealmID, ChannelID: initial.ChannelID,
			RecipientAttestations: []identityapi.CapabilityDeliveryAttestation{attestation},
			DrainFor:              15 * time.Minute,
		},
	)
	require.NoError(t, err)
	require.True(t, result.Renewal)
	require.Equal(t, identityapi.CapabilityRealmDiscovery, result.ChannelClass)
	require.Equal(t, uint32(2), result.PendingGeneration)
	require.Len(t, result.Deliveries, 1)
	pending := fixture.store.state.Channels[0].PendingGrants[0]
	require.NotEqual(t, current.GrantID, pending.GrantID)
	require.False(t, bytes.Equal(current.Secret, pending.Secret))
	require.Equal(t, fixture.clock().Add(GrantValidity), pending.NotAfter)
	require.Equal(t, principal, pending.SubjectPrincipal)
	restartMembershipAuthority(fixture)
	replayed, err := fixture.service.RenewChannelGrants(
		ctx, renewalCommand(genesis.RealmID, initial.ChannelID),
		RenewalRequest{
			Version: ContractVersion, RequestID: "renewal-one",
			RealmID: genesis.RealmID, ChannelID: initial.ChannelID,
			RecipientAttestations: []identityapi.CapabilityDeliveryAttestation{attestation},
			DrainFor:              15 * time.Minute,
		},
	)
	require.NoError(t, err)
	require.Equal(t, result, replayed)
	require.Len(t, fixture.store.state.Rotations, 1)
	require.True(t, fixture.store.state.Rotations[0].Renewal)
	require.Equal(t, "channel_renewal", fixture.store.state.Operations[1].Kind)
}

func TestRenewalFailsClosedBeforeThresholdAndChannelExpiryIsIsolated(t *testing.T) {
	ctx := context.Background()
	fixture := newServiceFixture(t)
	fixture.service.random = rand.Reader
	genesis, err := fixture.service.CreateOrReopen(
		ctx, fixture.createCommand(),
		CreateRequest{
			Version: ContractVersion, RequestID: "renewal-bound-genesis",
			RealmClass: RealmClassProduction,
		},
	)
	require.NoError(t, err)
	_, _, attestation := newMembershipMember(t, fixture, 0x95)
	first, err := fixture.service.IssueInitialGeneration(
		ctx,
		Command{
			Actor: "operator", Effective: "operator", Action: ActionIssueDelivery,
			ResourceKind: ResourceKindGenerationDelivery,
			ResourceID:   InitialGenerationDeliveryResource(genesis.RealmID, "renewal-long"),
		},
		InitialGenerationRequest{
			Version: ContractVersion, RequestID: "renewal-long", RealmID: genesis.RealmID,
			ChannelClass:         identityapi.CapabilityRealmDiscovery,
			Permissions:          identityapi.CapabilityPublish | identityapi.CapabilitySubscribe,
			RecipientAttestation: attestation, ValidFor: GrantValidity,
		},
	)
	require.NoError(t, err)
	_, err = fixture.service.RenewChannelGrants(
		ctx, renewalCommand(genesis.RealmID, first.ChannelID),
		RenewalRequest{
			Version: ContractVersion, RequestID: "renewal-too-early",
			RealmID: genesis.RealmID, ChannelID: first.ChannelID,
			RecipientAttestations: []identityapi.CapabilityDeliveryAttestation{attestation},
			DrainFor:              15 * time.Minute,
		},
	)
	require.ErrorIs(t, err, ErrConflict)
	require.Empty(t, fixture.store.state.Rotations)

	expiring := fixture.store.state.Channels[0]
	healthy := expiring
	healthy.ID = [16]byte{0x77}
	healthy.Class = string(identityapi.CapabilityDataExchange)
	healthy.Grant.ChannelID = healthy.ID
	healthy.CurrentGrants = cloneGrantRecords(expiring.CurrentGrants)
	healthy.CurrentGrants[0].ChannelID = healthy.ID
	healthy.CurrentGrants[0].Scope = identityapi.CapabilityDataExchange
	healthy.CurrentGrants[0].NotAfter = fixture.clock().Add(GrantValidity)
	fixture.store.state.Channels[0].CurrentGrants[0].NotAfter = fixture.clock()
	expired := channelStatusAt(fixture.store.state, fixture.store.state.Channels[0], fixture.clock())
	ready := channelStatusAt(fixture.store.state, healthy, fixture.clock())
	require.Equal(t, ReadinessUnavailable, expired.Readiness)
	require.Equal(t, ReasonChannelGrantExpired, expired.Reason)
	require.Equal(t, ReadinessReady, ready.Readiness)
	require.Empty(t, ready.Reason)
}

func renewalCommand(realmID string, channelID [16]byte) Command {
	return Command{
		Actor: "operator", Effective: "operator", Action: ActionRotateGeneration,
		ResourceKind: ResourceKindChannel, ResourceID: ChannelResource(realmID, channelID),
	}
}

type denyApplicationClassPolicy struct{ allowPolicy }

func (denyApplicationClassPolicy) AdmitChannelClass(
	_ context.Context,
	_ Command,
	scope identityapi.CapabilityScope,
) error {
	if scope == identityapi.CapabilityApplication {
		return errors.New("application channel denied")
	}
	return nil
}
