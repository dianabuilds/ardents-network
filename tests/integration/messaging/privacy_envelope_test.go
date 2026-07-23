//go:build integration

package messaging_test

import (
	"bytes"
	"crypto/ed25519"
	"path/filepath"
	"testing"
	"time"

	identityapi "ardents/internal/identity"
	identitycapability "ardents/internal/identity/capability"
	networkprivacy "ardents/internal/messaging"
	networkapi "ardents/internal/network"
	"ardents/internal/policy"
	"ardents/tests/testkit"

	"github.com/stretchr/testify/require"
)

func TestPrivateEnvelopeTraversesWakuRelayAndRejectsRestartReplay(t *testing.T) {
	scenario := testkit.BeginScenario(t, testkit.Spec{
		Layer: testkit.LayerIntegration, Domain: "network-foundation", ScenarioID: "NPI-002",
		Suite: "integration", Tags: []string{"integration", "network", "privacy", "envelope", "relay"},
		Speed: "default", Environment: "local",
	})
	ctx := t.Context()
	now := time.Now().UTC().Truncate(time.Second)
	fixture := newRelayPrivacyFixture(t, now)
	var sender networkapi.Service
	var receiver networkapi.Service
	var received <-chan networkapi.Envelope
	var sealed networkprivacy.SealedEnvelope
	var captured networkapi.Envelope

	scenario.Precondition("start receiver and subscribe to the capability-derived opaque topic", func(t *testing.T) {
		receiver = testkit.StartTransport(t)
		material, err := networkprivacy.Derive(fixture.receiverResolved)
		require.NoError(t, err)
		received, err = receiver.SubscribeRelayEnvelopes(
			ctx, networkprivacy.DefaultPubsubTopic, material.ContentTopic,
		)
		require.NoError(t, err)
	})

	scenario.Step("start sender and encrypt before relay publication", func(t *testing.T) {
		sender = testkit.StartBootstrappedTransport(t, receiver)
		testkit.WaitForRelayReadiness(t, sender)
		var err error
		sealed, err = networkprivacy.Seal(networkprivacy.SealRequest{
			Capability:     fixture.senderResolved,
			Class:          networkprivacy.MessageClassDiscoveryRecord,
			PayloadVersion: 1, Payload: fixture.plaintext,
			Signer: fixture.senderPrivate, IssuedAt: now,
		})
		require.NoError(t, err)
		require.NoError(t, sender.PublishRelayEnvelope(ctx, networkapi.Envelope{
			PubsubTopic: sealed.PubsubTopic, ContentTopic: sealed.ContentTopic,
			Payload: sealed.Payload,
		}))
	})

	scenario.Step("capture ciphertext from the real Waku subscriber", func(t *testing.T) {
		select {
		case item, ok := <-received:
			require.True(t, ok)
			captured = item
		case <-ctx.Done():
			require.FailNow(t, "timed out waiting for private relay envelope")
		}
	})

	scenario.Assert("carrier capture is opaque and the authorized receiver opens it", func(t *testing.T) {
		require.Equal(t, sealed.ContentTopic, captured.ContentTopic)
		require.Equal(t, sealed.Payload, captured.Payload)
		require.NotContains(t, captured.Payload, fixture.plaintext)
		require.NotContains(t, captured.Payload, []byte(fixture.senderGrant.SubjectPrincipal))
		require.NotContains(t, captured.ContentTopic, "discovery")
		require.Empty(t, testkit.InspectPrivateCapture(
			captured, fixture.plaintext, []byte(fixture.senderGrant.SubjectPrincipal), []byte("discovery"),
		))
		opened, err := networkprivacy.Open(fixture.openRequest(t, captured, now))
		require.NoError(t, err)
		require.Equal(t, fixture.plaintext, opened.Payload)
		require.Equal(t, fixture.senderGrant.SubjectPrincipal, opened.Sender)
	})

	scenario.Assert("capture detector rejects readable selector and payload mutations", func(t *testing.T) {
		readableSelector := captured
		readableSelector.ContentTopic = "/ardents/1/discovery-record/proto"
		selectorFindings := testkit.InspectPrivateCapture(readableSelector, []byte("discovery"))
		require.Contains(t, selectorFindings, testkit.FindingOpaqueSelectorInvalid)
		require.Contains(t, selectorFindings, testkit.FindingReadableTopic)

		readablePayload := captured
		readablePayload.Payload = append([]byte(nil), fixture.plaintext...)
		payloadFindings := testkit.InspectPrivateCapture(readablePayload, fixture.plaintext)
		require.Contains(t, payloadFindings, testkit.FindingEncryptedPayloadInvalid)
		require.Contains(t, payloadFindings, testkit.FindingReadablePayload)
	})

	scenario.Assert("durable restart replay and ciphertext tamper fail terminally", func(t *testing.T) {
		restored, err := networkprivacy.NewDurableReplayLedger(
			fixture.replayPath, fixture.replayKey, 32, 128,
		)
		require.NoError(t, err)
		request := fixture.openRequestWithLedger(captured, now, restored)
		_, err = networkprivacy.Open(request)
		require.Equal(t, networkprivacy.CodeEnvelopeReplayed, networkprivacy.CodeOf(err))

		tampered := captured
		tampered.Payload = append([]byte(nil), captured.Payload...)
		tampered.Payload[len(tampered.Payload)-1] ^= 1
		fresh, err := networkprivacy.NewDurableReplayLedger(
			filepath.Join(t.TempDir(), "ardents.db"), fixture.replayKey, 32, 128,
		)
		require.NoError(t, err)
		_, err = networkprivacy.Open(fixture.openRequestWithLedger(tampered, now, fresh))
		require.Equal(t, networkprivacy.CodeEnvelopeAuthentication, networkprivacy.CodeOf(err))
	})
}

