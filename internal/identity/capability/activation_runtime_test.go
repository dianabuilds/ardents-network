package capability_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"ardents/internal/channeldelivery"
	identityapi "ardents/internal/identity"
	identitycapability "ardents/internal/identity/capability"
	identityprincipal "ardents/internal/identity/principal"
	identitytrust "ardents/internal/identity/trust"
	"ardents/internal/messaging"
	"ardents/internal/transfer"

	"github.com/stretchr/testify/require"
)

func TestActiveReceiptWaitsForLiveExchangeGenerationSubscriptions(t *testing.T) {
	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	issuerPrivate := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0xf1}, ed25519.SeedSize))
	issuerPrincipal, err := identityprincipal.FromEd25519PublicKey(
		issuerPrivate.Public().(ed25519.PublicKey),
	)
	require.NoError(t, err)
	subjectPrivate := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0xf2}, ed25519.SeedSize))
	subjectPrincipal, err := identityprincipal.FromEd25519PublicKey(
		subjectPrivate.Public().(ed25519.PublicKey),
	)
	require.NoError(t, err)
	secret, ok := identityapi.NewCapabilitySecret(bytes.Repeat([]byte{0xf3}, 32))
	require.True(t, ok)
	current := identityapi.CapabilityGrant{
		Version: 1, ChannelID: [16]byte{0xf4}, Generation: 1,
		Secret: secret, GrantID: [16]byte{0xf5},
		IssuerPrincipal: issuerPrincipal.String(), SubjectPrincipal: subjectPrincipal.String(),
		Permissions: identityapi.CapabilityPublish | identityapi.CapabilitySubscribe,
		Scope:       identityapi.CapabilityDataExchange,
		NotBefore:   now.Add(-time.Hour), NotAfter: now.Add(time.Hour),
	}
	current, err = identitycapability.SignGrant(current, issuerPrivate)
	require.NoError(t, err)
	trust, err := identitytrust.NewRegistry([]identitytrust.Entry{{
		Principal: issuerPrincipal.String(),
		PublicKey: issuerPrivate.Public().(ed25519.PublicKey),
		Purposes:  []identitytrust.Purpose{identitytrust.PurposeChannelIssue},
	}})
	require.NoError(t, err)
	member, err := identitycapability.NewService(
		filepath.Join(t.TempDir(), "member.db"), bytes.Repeat([]byte{0xf6}, 32),
		subjectPrincipal.String(), trust, runtimeCapabilityPolicy{},
		func() time.Time { return now },
	)
	require.NoError(t, err)
	ref, err := member.ImportGrant(current)
	require.NoError(t, err)
	attestation, err := member.AttestDeliveryPublicKey(subjectPrivate, now.Add(time.Hour))
	require.NoError(t, err)
	next := current
	next.Generation = 2
	next.GrantID = [16]byte{0xf7}
	next.Secret, _ = identityapi.NewCapabilitySecret(bytes.Repeat([]byte{0xf8}, 32))
	next, err = identitycapability.SignGrant(next, issuerPrivate)
	require.NoError(t, err)
	bundle := identitycapability.GenerationBundle{
		Version: 1, RealmID: "r1_00112233445566778899aabbccddeeff",
		AuthorityPrincipal: issuerPrincipal.String(), AuthorityEpoch: 1, AuthoritySequence: 4,
		OperationID: "rao1_f0112233445566778899aabbccddeeff",
		DeliveryID:  "rad1_f0112233445566778899aabbccddeeff",
		ChannelID:   next.ChannelID, ChannelClass: next.Scope, Generation: 2,
		RecipientPrincipal: next.SubjectPrincipal,
		DeliveryKeyDigest:  identitycapability.DeliveryPublicKeyDigest(attestation.DeliveryPublicKey),
		SubjectGrant:       next, SenderGrants: []identityapi.CapabilityGrant{next},
		ActivationPhase: identitycapability.DeliveryPhaseInstalled,
		DrainDeadline:   now.Add(15 * time.Minute), ExpiresAt: now.Add(time.Hour),
		ReceiptKey: bytes.Repeat([]byte{0xf9}, 32),
	}
	sealed, err := identitycapability.SealGenerationBundleForRecipient(
		bundle, attestation, now,
		func(message []byte) ([]byte, error) {
			return ed25519.Sign(issuerPrivate, message), nil
		},
	)
	require.NoError(t, err)
	_, err = member.InstallGenerationDelivery(sealed)
	require.NoError(t, err)
	replay, err := messaging.NewDurableReplayLedger(
		filepath.Join(t.TempDir(), "replay.db"), bytes.Repeat([]byte{0xfa}, 32), 16, 64,
	)
	require.NoError(t, err)
	channel, err := messaging.NewChannel(messaging.ChannelConfig{
		Resolver: member, Authorizer: member, Replay: replay, Reference: ref,
		Subject: subjectPrincipal.String(), Scope: current.Scope,
		Signer: func() ed25519.PrivateKey { return subjectPrivate },
		Clock:  func() time.Time { return now },
	})
	require.NoError(t, err)
	carrier := &activationCarrier{}
	exchange := transfer.NewPrivateExchange(channel, carrier)
	runContext, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	require.NoError(t, exchange.Start(runContext))
	initialTopics := carrier.snapshot()
	require.Len(t, initialTopics, 1)
	delivery, err := channeldelivery.New(
		member, subjectPrivate, subjectPrincipal.String(), func() time.Time { return now },
	)
	require.NoError(t, err)
	delivery.BindActivationRuntime(exchange)
	activation, err := identitycapability.SignGenerationActivationWith(
		identitycapability.GenerationActivation{
			Version: 1, RealmID: bundle.RealmID,
			AuthorityPrincipal: bundle.AuthorityPrincipal, AuthorityEpoch: 1,
			AuthoritySequence: 6, OperationID: bundle.OperationID,
			ChannelID: bundle.ChannelID, ChannelClass: bundle.ChannelClass,
			PreviousGeneration: 1, Generation: 2, EffectiveAt: now,
			DrainDeadline:    bundle.DrainDeadline,
			CheckpointDigest: "ac1_3ae66f80a93c4216bd9e07d3517eb44ad2ca1c5f873eaab607da3f3f238481d3",
		},
		func(message []byte) ([]byte, error) {
			return ed25519.Sign(issuerPrivate, message), nil
		},
	)
	require.NoError(t, err)
	carrier.failNextSubscription()
	_, err = delivery.Activate(
		context.Background(), channeldelivery.Command{Actor: "operator", Effective: "operator"},
		channeldelivery.ContractVersion, activation,
	)
	require.ErrorIs(t, err, channeldelivery.ErrUnavailable)
	require.Equal(t, initialTopics, carrier.snapshot(),
		"failed adoption must retain the original live subscription")
	active, err := delivery.Activate(
		context.Background(), channeldelivery.Command{Actor: "operator", Effective: "operator"},
		channeldelivery.ContractVersion, activation,
	)
	require.NoError(t, err)
	require.Equal(t, identitycapability.DeliveryPhaseActive, active.Phase)
	topics := carrier.snapshot()
	require.Len(t, topics, 3)
	require.Contains(t, topics[1:], initialTopics[0])
	require.NotEqual(t, topics[1], topics[2])
}

type runtimeCapabilityPolicy struct{}

func (runtimeCapabilityPolicy) AllowCapabilityUse(identityapi.CapabilityUse) error { return nil }

type activationCarrier struct {
	mu       sync.Mutex
	topics   []string
	failNext bool
}

func (c *activationCarrier) PublishPrivateEnvelope(context.Context, messaging.SealedEnvelope) error {
	return nil
}

func (c *activationCarrier) SubscribePrivateEnvelopes(
	ctx context.Context,
	topic string,
) (<-chan messaging.SealedEnvelope, error) {
	c.mu.Lock()
	if c.failNext {
		c.failNext = false
		c.mu.Unlock()
		return nil, errors.New("subscription unavailable")
	}
	c.topics = append(c.topics, topic)
	c.mu.Unlock()
	envelopes := make(chan messaging.SealedEnvelope)
	go func() {
		<-ctx.Done()
		close(envelopes)
	}()
	return envelopes, nil
}

func (c *activationCarrier) failNextSubscription() {
	c.mu.Lock()
	c.failNext = true
	c.mu.Unlock()
}

func (c *activationCarrier) snapshot() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.topics...)
}
