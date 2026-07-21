package messaging

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	identityapi "ardents/internal/identity"
	identitycapability "ardents/internal/identity/capability"
	identityprincipal "ardents/internal/identity/principal"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/chacha20poly1305"
	"google.golang.org/protobuf/proto"
)

var envelopeTestNow = time.Unix(1_800_100_000, 0).UTC()

func TestPrivateEnvelopeRoundTripHidesProductSemantics(t *testing.T) {
	fixture := newEnvelopeFixture(t, true)
	payload := []byte("service=private-api blob=QmSecret request=req-42")
	sealed := fixture.seal(t, payload)
	digest := sha256.Sum256(sealed.Payload)
	require.Equal(t, "e29d0284fb9ac2b66be0ef4f06a310670256ad4d7b6280967f9e97982ccbe576", hex.EncodeToString(digest[:]))

	require.NotContains(t, sealed.ContentTopic, "service")
	require.NotContains(t, sealed.Payload, payload)
	require.NotContains(t, sealed.Payload, []byte(fixture.senderGrant.SubjectPrincipal))
	require.NotContains(t, sealed.Payload, fixture.senderGrant.GrantID[:])
	require.Equal(t, headerSize+1024+chacha20poly1305.Overhead, len(sealed.Payload))

	opened, err := Open(fixture.openRequest(t, sealed))
	require.NoError(t, err)
	require.Equal(t, payload, opened.Payload)
	require.Equal(t, fixture.senderGrant.SubjectPrincipal, opened.Sender)
	require.Equal(t, MessageClassDiscoveryRecord, opened.Class)

	sealedJSON, err := json.Marshal(sealed)
	require.NoError(t, err)
	require.NotContains(t, string(sealedJSON), string(sealed.Payload))
	require.NotContains(t, string(sealedJSON), sealed.ContentTopic)
	openedJSON, err := json.Marshal(opened)
	require.NoError(t, err)
	require.NotContains(t, string(openedJSON), string(payload))
	require.NotContains(t, string(openedJSON), opened.Sender)
}

func TestPrivateEnvelopeRejectsTamperRelocationAndWrongKey(t *testing.T) {
	fixture := newEnvelopeFixture(t, true)
	sealed := fixture.seal(t, []byte("protected"))

	tampered := sealed
	tampered.Payload = append([]byte(nil), sealed.Payload...)
	tampered.Payload[len(tampered.Payload)-1] ^= 1
	requireEnvelopeCode(t, openWithFreshReplay(t, fixture, tampered), CodeEnvelopeAuthentication)

	relocated := sealed
	relocated.ContentTopic = "/ardents/1/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/proto"
	requireEnvelopeCode(t, openWithFreshReplay(t, fixture, relocated), CodeEnvelopeAuthentication)

	wrongKey := fixture
	wrongKey.receiverResolved.Secret = mustSecret(t, 0x71)
	requireEnvelopeCode(t, openWithFreshReplay(t, wrongKey, sealed), CodeEnvelopeAuthentication)
}

func TestPrivateEnvelopeRejectsReplayAcrossRestart(t *testing.T) {
	fixture := newEnvelopeFixture(t, true)
	sealed := fixture.seal(t, []byte("once"))
	request := fixture.openRequest(t, sealed)
	_, err := Open(request)
	require.NoError(t, err)

	restored, err := NewDurableReplayLedger(
		fixture.replayPath, fixture.replayKey, 8, 16,
	)
	require.NoError(t, err)
	request.Replay = restored
	_, err = Open(request)
	requireEnvelopeCode(t, err, CodeEnvelopeReplayed)

	raw, err := os.ReadFile(fixture.replayPath)
	require.NoError(t, err)
	require.NotContains(t, raw, []byte(fixture.receiverResolved.Ref))
	require.NotContains(t, raw, sealed.Payload[28:44])
}

