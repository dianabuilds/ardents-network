package access

import (
	"testing"
	"time"

	identitycontract "ardents/api/ardents/identity/v1"
	"github.com/stretchr/testify/require"
)

func storeChallenge(index int, now time.Time, source SourceKey) storedChallenge {
	var id ChallengeID
	id[0] = byte(index)
	id[1] = byte(index >> 8)
	id[2] = byte(index >> 16)
	return storedChallenge{challenge: Challenge{ID: id, ExpiresAt: now.Add(identitycontract.ChallengeLifetime)}, source: source}
}

func TestChallengeStoreExactGlobalSourceAndRateBounds(t *testing.T) {
	now := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	store := newChallengeStore()
	for index := 0; index < identitycontract.MaxActiveChallenges; index++ {
		var source SourceKey
		source[0] = byte(index)
		source[1] = byte(index >> 8)
		item := storeChallenge(index+1, now, source)
		require.NoError(t, store.add(now, item))
	}
	var extra SourceKey
	extra[0] = 0xff
	extra[1] = 0xff
	require.ErrorIs(t, store.add(now, storeChallenge(identitycontract.MaxActiveChallenges+1, now, extra)), ErrResourceExhausted)
	store.cleanupLocked(now.Add(identitycontract.ChallengeLifetime))
	require.Empty(t, store.items)
	require.Empty(t, store.sourceCounts)

	store = newChallengeStore()
	var source SourceKey
	source[0] = 1
	for index := 0; index < identitycontract.BeginRateBurst; index++ {
		item := storeChallenge(index+1, now, source)
		require.NoError(t, store.add(now, item))
		require.True(t, store.consume(now, item))
	}
	require.ErrorIs(t, store.add(now, storeChallenge(20, now, source)), ErrResourceExhausted)
	require.ErrorIs(t, store.add(now.Add(6*time.Second-time.Nanosecond), storeChallenge(21, now.Add(6*time.Second-time.Nanosecond), source)), ErrResourceExhausted)
	require.NoError(t, store.add(now.Add(6*time.Second), storeChallenge(22, now.Add(6*time.Second), source)))
}

func TestSessionStoreExactGroupBoundExpiryAndReverseDeviceInvalidation(t *testing.T) {
	now := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	var key [32]byte
	key[0] = 1
	store := newSessionStore(key)
	var source SourceKey
	source[0] = 2
	audience := Audience{Node: "node", ProtocolMajor: 1}
	for index := 0; index < identitycontract.MaxActiveSessionsPerSourceKey; index++ {
		var secret SessionSecret
		secret[0] = byte(index + 1)
		session := Session{ID: "id", Principal: "principal", DeviceID: "device", CredentialID: string(rune('a' + index)), Binding: AuthenticationBinding{Audience: audience}, ExpiresAt: now.Add(time.Minute)}
		require.NoError(t, store.add(now, secret, session, source))
	}
	var extra SessionSecret
	extra[0] = 99
	require.ErrorIs(t, store.add(now, extra, Session{Principal: "principal", DeviceID: "device", CredentialID: "extra", Binding: AuthenticationBinding{Audience: audience}, ExpiresAt: now.Add(time.Minute)}, source), ErrResourceExhausted)
	store.invalidateDevice("device")
	require.Empty(t, store.items)
	require.Empty(t, store.byDevice)
	require.Empty(t, store.byCredential)
	require.Empty(t, store.groups)
	require.NoError(t, store.add(now, extra, Session{Principal: "principal", DeviceID: "other", CredentialID: "credential", Binding: AuthenticationBinding{Audience: audience}, ExpiresAt: now.Add(time.Second)}, source))
	_, found := store.get(now.Add(time.Second), extra)
	require.False(t, found)
}

func TestSessionStoreExactGlobalBound(t *testing.T) {
	now := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	var key [32]byte
	key[0] = 9
	store := newSessionStore(key)
	for index := 0; index < identitycontract.MaxActiveSessions; index++ {
		var secret SessionSecret
		secret[0] = byte(index)
		secret[1] = byte(index >> 8)
		secret[2] = byte(index >> 16)
		var source SourceKey
		source[0] = byte(index / identitycontract.MaxActiveSessionsPerSourceKey)
		source[1] = byte((index / identitycontract.MaxActiveSessionsPerSourceKey) >> 8)
		session := Session{Principal: "principal", DeviceID: "device", CredentialID: "credential", Binding: AuthenticationBinding{Audience: Audience{Node: "node", ProtocolMajor: uint32(index / identitycontract.MaxActiveSessionsPerSourceKey)}}, ExpiresAt: now.Add(time.Minute)}
		require.NoError(t, store.add(now, secret, session, source))
	}
	var extra SessionSecret
	extra[0] = 0xff
	extra[1] = 0xff
	extra[2] = 1
	require.ErrorIs(t, store.add(now, extra, Session{Principal: "another", DeviceID: "device", CredentialID: "credential", ExpiresAt: now.Add(time.Minute)}, SourceKey{}), ErrResourceExhausted)
}

func TestEnrollmentProofStoreIsBoundExpiringAndOneUse(t *testing.T) {
	now := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	var key, binding, wrong [32]byte
	key[0] = 1
	binding[0] = 2
	wrong[0] = 3
	var proof EnrollmentProof
	proof[0] = 4
	store := newProofStore(key)
	require.NoError(t, store.add(now, proof, binding, now.Add(time.Minute)))
	require.False(t, store.consume(now, proof, wrong))
	require.True(t, store.consume(now, proof, binding))
	require.False(t, store.consume(now, proof, binding))
	require.NoError(t, store.add(now, proof, binding, now.Add(time.Minute)))
	require.False(t, store.consume(now.Add(time.Minute), proof, binding))
}
