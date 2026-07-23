package messaging

import (
	"bytes"
	"crypto/ed25519"
	"path/filepath"
	"testing"
	"time"

	identityapi "ardents/internal/identity"

	"github.com/stretchr/testify/require"
)

func TestChannelResolvesEveryPublishAndReceiveWithoutCapabilityCache(t *testing.T) {
	fixture := newEnvelopeFixture(t, true)
	senderResolver := &channelResolver{resolved: fixture.senderResolved}
	receiverResolver := &channelResolver{resolved: fixture.receiverResolved}
	sender := newTestChannel(t, senderResolver, fixture.receiverAuthority, fixture.senderResolved, fixture.senderPrivate, 0xc1)
	receiver := newTestChannel(t, receiverResolver, fixture.receiverAuthority, fixture.receiverResolved, fixture.senderPrivate, 0xd1)

	first, err := sender.Seal(MessageClassDiscoveryRecord, 1, []byte("first"))
	require.NoError(t, err)
	_, err = receiver.StoreContentTopic()
	require.NoError(t, err)
	opened, err := receiver.Open(first)
	require.NoError(t, err)
	require.Equal(t, []byte("first"), opened.Payload)
	_, err = sender.Seal(MessageClassDiscoveryRecord, 1, []byte("second"))
	require.NoError(t, err)

	require.Equal(t, []identityapi.CapabilityPermission{
		identityapi.CapabilityPublish, identityapi.CapabilityPublish,
	}, senderResolver.permissions)
	require.Equal(t, []identityapi.CapabilityPermission{
		identityapi.CapabilityStoreFetch, identityapi.CapabilitySubscribe,
	}, receiverResolver.permissions)
}

func TestChannelFailsClosedWhenChannelGrantResolutionIsRevoked(t *testing.T) {
	fixture := newEnvelopeFixture(t, true)
	resolver := &channelResolver{resolved: fixture.senderResolved, err: channelFailure{"privacy.channel_grant.revoked"}}
	channel := newTestChannel(t, resolver, fixture.receiverAuthority, fixture.senderResolved, fixture.senderPrivate, 0xe1)

	_, err := channel.Seal(MessageClassDiscoveryRecord, 1, []byte("denied"))
	require.Error(t, err)
	require.Equal(t, "privacy.channel_grant.revoked", CodeOf(err))
}

type channelResolver struct {
	resolved    identityapi.ResolvedCapability
	err         error
	permissions []identityapi.CapabilityPermission
}

func (r *channelResolver) ResolveCapability(use identityapi.CapabilityUse) (identityapi.ResolvedCapability, error) {
	r.permissions = append(r.permissions, use.Permission)
	if r.err != nil {
		return identityapi.ResolvedCapability{}, r.err
	}
	return r.resolved, nil
}

type channelFailure struct{ code string }

func (e channelFailure) Error() string       { return "capability denied" }
func (e channelFailure) FailureCode() string { return e.code }

func newTestChannel(t *testing.T, resolver identityapi.CapabilityResolver, authorizer identityapi.CapabilitySenderAuthorizer, resolved identityapi.ResolvedCapability, signer ed25519.PrivateKey, keyByte byte) *Channel {
	t.Helper()
	replay, err := NewDurableReplayLedger(filepath.Join(t.TempDir(), "replay.db"), bytes.Repeat([]byte{keyByte}, 32), 16, 64)
	require.NoError(t, err)
	channel, err := NewChannel(ChannelConfig{
		Resolver: resolver, Authorizer: authorizer, Replay: replay,
		Reference: resolved.Ref, Subject: resolved.Subject, Scope: resolved.Scope,
		Signer: func() ed25519.PrivateKey { return signer }, Clock: func() time.Time { return envelopeTestNow },
	})
	require.NoError(t, err)
	return channel
}