func TestPrivateEnvelopeRejectsOuterVersionFlagsTimeAndSize(t *testing.T) {
	fixture := newEnvelopeFixture(t, true)
	sealed := fixture.seal(t, []byte("bounded"))
	tests := []struct {
		name string
		edit func([]byte) []byte
		code string
	}{
		{name: "version", code: CodeEnvelopeVersionUnsupported, edit: func(raw []byte) []byte { raw[4] = 2; return raw }},
		{name: "suite", code: CodeEnvelopeSuiteUnsupported, edit: func(raw []byte) []byte { raw[5] = 2; return raw }},
		{name: "flags", code: CodeEnvelopeFlagsUnsupported, edit: func(raw []byte) []byte { raw[7] = 1; return raw }},
		{name: "generation", code: CodeEnvelopeAuthentication, edit: func(raw []byte) []byte { raw[11] = 2; return raw }},
		{name: "length", code: CodeEnvelopeMalformed, edit: func(raw []byte) []byte { binary.BigEndian.PutUint32(raw[68:72], 1); return raw }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			changed := sealed
			changed.Payload = tt.edit(append([]byte(nil), sealed.Payload...))
			requireEnvelopeCode(t, openWithFreshReplay(t, fixture, changed), tt.code)
		})
	}

	expired := fixture.openRequest(t, sealed)
	expired.Now = envelopeTestNow.Add(15 * time.Minute)
	_, err := Open(expired)
	requireEnvelopeCode(t, err, CodeEnvelopeExpired)

	future := fixture.sealAt(t, envelopeTestNow.Add(6*time.Minute), []byte("future"))
	futureRequest := fixture.openRequest(t, future)
	futureRequest.Now = envelopeTestNow
	_, err = Open(futureRequest)
	requireEnvelopeCode(t, err, CodeEnvelopeTimeInvalid)

	oversized := fixture.openRequest(t, sealed)
	oversized.Payload = make([]byte, maximumOuterSize+1)
	_, err = Open(oversized)
	requireEnvelopeCode(t, err, CodeEnvelopeOversized)
}

func TestPrivateEnvelopeRejectsInvalidInnerSignatureAndSenderGrant(t *testing.T) {
	fixture := newEnvelopeFixture(t, true)
	sealed := fixture.seal(t, []byte("signed"))

	badSignature := rewriteAuthenticatedInner(t, fixture, sealed, func(message *PrivateMessageV1) {
		message.Signature[0] ^= 1
	})
	requireEnvelopeCode(t, openWithFreshReplay(t, fixture, badSignature), CodeEnvelopeSignatureInvalid)

	badPadding := rewriteAuthenticatedInner(t, fixture, sealed, func(message *PrivateMessageV1) {
		message.Padding[0] = 1
	})
	requireEnvelopeCode(t, openWithFreshReplay(t, fixture, badPadding), CodeEnvelopeMalformed)

	badProtocol := rewriteAuthenticatedInner(t, fixture, sealed, func(message *PrivateMessageV1) {
		message.ProtocolVersion = 2
	})
	requireEnvelopeCode(t, openWithFreshReplay(t, fixture, badProtocol), CodeEnvelopeMalformed)

	badClass := rewriteAuthenticatedInner(t, fixture, sealed, func(message *PrivateMessageV1) {
		message.MessageClass = 99
	})
	requireEnvelopeCode(t, openWithFreshReplay(t, fixture, badClass), CodeEnvelopeSenderUnauthorized)

	badPrincipal := rewriteAuthenticatedInner(t, fixture, sealed, func(message *PrivateMessageV1) {
		message.SenderPrincipal = identityprincipal.DeriveID("p", deterministicPrivate(0x44).Public().(ed25519.PublicKey))
	})
	requireEnvelopeCode(t, openWithFreshReplay(t, fixture, badPrincipal), CodeEnvelopeMalformed)

	unauthorized := newEnvelopeFixture(t, false)
	unauthorizedSealed := unauthorized.seal(t, []byte("unknown sender"))
	requireEnvelopeCode(t, openWithFreshReplay(t, unauthorized, unauthorizedSealed), CodeEnvelopeSenderUnauthorized)
}

func TestPrivateEnvelopeRejectsRevokedSenderAndOversizedPlaintext(t *testing.T) {
	fixture := newEnvelopeFixture(t, true)
	revocation, err := identitycapability.SignRevocation(identityapi.CapabilityRevocation{
		Version: 1, GrantID: fixture.senderGrant.GrantID,
		IssuerPrincipal: fixture.senderGrant.IssuerPrincipal,
		RevokedAt:       envelopeTestNow,
	}, fixture.issuerPrivate)
	require.NoError(t, err)
	require.NoError(t, fixture.receiverAuthority.ApplyRevocation(revocation))
	sealed := fixture.seal(t, []byte("revoked sender"))
	requireEnvelopeCode(t, openWithFreshReplay(t, fixture, sealed), CodeEnvelopeSenderUnauthorized)

	backdated := fixture.sealAt(t, envelopeTestNow.Add(-time.Second), []byte("backdated after revocation"))
	requireEnvelopeCode(t, openWithFreshReplay(t, fixture, backdated), CodeEnvelopeSenderUnauthorized)

	_, err = Seal(fixture.sealRequest(bytes.Repeat([]byte{0x42}, maximumInnerSize)))
	requireEnvelopeCode(t, err, CodeEnvelopeOversized)
}

func TestPrivateEnvelopeRoundTripsAcrossPaddingBoundaries(t *testing.T) {
	fixture := newEnvelopeFixture(t, true)
	for _, size := range []int{1, 700, 850, 900, 950, 3700, 3950, 15800, 16000, 65000} {
		t.Run(stringSize(size), func(t *testing.T) {
			payload := bytes.Repeat([]byte{0x42}, size)
			sealed := fixture.seal(t, payload)
			opened, err := Open(freshOpenRequest(t, fixture, sealed))
			require.NoError(t, err)
			require.Equal(t, payload, opened.Payload)
		})
	}
}

