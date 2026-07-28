package capability

import (
	"bytes"
	"crypto/ed25519"
	"path/filepath"
	"testing"
	"time"

	identityapi "ardents/internal/identity"
	"ardents/internal/messaging"

	"github.com/stretchr/testify/require"
)

func TestPendingGenerationRequiresSignedActivationBeforeItBecomesReady(t *testing.T) {
	now := capabilityTestNow
	current, issuerPublic, issuerPrivate := signedTestGrant(t, 1)
	storePath := filepath.Join(t.TempDir(), "member-capabilities.db")
	storeKey := bytes.Repeat([]byte{0xc1}, 32)
	member, err := NewService(
		storePath, storeKey, current.SubjectPrincipal, trustedIssuer(issuerPublic),
		allowCapabilityAdmission{}, func() time.Time { return now },
	)
	require.NoError(t, err)
	stableRef, err := member.ImportGrant(current)
	require.NoError(t, err)
	attestation, err := member.AttestDeliveryPublicKey(
		subjectIdentityPrivate(t, current.SubjectPrincipal), now.Add(time.Hour),
	)
	require.NoError(t, err)
	next := current
	next.Generation = 2
	next.GrantID = fixedID(0xc2)
	next.Secret, _ = identityapi.NewCapabilitySecret(bytes.Repeat([]byte{0xc3}, 32))
	next, err = SignGrant(next, issuerPrivate)
	require.NoError(t, err)
	bundle := GenerationBundle{
		Version: 1, RealmID: "r1_00112233445566778899aabbccddeeff",
		AuthorityPrincipal: current.IssuerPrincipal, AuthorityEpoch: 1,
		AuthoritySequence: 4,
		OperationID:       "rao1_20112233445566778899aabbccddeeff",
		DeliveryID:        "rad1_20112233445566778899aabbccddeeff",
		ChannelID:         next.ChannelID, ChannelClass: next.Scope, Generation: 2,
		RecipientPrincipal: next.SubjectPrincipal,
		DeliveryKeyDigest:  DeliveryPublicKeyDigest(attestation.DeliveryPublicKey),
		SubjectGrant:       next, SenderGrants: []identityapi.CapabilityGrant{next},
		ActivationPhase: DeliveryPhaseInstalled, DrainDeadline: now.Add(15 * time.Minute),
		ExpiresAt: now.Add(time.Hour), ReceiptKey: bytes.Repeat([]byte{0xc4}, 32),
	}
	sealed, err := SealGenerationBundleForRecipient(
		bundle, attestation, now,
		func(message []byte) ([]byte, error) {
			return ed25519.Sign(issuerPrivate, message), nil
		},
	)
	require.NoError(t, err)
	installed, err := member.InstallGenerationDelivery(sealed)
	require.NoError(t, err)
	readiness := member.GenerationReadiness(next.ChannelID)
	require.False(t, readiness.Ready)
	require.Equal(t, uint32(1), readiness.CurrentGeneration)
	require.Equal(t, uint32(2), readiness.PendingGeneration)
	resolved, err := member.ResolveCapability(validUse(stableRef, current))
	require.NoError(t, err)
	require.Equal(t, uint32(1), resolved.Generation)
	restarted, err := NewService(
		storePath, storeKey, current.SubjectPrincipal, trustedIssuer(issuerPublic),
		allowCapabilityAdmission{}, func() time.Time { return now },
	)
	require.NoError(t, err)
	require.False(t, restarted.GenerationReadiness(next.ChannelID).Ready)

	forged := installed
	forged.Phase = DeliveryPhaseActive
	forged, err = AuthenticateGenerationDeliveryReceipt(forged, bundle.ReceiptKey)
	require.NoError(t, err)
	require.NoError(t, VerifyGenerationDeliveryReceipt(forged, bundle.ReceiptKey))
	require.False(t, restarted.GenerationReadiness(next.ChannelID).Ready,
		"a receipt-key holder cannot make a member ready without the activation checkpoint")

	activation, err := SignGenerationActivationWith(GenerationActivation{
		Version: 1, RealmID: bundle.RealmID, AuthorityPrincipal: bundle.AuthorityPrincipal,
		AuthorityEpoch: 1, AuthoritySequence: 6, OperationID: bundle.OperationID,
		ChannelID: bundle.ChannelID, ChannelClass: bundle.ChannelClass,
		PreviousGeneration: 1, Generation: 2, EffectiveAt: now,
		DrainDeadline:    bundle.DrainDeadline,
		CheckpointDigest: "ac1_1ae66f80a93c4216bd9e07d3517eb44ad2ca1c5f873eaab607da3f3f238481d3",
	}, func(message []byte) ([]byte, error) {
		return ed25519.Sign(issuerPrivate, message), nil
	})
	require.NoError(t, err)
	restarted.activationAfterCommit = func() error { return assertCrashAfterCommit }
	_, err = restarted.ActivateGeneration(activation)
	require.Error(t, err)
	reopened, err := NewService(
		storePath, storeKey, current.SubjectPrincipal, trustedIssuer(issuerPublic),
		allowCapabilityAdmission{}, func() time.Time { return now },
	)
	require.NoError(t, err)
	active, err := reopened.ActivateGeneration(activation)
	require.NoError(t, err)
	require.Equal(t, DeliveryPhaseActive, active.Phase)
	require.Equal(t, uint64(6), active.AuthoritySequence)
	readiness = reopened.GenerationReadiness(next.ChannelID)
	require.True(t, readiness.Ready)
	require.Equal(t, uint32(2), readiness.CurrentGeneration)
	require.Equal(t, uint32(1), readiness.PreviousGeneration)
	require.Equal(t, activation.CheckpointDigest, readiness.CheckpointDigest)
	replay, err := messaging.NewDurableReplayLedger(
		filepath.Join(t.TempDir(), "replay.db"), bytes.Repeat([]byte{0xc5}, 32), 16, 64,
	)
	require.NoError(t, err)
	channel, err := messaging.NewChannel(messaging.ChannelConfig{
		Resolver: reopened, Authorizer: reopened, Replay: replay,
		Reference: stableRef, Subject: current.SubjectPrincipal, Scope: current.Scope,
		Signer: func() ed25519.PrivateKey {
			return subjectIdentityPrivate(t, current.SubjectPrincipal)
		},
		Clock: func() time.Time { return now },
	})
	require.NoError(t, err)
	oldResolved, err := reopened.ResolveCapabilityGeneration(identityapi.CapabilityUse{
		Ref: stableRef, Subject: current.SubjectPrincipal,
		Permission: identityapi.CapabilityPublish, Scope: current.Scope, At: now,
	}, 1)
	require.Error(t, err)
	oldEnvelope, err := messaging.Seal(messaging.SealRequest{
		Capability: resolvedCapabilityForTest(stableRef, current),
		Class:      messaging.MessageClassDiscoveryRecord, PayloadVersion: 1,
		Payload: []byte("generation-one"),
		Signer:  subjectIdentityPrivate(t, current.SubjectPrincipal), IssuedAt: now,
	})
	require.NoError(t, err)
	_ = oldResolved
	topics, err := channel.ContentTopics()
	require.NoError(t, err)
	require.Len(t, topics, 2)
	opened, err := channel.Open(oldEnvelope)
	require.NoError(t, err)
	require.Equal(t, []byte("generation-one"), opened.Payload)
	newEnvelope, err := channel.Seal(
		messaging.MessageClassDiscoveryRecord, 1, []byte("generation-two"),
	)
	require.NoError(t, err)
	require.NotEqual(t, oldEnvelope.ContentTopic, newEnvelope.ContentTopic)
	resolved, err = reopened.ResolveCapability(validUse(stableRef, next))
	require.NoError(t, err)
	require.Equal(t, uint32(2), resolved.Generation)
	previous, err := reopened.ResolveCapabilityGeneration(identityapi.CapabilityUse{
		Ref: stableRef, Subject: current.SubjectPrincipal,
		Permission: identityapi.CapabilitySubscribe, Scope: current.Scope, At: now,
	}, 1)
	require.NoError(t, err)
	require.Equal(t, uint32(1), previous.Generation)
	_, err = reopened.ResolveCapabilityGeneration(identityapi.CapabilityUse{
		Ref: stableRef, Subject: current.SubjectPrincipal,
		Permission: identityapi.CapabilityPublish, Scope: current.Scope, At: now,
	}, 1)
	require.Error(t, err)
}

func resolvedCapabilityForTest(
	ref identityapi.CapabilityRef,
	grant identityapi.CapabilityGrant,
) identityapi.ResolvedCapability {
	return identityapi.ResolvedCapability{
		Ref: ref, ChannelID: grant.ChannelID, Generation: grant.Generation,
		GrantID: grant.GrantID, Subject: grant.SubjectPrincipal,
		Permissions: grant.Permissions, Scope: grant.Scope, Secret: grant.Secret,
	}
}
