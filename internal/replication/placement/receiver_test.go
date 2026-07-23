package placement_test

import (
	"crypto/ed25519"
	"crypto/sha256"
	"testing"
	"time"

	model "ardents/internal/content/catalog"
	"ardents/internal/content/payload"
	identityprincipal "ardents/internal/identity/principal"
	"ardents/internal/replication/placement"

	"github.com/stretchr/testify/require"
)

func TestReceiverReserveCommitAndDuplicateAreIdempotent(t *testing.T) {
	now := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	stored := 0
	receiver := placement.NewReceiver(placement.ReceiverConfig{
		NodePrincipal: testPrincipal("target"), MaxBytes: 1024, Now: func() time.Time { return now },
		Store: func(_ model.Blob, _ []byte, _ time.Time) error { stored++; return nil },
	})
	offer, blob, ciphertext := validOffer(t, now, "op-1", "nonce-1", []byte("encrypted-ciphertext"))
	auth := validAuth("source")

	accepted, err := receiver.Reserve(offer, auth)
	require.NoError(t, err)
	require.Equal(t, placement.ReservationAccepted, accepted.Status)
	duplicate, err := receiver.Reserve(offer, auth)
	require.NoError(t, err)
	require.Equal(t, accepted, duplicate)

	commit, err := receiver.Commit(placement.CommitRequest{
		OperationID: offer.OperationID, Token: accepted.Token, Blob: blob,
		Ciphertext: ciphertext, LeaseExpiresAt: now.Add(24 * time.Hour),
	}, auth)
	require.NoError(t, err)
	require.Equal(t, placement.CommitmentActive, commit.State)
	require.Equal(t, 1, stored)

	again, err := receiver.Commit(placement.CommitRequest{
		OperationID: offer.OperationID, Token: accepted.Token, Blob: blob,
		Ciphertext: ciphertext, LeaseExpiresAt: now.Add(24 * time.Hour),
	}, auth)
	require.NoError(t, err)
	require.Equal(t, commit, again)
	require.Equal(t, 1, stored)
}

func TestReceiverRejectsQuotaAndUntrustedPeer(t *testing.T) {
	now := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	receiver := placement.NewReceiver(placement.ReceiverConfig{
		NodePrincipal: testPrincipal("target"), MaxBytes: 4, Now: func() time.Time { return now },
	})
	offer, _, _ := validOffer(t, now, "op-quota", "nonce-q", []byte("ciphertext"))

	denied, err := receiver.Reserve(offer, validAuth("source"))
	require.NoError(t, err)
	require.Equal(t, placement.ReservationRejected, denied.Status)
	require.Equal(t, placement.ReasonQuota, denied.Reason)

	receiver = placement.NewReceiver(placement.ReceiverConfig{
		NodePrincipal: testPrincipal("target"), MaxBytes: 1024, Now: func() time.Time { return now },
	})
	auth := validAuth("source")
	auth.Trusted = false
	denied, err = receiver.Reserve(offer, auth)
	require.NoError(t, err)
	require.Equal(t, placement.ReasonUntrusted, denied.Reason)
}

func TestReceiverRejectsWrongCIDPartialCommitExpiryAndReplay(t *testing.T) {
	now := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	receiver := placement.NewReceiver(placement.ReceiverConfig{
		NodePrincipal: testPrincipal("target"), MaxBytes: 4096, Now: func() time.Time { return now },
	})
	offer, blob, ciphertext := validOffer(t, now, "op-guard", "nonce-g", []byte("ciphertext"))
	auth := validAuth("source")
	accepted, err := receiver.Reserve(offer, auth)
	require.NoError(t, err)

	_, err = receiver.Commit(placement.CommitRequest{
		OperationID: offer.OperationID, Token: accepted.Token, Blob: blob,
		LeaseExpiresAt: now.Add(time.Hour),
	}, auth)
	require.ErrorContains(t, err, "ciphertext")

	wrong := blob
	_, wrongReference, err := payload.DeriveIdentity([]byte("wrong-cid"))
	require.NoError(t, err)
	wrong.Reference = wrongReference
	_, err = receiver.Commit(placement.CommitRequest{
		OperationID: offer.OperationID, Token: accepted.Token, Blob: wrong,
		Ciphertext: ciphertext, LeaseExpiresAt: now.Add(time.Hour),
	}, auth)
	require.ErrorContains(t, err, "content identity")

	replay := validAuth("different-peer")
	_, err = receiver.Commit(placement.CommitRequest{
		OperationID: offer.OperationID, Token: accepted.Token, Blob: blob,
		Ciphertext: ciphertext, LeaseExpiresAt: now.Add(time.Hour),
	}, replay)
	require.ErrorContains(t, err, "peer")

	now = now.Add(3 * time.Minute)
	_, err = receiver.Commit(placement.CommitRequest{
		OperationID: offer.OperationID, Token: accepted.Token, Blob: blob,
		Ciphertext: ciphertext, LeaseExpiresAt: now.Add(time.Hour),
	}, auth)
	require.ErrorContains(t, err, "expired")
}