func TestDurableReplayLedgerCapacityAndExpiryPruning(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ardents.db")
	ledger, err := NewDurableReplayLedger(path, bytes.Repeat([]byte{0x91}, 32), 1, 2)
	require.NoError(t, err)
	first := ReplayUse{
		CapabilityRef: "cap_local", Generation: 1, MessageID: testMessageID(1),
		ExpiresAt: envelopeTestNow.Add(time.Minute), Now: envelopeTestNow,
	}
	require.NoError(t, ledger.Admit(first))
	second := first
	second.MessageID = testMessageID(2)
	requireEnvelopeCode(t, ledger.Admit(second), CodeReplayCapacityExhausted)
	second.Now = first.ExpiresAt
	second.ExpiresAt = second.Now.Add(time.Minute)
	require.NoError(t, ledger.Admit(second))
}

func TestDurableReplayLedgerRequiresPersistentPath(t *testing.T) {
	_, err := NewDurableReplayLedger("", bytes.Repeat([]byte{0x91}, 32), 1, 2)
	require.ErrorContains(t, err, "path is required")
}

type envelopeFixture struct {
	senderResolved    identityapi.ResolvedCapability
	receiverResolved  identityapi.ResolvedCapability
	senderPrivate     ed25519.PrivateKey
	senderGrant       identityapi.CapabilityGrant
	issuerPrivate     ed25519.PrivateKey
	receiverAuthority *identitycapability.Service
	replayPath        string
	replayKey         []byte
}

func newEnvelopeFixture(t *testing.T, importSender bool) envelopeFixture {
	t.Helper()
	issuerPrivate := deterministicPrivate(0x11)
	issuerPublic := issuerPrivate.Public().(ed25519.PublicKey)
	senderPrivate := deterministicPrivate(0x21)
	receiverPrivate := deterministicPrivate(0x31)
	secret := mustSecret(t, 0x41)
	channelID := testID(0x51)
	senderGrant := signedEnvelopeGrant(t, issuerPrivate, senderPrivate, secret, channelID, 0x61)
	receiverGrant := signedEnvelopeGrant(t, issuerPrivate, receiverPrivate, secret, channelID, 0x71)
	issuers := map[string]ed25519.PublicKey{senderGrant.IssuerPrincipal: issuerPublic}
	storePath := filepath.Join(t.TempDir(), "capabilities.db")
	authority, err := identitycapability.NewService(
		storePath, bytes.Repeat([]byte{0x81}, 32), receiverGrant.SubjectPrincipal,
		issuers, allowEnvelopeCapability{}, func() time.Time { return envelopeTestNow },
	)
	require.NoError(t, err)
	receiverRef, err := authority.ImportGrant(receiverGrant)
	require.NoError(t, err)
	if importSender {
		require.NoError(t, authority.ImportSenderGrant(senderGrant))
	}
	return envelopeFixture{
		senderResolved:   resolvedGrant("cap_sender", senderGrant),
		receiverResolved: resolvedGrant(receiverRef, receiverGrant),
		senderPrivate:    senderPrivate, senderGrant: senderGrant,
		issuerPrivate: issuerPrivate, receiverAuthority: authority,
		replayPath: filepath.Join(t.TempDir(), "replay.db"),
		replayKey:  bytes.Repeat([]byte{0x91}, 32),
	}
}

func (f envelopeFixture) seal(t *testing.T, payload []byte) SealedEnvelope {
	t.Helper()
	return f.sealAt(t, envelopeTestNow, payload)
}

func (f envelopeFixture) sealAt(t *testing.T, at time.Time, payload []byte) SealedEnvelope {
	t.Helper()
	sealed, err := Seal(f.sealRequestAt(at, payload))
	require.NoError(t, err)
	return sealed
}

func (f envelopeFixture) sealRequest(payload []byte) SealRequest {
	return f.sealRequestAt(envelopeTestNow, payload)
}

func (f envelopeFixture) sealRequestAt(at time.Time, payload []byte) SealRequest {
	return SealRequest{
		Capability: f.senderResolved, Class: MessageClassDiscoveryRecord,
		PayloadVersion: 1, Payload: payload, Signer: f.senderPrivate,
		IssuedAt: at, Random: bytes.NewReader(bytes.Repeat([]byte{0xa3}, 40)),
	}
}