type relayPrivacyFixture struct {
	senderResolved    identityapi.ResolvedCapability
	receiverResolved  identityapi.ResolvedCapability
	senderGrant       identityapi.CapabilityGrant
	senderPrivate     ed25519.PrivateKey
	receiverPrivate   ed25519.PrivateKey
	senderAuthority   *identitycapability.Service
	receiverAuthority *identitycapability.Service
	senderRef         identityapi.CapabilityRef
	receiverRef       identityapi.CapabilityRef
	replayPath        string
	replayKey         []byte
	plaintext         []byte
}

func newRelayPrivacyFixture(t *testing.T, now time.Time) relayPrivacyFixture {
	t.Helper()
	issuerPrivate := relayPrivate(0x12)
	issuerPublic := issuerPrivate.Public().(ed25519.PublicKey)
	issuer := integrationPrincipalID(issuerPublic)
	senderPrivate := relayPrivate(0x22)
	receiverPrivate := relayPrivate(0x32)
	secret, ok := identityapi.NewCapabilitySecret(bytes.Repeat([]byte{0x42}, 32))
	require.True(t, ok)
	channelID := integrationID(0x52)
	senderGrant := signedRelayGrant(t, issuerPrivate, issuer, senderPrivate, secret, channelID, 0x62, now)
	receiverGrant := signedRelayGrant(t, issuerPrivate, issuer, receiverPrivate, secret, channelID, 0x72, now)
	trusted := integrationTrustRegistry(t, issuer, issuerPublic)
	authority, err := identitycapability.NewService(
		filepath.Join(t.TempDir(), "capabilities.db"), bytes.Repeat([]byte{0x82}, 32),
		receiverGrant.SubjectPrincipal, trusted, policy.New(policy.Config{}), func() time.Time { return now },
	)
	require.NoError(t, err)
	receiverRef, err := authority.ImportGrant(receiverGrant)
	require.NoError(t, err)
	require.NoError(t, authority.ImportSenderGrant(senderGrant))
	senderAuthority, err := identitycapability.NewService(
		filepath.Join(t.TempDir(), "sender-capabilities.db"), bytes.Repeat([]byte{0x83}, 32),
		senderGrant.SubjectPrincipal, trusted, policy.New(policy.Config{}), func() time.Time { return now },
	)
	require.NoError(t, err)
	senderRef, err := senderAuthority.ImportGrant(senderGrant)
	require.NoError(t, err)
	receiverResolved, err := authority.ResolveCapability(identityapi.CapabilityUse{
		Ref: receiverRef, Subject: receiverGrant.SubjectPrincipal,
		Permission: identityapi.CapabilitySubscribe, Scope: receiverGrant.Scope, At: now,
	})
	require.NoError(t, err)
	return relayPrivacyFixture{
		senderResolved:   resolvedRelayGrant("cap_sender", senderGrant),
		receiverResolved: receiverResolved, senderGrant: senderGrant,
		senderPrivate: senderPrivate, receiverPrivate: receiverPrivate,
		senderAuthority: senderAuthority, receiverAuthority: authority,
		senderRef: senderRef, receiverRef: receiverRef,
		replayPath: filepath.Join(t.TempDir(), "ardents.db"),
		replayKey:  bytes.Repeat([]byte{0x92}, 32),
		plaintext:  []byte("node=p_sender service=private-api operation=discovery"),
	}
}