func TestReceiverRejectsMutatedDuplicateReservationAndExcessiveLease(t *testing.T) {
	now := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	receiver := placement.NewReceiver(placement.ReceiverConfig{
		NodePrincipal: testPrincipal("target"), MaxBytes: 4096, Now: func() time.Time { return now },
	})
	offer, _, _ := validOffer(t, now, "op-duplicate", "nonce-d", []byte("ciphertext"))
	auth := validAuth("source")
	_, err := receiver.Reserve(offer, auth)
	require.NoError(t, err)
	offer.EncryptedSize++
	_, err = receiver.Reserve(offer, auth)
	require.ErrorContains(t, err, "replay")

	other, _, _ := validOffer(t, now, "op-lease", "nonce-l", []byte("other"))
	other.RequestedLease = 25 * time.Hour
	denied, err := receiver.Reserve(other, auth)
	require.NoError(t, err)
	require.Equal(t, placement.ReasonLease, denied.Reason)

	unsupported, _, _ := validOffer(t, now, "op-version", "nonce-v", []byte("version"))
	unsupported.ProtocolVersion++
	denied, err = receiver.Reserve(unsupported, auth)
	require.NoError(t, err)
	require.Equal(t, placement.ReasonUnsupported, denied.Reason)

	large, _, _ := validOffer(t, now, "op-large", "nonce-large", make([]byte, placement.MaxInlineReplicaBytes+1))
	denied, err = receiver.Reserve(large, auth)
	require.NoError(t, err)
	require.Equal(t, placement.ReasonUnsupported, denied.Reason)
}

func TestReceiverCapacityTracksReservationsAndCommittedBytes(t *testing.T) {
	now := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	receiver := placement.NewReceiver(placement.ReceiverConfig{
		NodePrincipal: testPrincipal("target"), MaxBytes: 4096, Now: func() time.Time { return now },
		Store: func(_ model.Blob, _ []byte, _ time.Time) error { return nil },
	})
	offer, blob, ciphertext := validOffer(t, now, "op-capacity", "nonce-c", []byte("ciphertext"))

	initial := receiver.Capacity()
	require.Equal(t, int64(4096), initial.FreeBytes)
	accepted, err := receiver.Reserve(offer, validAuth("source"))
	require.NoError(t, err)
	reserved := receiver.Capacity()
	require.Equal(t, offer.EncryptedSize, reserved.ReservedBytes)
	require.Equal(t, int64(4096)-offer.EncryptedSize, reserved.FreeBytes)

	_, err = receiver.Commit(placement.CommitRequest{
		OperationID: offer.OperationID, Token: accepted.Token, Blob: blob,
		Ciphertext: ciphertext, LeaseExpiresAt: now.Add(time.Hour),
	}, validAuth("source"))
	require.NoError(t, err)
	committed := receiver.Capacity()
	require.Zero(t, committed.ReservedBytes)
	require.Equal(t, offer.EncryptedSize, committed.UsedBytes)
	require.Equal(t, int64(4096)-offer.EncryptedSize, committed.FreeBytes)
}

func TestReceiverAcceptsCanonicalEncryptedChunkLimit(t *testing.T) {
	now := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	receiver := placement.NewReceiver(placement.ReceiverConfig{
		NodePrincipal: testPrincipal("target"), MaxBytes: placement.MaxInlineReplicaBytes + 1024, Now: func() time.Time { return now },
	})
	offer, _, _ := validOffer(t, now, "op-canonical-chunk", "nonce-chunk", make([]byte, placement.MaxInlineReplicaBytes))
	result, err := receiver.Reserve(offer, validAuth("source"))
	require.NoError(t, err)
	require.Equal(t, placement.ReservationAccepted, result.Status)
}