func (f envelopeFixture) openRequest(t *testing.T, sealed SealedEnvelope) OpenRequest {
	t.Helper()
	ledger, err := NewDurableReplayLedger(f.replayPath, f.replayKey, 8, 16)
	require.NoError(t, err)
	return OpenRequest{
		Capability: f.receiverResolved, PubsubTopic: sealed.PubsubTopic,
		ContentTopic: sealed.ContentTopic, Payload: sealed.Payload,
		Authorizer: f.receiverAuthority, Replay: ledger, Now: envelopeTestNow,
	}
}

func openWithFreshReplay(t *testing.T, fixture envelopeFixture, sealed SealedEnvelope) error {
	t.Helper()
	_, err := Open(freshOpenRequest(t, fixture, sealed))
	return err
}

func freshOpenRequest(t *testing.T, fixture envelopeFixture, sealed SealedEnvelope) OpenRequest {
	t.Helper()
	fixture.replayPath = filepath.Join(t.TempDir(), "replay.db")
	return fixture.openRequest(t, sealed)
}

func rewriteAuthenticatedInner(t *testing.T, fixture envelopeFixture, sealed SealedEnvelope, mutate func(*PrivateMessageV1)) SealedEnvelope {
	t.Helper()
	header, err := parseHeader(sealed.Payload)
	require.NoError(t, err)
	material, err := Derive(fixture.receiverResolved)
	require.NoError(t, err)
	aead, err := chacha20poly1305.NewX(material.EnvelopeKey())
	require.NoError(t, err)
	aad, err := associatedData(sealed.Payload[:headerSize], sealed.PubsubTopic, sealed.ContentTopic)
	require.NoError(t, err)
	inner, err := aead.Open(nil, header.Nonce[:], sealed.Payload[headerSize:], aad)
	require.NoError(t, err)
	message := &PrivateMessageV1{}
	require.NoError(t, proto.Unmarshal(inner, message))
	mutate(message)
	inner, err = proto.MarshalOptions{Deterministic: true}.Marshal(message)
	require.NoError(t, err)
	header.CiphertextLength = uint32(len(inner) + aead.Overhead())
	headerRaw := header.marshal()
	aad, err = associatedData(headerRaw, sealed.PubsubTopic, sealed.ContentTopic)
	require.NoError(t, err)
	sealed.Payload = append(headerRaw, aead.Seal(nil, header.Nonce[:], inner, aad)...)
	return sealed
}

func signedEnvelopeGrant(t *testing.T, issuer, subject ed25519.PrivateKey, secret identityapi.CapabilitySecret, channelID [16]byte, grantByte byte) identityapi.CapabilityGrant {
	t.Helper()
	issuerPublic := issuer.Public().(ed25519.PublicKey)
	subjectPublic := subject.Public().(ed25519.PublicKey)
	grant := identityapi.CapabilityGrant{
		Version: 1, ChannelID: channelID, Generation: 1, Secret: secret,
		GrantID:          testID(grantByte),
		IssuerPrincipal:  identityprincipal.DeriveID("p", issuerPublic),
		SubjectPrincipal: identityprincipal.DeriveID("p", subjectPublic),
		Permissions:      identityapi.CapabilitySubscribe | identityapi.CapabilityPublish,
		Scope:            identityapi.CapabilityRealmDiscovery,
		NotBefore:        envelopeTestNow.Add(-time.Hour), NotAfter: envelopeTestNow.Add(time.Hour),
	}
	signed, err := identitycapability.SignGrant(grant, issuer)
	require.NoError(t, err)
	return signed
}

func resolvedGrant(ref identityapi.CapabilityRef, grant identityapi.CapabilityGrant) identityapi.ResolvedCapability {
	return identityapi.ResolvedCapability{
		Ref: ref, ChannelID: grant.ChannelID, Generation: grant.Generation,
		GrantID: grant.GrantID, Subject: grant.SubjectPrincipal,
		Permissions: grant.Permissions, Scope: grant.Scope, Secret: grant.Secret,
	}
}

func deterministicPrivate(value byte) ed25519.PrivateKey {
	return ed25519.NewKeyFromSeed(bytes.Repeat([]byte{value}, ed25519.SeedSize))
}

func mustSecret(t *testing.T, value byte) identityapi.CapabilitySecret {
	t.Helper()
	secret, ok := identityapi.NewCapabilitySecret(bytes.Repeat([]byte{value}, 32))
	require.True(t, ok)
	return secret
}

func testID(value byte) [16]byte {
	var id [16]byte
	for index := range id {
		id[index] = value
	}
	return id
}

func testMessageID(value byte) [16]byte { return testID(value) }

func stringSize(size int) string {
	return "payload_" + strconv.Itoa(size)
}

func requireEnvelopeCode(t *testing.T, err error, code string) {
	t.Helper()
	require.Error(t, err)
	require.Equal(t, code, CodeOf(err))
}

type allowEnvelopeCapability struct{}

func (allowEnvelopeCapability) AllowCapabilityUse(identityapi.CapabilityUse) error { return nil }
