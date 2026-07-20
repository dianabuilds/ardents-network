package testkit

import (
	"bytes"
	"crypto/ed25519"
	"path/filepath"
	"testing"
	"time"

	identityapi "ardents/internal/identity/api"
	identitycapability "ardents/internal/identity/capability"
	identityprincipal "ardents/internal/identity/principal"
	networkprivacy "ardents/internal/network/privacy"
	policy "ardents/internal/policy"

	"github.com/stretchr/testify/require"
)

type DiscoveryPrivacyFixture struct {
	Sender   *networkprivacy.Channel
	Receiver *networkprivacy.Channel
	issuer   ed25519.PrivateKey
	grant    identityapi.CapabilityGrant
	receiver *identitycapability.Service
}

type PrivacyGroupFixture struct {
	Channels    []*networkprivacy.Channel
	issuer      ed25519.PrivateKey
	grants      []identityapi.CapabilityGrant
	authorities []*identitycapability.Service
}

func NewDiscoveryPrivacyFixture(t *testing.T, now time.Time) DiscoveryPrivacyFixture {
	return newPrivacyFixture(t, now, identityapi.CapabilityRealmDiscovery)
}

func NewDataPrivacyFixture(t *testing.T, now time.Time) DiscoveryPrivacyFixture {
	return newPrivacyFixture(t, now, identityapi.CapabilityDataExchange)
}

func NewDiscoveryPrivacyGroupFixture(t *testing.T, now time.Time, count int) PrivacyGroupFixture {
	return newPrivacyGroupFixture(t, now, count, identityapi.CapabilityRealmDiscovery)
}

func NewDataPrivacyGroupFixture(t *testing.T, now time.Time, count int) PrivacyGroupFixture {
	return newPrivacyGroupFixture(t, now, count, identityapi.CapabilityDataExchange)
}

func newPrivacyGroupFixture(t *testing.T, now time.Time, count int, scope identityapi.CapabilityScope) PrivacyGroupFixture {
	t.Helper()
	require.GreaterOrEqual(t, count, 2)
	secretByte, channelByte := byte(0x47), byte(0x57)
	if scope == identityapi.CapabilityDataExchange {
		secretByte, channelByte = 0x48, 0x58
	}
	issuerPrivate := privacyPrivate(0x17)
	issuerPublic := issuerPrivate.Public().(ed25519.PublicKey)
	issuer := identityprincipal.DeriveID("p", issuerPublic)
	secret, ok := identityapi.NewCapabilitySecret(bytes.Repeat([]byte{secretByte}, 32))
	require.True(t, ok)
	privates := make([]ed25519.PrivateKey, count)
	grants := make([]identityapi.CapabilityGrant, count)
	for index := range count {
		privates[index] = privacyPrivate(byte(0x27 + index*0x10))
		grants[index] = privacyGrant(t, issuerPrivate, issuer, privates[index], secret, privacyID(channelByte), byte(0x67+index), scope, now)
	}
	trusted := map[string]ed25519.PublicKey{issuer: issuerPublic}
	channels := make([]*networkprivacy.Channel, count)
	authorities := make([]*identitycapability.Service, count)
	for index := range count {
		authority, ref := privacyAuthority(t, now, grants[index], trusted, byte(0x87+index))
		authorities[index] = authority
		for other := range count {
			if other != index {
				require.NoError(t, authority.ImportSenderGrant(grants[other]))
			}
		}
		channels[index] = privacyChannel(t, now, authority, ref, grants[index].SubjectPrincipal, privates[index], scope, byte(0xa7+index))
	}
	return PrivacyGroupFixture{Channels: channels, issuer: issuerPrivate, grants: grants, authorities: authorities}
}

func (f PrivacyGroupFixture) RevokeSender(t *testing.T, receiverIndex, senderIndex int, at time.Time) {
	t.Helper()
	require.GreaterOrEqual(t, receiverIndex, 0)
	require.Less(t, receiverIndex, len(f.authorities))
	require.GreaterOrEqual(t, senderIndex, 0)
	require.Less(t, senderIndex, len(f.grants))
	grant := f.grants[senderIndex]
	revocation, err := identitycapability.SignRevocation(identityapi.CapabilityRevocation{
		Version: 1, GrantID: grant.GrantID, IssuerPrincipal: grant.IssuerPrincipal, RevokedAt: at,
	}, f.issuer)
	require.NoError(t, err)
	require.NoError(t, f.authorities[receiverIndex].ApplyRevocation(revocation))
}