func (f relayPrivacyFixture) channels(t *testing.T, now time.Time) (*networkprivacy.Channel, *networkprivacy.Channel) {
	t.Helper()
	senderReplay, err := networkprivacy.NewDurableReplayLedger(filepath.Join(t.TempDir(), "sender-replay.db"), bytes.Repeat([]byte{0xa2}, 32), 64, 256)
	require.NoError(t, err)
	receiverReplay, err := networkprivacy.NewDurableReplayLedger(filepath.Join(t.TempDir(), "receiver-replay.db"), bytes.Repeat([]byte{0xb2}, 32), 64, 256)
	require.NoError(t, err)
	sender, err := networkprivacy.NewChannel(networkprivacy.ChannelConfig{
		Resolver: f.senderAuthority, Authorizer: f.senderAuthority, Replay: senderReplay,
		Reference: f.senderRef, Subject: f.senderGrant.SubjectPrincipal,
		Scope: identityapi.CapabilityRealmDiscovery, Signer: func() ed25519.PrivateKey { return f.senderPrivate },
		Clock: func() time.Time { return now },
	})
	require.NoError(t, err)
	receiver, err := networkprivacy.NewChannel(networkprivacy.ChannelConfig{
		Resolver: f.receiverAuthority, Authorizer: f.receiverAuthority, Replay: receiverReplay,
		Reference: f.receiverRef, Subject: f.receiverResolved.Subject,
		Scope: identityapi.CapabilityRealmDiscovery, Signer: func() ed25519.PrivateKey { return f.receiverPrivate },
		Clock: func() time.Time { return now },
	})
	require.NoError(t, err)
	return sender, receiver
}

func (f relayPrivacyFixture) openRequest(t *testing.T, envelope networkapi.Envelope, now time.Time) networkprivacy.OpenRequest {
	t.Helper()
	ledger, err := networkprivacy.NewDurableReplayLedger(f.replayPath, f.replayKey, 32, 128)
	require.NoError(t, err)
	return f.openRequestWithLedger(envelope, now, ledger)
}

func (f relayPrivacyFixture) openRequestWithLedger(envelope networkapi.Envelope, now time.Time, ledger networkprivacy.ReplayLedger) networkprivacy.OpenRequest {
	return networkprivacy.OpenRequest{
		Capability:  f.receiverResolved,
		PubsubTopic: envelope.PubsubTopic, ContentTopic: envelope.ContentTopic,
		Payload: envelope.Payload, Authorizer: f.receiverAuthority, Replay: ledger, Now: now,
	}
}

func signedRelayGrant(t *testing.T, issuerPrivate ed25519.PrivateKey, issuer string, subjectPrivate ed25519.PrivateKey, secret identityapi.CapabilitySecret, channelID [16]byte, grantByte byte, now time.Time) identityapi.CapabilityGrant {
	t.Helper()
	grant := identityapi.CapabilityGrant{
		Version: 1, ChannelID: channelID, Generation: 1, Secret: secret,
		GrantID: integrationID(grantByte), IssuerPrincipal: issuer,
		SubjectPrincipal: integrationPrincipalID(subjectPrivate.Public().(ed25519.PublicKey)),
		Permissions:      identityapi.CapabilitySubscribe | identityapi.CapabilityPublish | identityapi.CapabilityStoreFetch,
		Scope:            identityapi.CapabilityRealmDiscovery,
		NotBefore:        now.Add(-time.Hour), NotAfter: now.Add(time.Hour),
	}
	signed, err := identitycapability.SignGrant(grant, issuerPrivate)
	require.NoError(t, err)
	return signed
}

func resolvedRelayGrant(ref identityapi.CapabilityRef, grant identityapi.CapabilityGrant) identityapi.ResolvedCapability {
	return identityapi.ResolvedCapability{
		Ref: ref, ChannelID: grant.ChannelID, Generation: grant.Generation,
		GrantID: grant.GrantID, Subject: grant.SubjectPrincipal,
		Permissions: grant.Permissions, Scope: grant.Scope, Secret: grant.Secret,
	}
}

func relayPrivate(value byte) ed25519.PrivateKey {
	return ed25519.NewKeyFromSeed(bytes.Repeat([]byte{value}, ed25519.SeedSize))
}