func TestReceiverSnapshotRedactsTokenAndRestoreRejectsPartialReservations(t *testing.T) {
	now := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	receiver := placement.NewReceiver(placement.ReceiverConfig{
		NodePrincipal: testPrincipal("target"), MaxBytes: 4096, Now: func() time.Time { return now },
	})
	offer, _, _ := validOffer(t, now, "op-persisted", "nonce-persisted", []byte("ciphertext"))
	accepted, err := receiver.Reserve(offer, validAuth("source"))
	require.NoError(t, err)
	require.NotEmpty(t, accepted.Token)

	canonical := receiver.Snapshot()
	stored := canonical.Reservations[offer.OperationID]
	require.Empty(t, stored.Result.Token)
	require.NotEmpty(t, stored.TokenDigest)

	valid := placement.NewReceiver(placement.ReceiverConfig{Now: func() time.Time { return now }})
	require.NoError(t, valid.Restore(canonical))
	require.Equal(t, offer.EncryptedSize, valid.Capacity().ReservedBytes)

	tests := map[string]func(*placement.StoredReservation){
		"missing token digest": func(item *placement.StoredReservation) { item.TokenDigest = "" },
		"plaintext token":      func(item *placement.StoredReservation) { item.Result.Token = accepted.Token },
		"missing protocol":     func(item *placement.StoredReservation) { item.Offer.ProtocolVersion = 0 },
		"result mismatch":      func(item *placement.StoredReservation) { item.Result.OperationID = "other" },
		"unknown status":       func(item *placement.StoredReservation) { item.Result.Status = "unknown" },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			state := canonical
			state.Reservations = map[string]placement.StoredReservation{offer.OperationID: stored}
			item := state.Reservations[offer.OperationID]
			mutate(&item)
			state.Reservations[offer.OperationID] = item
			restored := placement.NewReceiver(placement.ReceiverConfig{Now: func() time.Time { return now }})
			require.Error(t, restored.Restore(state))
			require.Empty(t, restored.Snapshot().Reservations)
		})
	}
}

func TestSelectTargetsUsesOnlyFreshEligibleCapacityAndDiversity(t *testing.T) {
	now := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	candidates := []placement.Candidate{
		{NodePrincipal: testPrincipal("owner"), Trusted: true, CapabilityValid: true, PolicyAllowed: true, Usable: true, CapacityBytes: 128 * 1024, ObservedAt: now},
		{NodePrincipal: testPrincipal("good-b"), FailureDomain: "b", Trusted: true, CapabilityValid: true, PolicyAllowed: true, Usable: true, CapacityBytes: 128 * 1024, ObservedAt: now},
		{NodePrincipal: testPrincipal("good-a"), FailureDomain: "a", Trusted: true, CapabilityValid: true, PolicyAllowed: true, Usable: true, CapacityBytes: 128 * 1024, ObservedAt: now},
		{NodePrincipal: testPrincipal("stale"), Trusted: true, CapabilityValid: true, PolicyAllowed: true, Usable: true, CapacityBytes: 128 * 1024, ObservedAt: now.Add(-16 * time.Minute)},
		{NodePrincipal: testPrincipal("untrusted"), CapabilityValid: true, PolicyAllowed: true, Usable: true, CapacityBytes: 128 * 1024, ObservedAt: now},
		{NodePrincipal: testPrincipal("unavailable"), Trusted: true, PolicyAllowed: true, Usable: true, DenialReason: placement.ReasonObservation},
	}

	decision := placement.SelectTargets(placement.SelectionRequest{
		OwnerPrincipal: testPrincipal("owner"), EncryptedSize: 50, Count: 2, Now: now,
	}, candidates)
	require.Equal(t, []identityprincipal.ID{testPrincipal("good-a"), testPrincipal("good-b")}, decision.SelectedNodePrincipals())
	require.Contains(t, decision.Denials, placement.Denial{NodePrincipal: testPrincipal("unavailable"), Reason: placement.ReasonObservation})
}

func validOffer(t *testing.T, now time.Time, operationID, nonce string, ciphertext []byte) (placement.ReservationOffer, model.Blob, []byte) {
	t.Helper()
	hash, cid, err := payload.DeriveIdentity(ciphertext)
	require.NoError(t, err)
	blob := model.Blob{Reference: cid, Hash: hash, Size: int64(len(ciphertext)), Encrypted: true, Cipher: payload.AES256GCMCipher}
	return placement.ReservationOffer{
		OperationID: operationID, ProtocolVersion: placement.ReplicaProtocolVersion,
		IntentVersion: 1, ContentReference: cid,
		EncryptedSize: int64(len(ciphertext)), RequestedLease: 24 * time.Hour,
		ExpiresAt: now.Add(2 * time.Minute), Nonce: nonce,
	}, blob, ciphertext
}

func validAuth(peer string) placement.PeerAuthorization {
	return placement.PeerAuthorization{
		NodePrincipal: testPrincipal(peer), Authenticated: true, Trusted: true,
		CapabilityValid: true, PolicyAllowed: true,
	}
}

func testPrincipal(label string) identityprincipal.ID {
	seed := sha256.Sum256([]byte(label))
	private := ed25519.NewKeyFromSeed(seed[:])
	principal, err := identityprincipal.FromEd25519PublicKey(private.Public().(ed25519.PublicKey))
	if err != nil {
		panic(err)
	}
	return principal
}