func newPrivacyFixture(t *testing.T, now time.Time, scope identityapi.CapabilityScope) DiscoveryPrivacyFixture {
	t.Helper()
	secretByte := byte(0x47)
	channelByte := byte(0x57)
	if scope == identityapi.CapabilityDataExchange {
		secretByte = 0x48
		channelByte = 0x58
	}
	issuerPrivate := privacyPrivate(0x17)
	issuerPublic := issuerPrivate.Public().(ed25519.PublicKey)
	issuer := identityprincipal.DeriveID("p", issuerPublic)
	senderPrivate := privacyPrivate(0x27)
	receiverPrivate := privacyPrivate(0x37)
	secret, ok := identityapi.NewCapabilitySecret(bytes.Repeat([]byte{secretByte}, 32))
	require.True(t, ok)
	channelID := privacyID(channelByte)
	senderGrant := privacyGrant(t, issuerPrivate, issuer, senderPrivate, secret, channelID, 0x67, scope, now)
	receiverGrant := privacyGrant(t, issuerPrivate, issuer, receiverPrivate, secret, channelID, 0x77, scope, now)
	trusted := map[string]ed25519.PublicKey{issuer: issuerPublic}
	senderAuthority, senderRef := privacyAuthority(t, now, senderGrant, trusted, 0x87)
	receiverAuthority, receiverRef := privacyAuthority(t, now, receiverGrant, trusted, 0x97)
	require.NoError(t, senderAuthority.ImportSenderGrant(receiverGrant))
	require.NoError(t, receiverAuthority.ImportSenderGrant(senderGrant))
	return DiscoveryPrivacyFixture{
		Sender:   privacyChannel(t, now, senderAuthority, senderRef, senderGrant.SubjectPrincipal, senderPrivate, scope, 0xa7),
		Receiver: privacyChannel(t, now, receiverAuthority, receiverRef, receiverGrant.SubjectPrincipal, receiverPrivate, scope, 0xb7),
		issuer:   issuerPrivate, grant: senderGrant, receiver: receiverAuthority,
	}
}

func (f DiscoveryPrivacyFixture) RevokeSender(t *testing.T, at time.Time) {
	t.Helper()
	revocation, err := identitycapability.SignRevocation(identityapi.CapabilityRevocation{
		Version: 1, GrantID: f.grant.GrantID, IssuerPrincipal: f.grant.IssuerPrincipal, RevokedAt: at,
	}, f.issuer)
	require.NoError(t, err)
	require.NoError(t, f.receiver.ApplyRevocation(revocation))
}

func privacyAuthority(t *testing.T, now time.Time, grant identityapi.CapabilityGrant, trusted map[string]ed25519.PublicKey, keyByte byte) (*identitycapability.Service, identityapi.CapabilityRef) {
	t.Helper()
	authority, err := identitycapability.NewService(
		filepath.Join(t.TempDir(), "capabilities.db"), bytes.Repeat([]byte{keyByte}, 32),
		grant.SubjectPrincipal, trusted, policy.New(policy.Config{}), func() time.Time { return now },
	)
	require.NoError(t, err)
	ref, err := authority.ImportGrant(grant)
	require.NoError(t, err)
	return authority, ref
}

func privacyChannel(t *testing.T, now time.Time, authority *identitycapability.Service, ref identityapi.CapabilityRef, subject string, signer ed25519.PrivateKey, scope identityapi.CapabilityScope, keyByte byte) *networkprivacy.Channel {
	t.Helper()
	replay, err := networkprivacy.NewDurableReplayLedger(
		filepath.Join(t.TempDir(), "privacy-replay.db"), bytes.Repeat([]byte{keyByte}, 32), 128, 512,
	)
	require.NoError(t, err)
	channel, err := networkprivacy.NewChannel(networkprivacy.ChannelConfig{
		Resolver: authority, Authorizer: authority, Replay: replay,
		Reference: ref, Subject: subject, Scope: scope,
		Signer: func() ed25519.PrivateKey { return signer }, Clock: func() time.Time { return now },
	})
	require.NoError(t, err)
	return channel
}

func privacyGrant(t *testing.T, issuerPrivate ed25519.PrivateKey, issuer string, subjectPrivate ed25519.PrivateKey, secret identityapi.CapabilitySecret, channelID [16]byte, grantByte byte, scope identityapi.CapabilityScope, now time.Time) identityapi.CapabilityGrant {
	t.Helper()
	grant := identityapi.CapabilityGrant{
		Version: 1, ChannelID: channelID, Generation: 1, Secret: secret,
		GrantID: privacyID(grantByte), IssuerPrincipal: issuer,
		SubjectPrincipal: identityprincipal.DeriveID("p", subjectPrivate.Public().(ed25519.PublicKey)),
		Permissions:      identityapi.CapabilitySubscribe | identityapi.CapabilityPublish | identityapi.CapabilityStoreFetch,
		Scope:            scope,
		NotBefore:        now.Add(-time.Hour), NotAfter: now.Add(time.Hour),
	}
	signed, err := identitycapability.SignGrant(grant, issuerPrivate)
	require.NoError(t, err)
	return signed
}

func privacyPrivate(value byte) ed25519.PrivateKey {
	return ed25519.NewKeyFromSeed(bytes.Repeat([]byte{value}, ed25519.SeedSize))
}

func privacyID(value byte) [16]byte {
	var id [16]byte
	for index := range id {
		id[index] = value
	}
	return id
}
