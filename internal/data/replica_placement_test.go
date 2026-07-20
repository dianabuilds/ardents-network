package data

import (
	"testing"
	"time"

	"ardents/internal/data/payload"
	"ardents/internal/data/placement"

	"github.com/stretchr/testify/require"
)

func TestReplicaCapacityHasBoundedSafeDefault(t *testing.T) {
	service := NewInDir(t.TempDir())
	require.Equal(t, int64(1<<30), service.ReplicaCapacity().FreeBytes)
}

func TestReplicaReservationAndCommitmentPersistAcrossRestart(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UTC()
	ciphertext := []byte("persistent-encrypted-replica")
	hash, cid, err := payload.DeriveIdentity(ciphertext)
	require.NoError(t, err)
	blob := Blob{
		ID: cid, CID: cid, Hash: hash, Size: int64(len(ciphertext)),
		MediaType: "application/octet-stream", Encrypted: true, Cipher: payload.AES256GCMCipher,
	}
	offer := placement.ReservationOffer{
		OperationID: "persistent-operation", ProtocolVersion: placement.ReplicaProtocolVersion,
		IntentVersion: 1, BlobID: cid, CID: cid,
		EncryptedSize: int64(len(ciphertext)), RequestedLease: 24 * time.Hour,
		ExpiresAt: now.Add(2 * time.Minute), Nonce: "persistent-nonce",
	}
	auth := placement.PeerAuthorization{
		PeerID: "source", Authenticated: true, Trusted: true,
		CapabilityValid: true, PolicyAllowed: true,
	}

	first := NewInDirWithConfig(dir, Config{MaxReplicaRetentionBytes: 1024})
	first.SetLocalNodeID("target")
	require.NoError(t, first.Load())
	accepted, err := first.ReserveReplica(offer, auth)
	require.NoError(t, err)
	require.NotEmpty(t, accepted.Token)

	restored := NewInDirWithConfig(dir, Config{MaxReplicaRetentionBytes: 1024})
	restored.SetLocalNodeID("target")
	require.NoError(t, restored.Load())
	duplicate, err := restored.ReserveReplica(offer, auth)
	require.NoError(t, err)
	require.Equal(t, accepted, duplicate)

	commitment, err := restored.CommitReplica(placement.CommitRequest{
		OperationID: offer.OperationID, Token: accepted.Token, Blob: blob,
		Ciphertext: ciphertext, LeaseExpiresAt: now.Add(24 * time.Hour),
	}, auth)
	require.NoError(t, err)

	final := NewInDirWithConfig(dir, Config{MaxReplicaRetentionBytes: 1024})
	final.SetLocalNodeID("target")
	require.NoError(t, final.Load())
	require.Equal(t, commitment, final.ReplicaPlacementState().Commitments[offer.OperationID])
	stored, ok := final.GetBlob(cid)
	require.True(t, ok)
	require.True(t, stored.Encrypted)
	require.Equal(t, "relay-temporary", stored.Retention)
	require.True(t, final.HasCurrentReplicaCommitment(cid))
	raw, err := final.GetBlobPayload(cid)
	require.NoError(t, err)
	require.Equal(t, ciphertext, raw)
}

func TestReplicaLeaseRenewalAndCorruptionPersistAcrossRestart(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UTC().Truncate(time.Second)
	service, commitment := committedReplicaFixture(t, dir, now)

	renewed, err := service.RenewReplicaCommitment(commitment.OperationID, now.Add(time.Minute), now.Add(24*time.Hour))
	require.NoError(t, err)
	require.Equal(t, now.Add(time.Minute), renewed.LastObservedAt)
	require.Equal(t, now.Add(24*time.Hour), renewed.LeaseExpiresAt)
	retained, ok := service.GetBlob(commitment.BlobID)
	require.True(t, ok)
	require.Equal(t, renewed.LeaseExpiresAt, retained.ExpiresAt)

	corrupt, err := service.MarkReplicaCommitment(commitment.OperationID, placement.CommitmentCorrupt, now.Add(2*time.Minute), "ciphertext cid mismatch")
	require.NoError(t, err)
	require.Equal(t, placement.CommitmentCorrupt, corrupt.State)
	require.Equal(t, "replica integrity verification failed", corrupt.HealthReason)
	_, err = service.RenewReplicaCommitment(commitment.OperationID, now.Add(3*time.Minute), now.Add(25*time.Hour))
	require.ErrorContains(t, err, "not renewable")

	reloaded := NewInDirWithConfig(dir, Config{MaxReplicaRetentionBytes: 1024})
	reloaded.SetLocalNodeID("target")
	require.NoError(t, reloaded.Load())
	require.Equal(t, corrupt, reloaded.ReplicaPlacementState().Commitments[commitment.OperationID])
}

func TestReplicaLeaseRenewalDoesNotMutateCommitmentWithoutPayload(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	service, commitment := committedReplicaFixture(t, t.TempDir(), now)
	_, err := service.DropBlob(commitment.BlobID)
	require.NoError(t, err)
	require.False(t, service.HasCurrentReplicaCommitment(commitment.BlobID))

	_, err = service.RenewReplicaCommitment(commitment.OperationID, now.Add(time.Minute), now.Add(24*time.Hour))
	require.ErrorContains(t, err, "payload is not locally available")
	require.Equal(t, commitment, service.ReplicaPlacementState().Commitments[commitment.OperationID])
}

func committedReplicaFixture(t *testing.T, dir string, now time.Time) (*Service, placement.Commitment) {
	t.Helper()
	ciphertext := []byte("renewable-encrypted-replica")
	hash, cid, err := payload.DeriveIdentity(ciphertext)
	require.NoError(t, err)
	blob := Blob{ID: cid, CID: cid, Hash: hash, Size: int64(len(ciphertext)), MediaType: "application/octet-stream", Encrypted: true, Cipher: payload.AES256GCMCipher}
	offer := placement.ReservationOffer{
		OperationID: "renewable-operation", ProtocolVersion: placement.ReplicaProtocolVersion,
		IntentVersion: 1, BlobID: cid, CID: cid, EncryptedSize: int64(len(ciphertext)),
		RequestedLease: 24 * time.Hour, ExpiresAt: now.Add(2 * time.Minute), Nonce: "renewable-nonce",
	}
	auth := placement.PeerAuthorization{PeerID: "source", Authenticated: true, Trusted: true, CapabilityValid: true, PolicyAllowed: true}
	service := NewInDirWithConfig(dir, Config{MaxReplicaRetentionBytes: 1024})
	service.SetLocalNodeID("target")
	require.NoError(t, service.Load())
	accepted, err := service.ReserveReplica(offer, auth)
	require.NoError(t, err)
	commitment, err := service.CommitReplica(placement.CommitRequest{
		OperationID: offer.OperationID, Token: accepted.Token, Blob: blob,
		Ciphertext: ciphertext, LeaseExpiresAt: now.Add(23 * time.Hour),
	}, auth)
	require.NoError(t, err)
	return service, commitment
}
