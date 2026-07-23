package replication

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"ardents/internal/content/payload"
	"ardents/internal/replication/placement"
	"ardents/internal/storage"

	"github.com/stretchr/testify/require"
)

func TestReplicaCapacityHasBoundedSafeDefault(t *testing.T) {
	service := newInDir(t.TempDir())
	require.Equal(t, int64(1<<30), service.ReplicaCapacity().FreeBytes)
}

func TestReplicaReservationAndCommitmentPersistAcrossRestart(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UTC()
	ciphertext := []byte("persistent-encrypted-replica")
	hash, cid, err := payload.DeriveIdentity(ciphertext)
	require.NoError(t, err)
	blob := Blob{
		Reference: cid, Hash: hash, Size: int64(len(ciphertext)),
		MediaType: "application/octet-stream", Encrypted: true, Cipher: payload.AES256GCMCipher,
	}
	offer := placement.ReservationOffer{
		OperationID: "persistent-operation", ProtocolVersion: placement.ReplicaProtocolVersion,
		IntentVersion: 1, ContentReference: cid,
		EncryptedSize: int64(len(ciphertext)), RequestedLease: 24 * time.Hour,
		ExpiresAt: now.Add(2 * time.Minute), Nonce: "persistent-nonce",
	}
	auth := placement.PeerAuthorization{
		NodePrincipal: replicationTestPrincipal("source"), Authenticated: true, Trusted: true,
		CapabilityValid: true, PolicyAllowed: true,
	}

	first := newInDirWithConfig(dir, ContentConfig{MaxReplicaRetentionBytes: 1024})
	first.SetLocalNodeID("target")
	require.NoError(t, first.Load())
	accepted, err := first.ReserveReplica(offer, auth)
	require.NoError(t, err)
	require.NotEmpty(t, accepted.Token)

	restored := newInDirWithConfig(dir, ContentConfig{MaxReplicaRetentionBytes: 1024})
	restored.SetLocalNodeID("target")
	require.NoError(t, restored.Load())
	duplicate, err := restored.ReserveReplica(offer, auth)
	require.NoError(t, err)
	require.Equal(t, placement.ReservationAccepted, duplicate.Status)
	require.Empty(t, duplicate.Token)

	commitment, err := restored.CommitReplica(placement.CommitRequest{
		OperationID: offer.OperationID, Token: accepted.Token, Blob: blob,
		Ciphertext: ciphertext, LeaseExpiresAt: now.Add(24 * time.Hour),
	}, auth)
	require.NoError(t, err)

	final := newInDirWithConfig(dir, ContentConfig{MaxReplicaRetentionBytes: 1024})
	final.SetLocalNodeID("target")
	require.NoError(t, final.Load())
	require.Equal(t, commitment, final.ReplicaPlacementState().Commitments[offer.OperationID])
	stored, ok := final.GetBlob(cid.String())
	require.True(t, ok)
	require.True(t, stored.Encrypted)
	require.Equal(t, "relay-temporary", stored.Retention)
	require.True(t, final.HasCurrentReplicaCommitment(cid.String()))
	raw, err := final.GetBlobPayload(cid.String())
	require.NoError(t, err)
	require.Equal(t, ciphertext, raw)
}

func TestReplicationStateRejectsMissingAvailabilityCollections(t *testing.T) {
	dir := t.TempDir()
	principal := replicationTestPrincipal("not-restored")
	require.NoError(t, storage.SaveJSON(storage.PathInDir(dir), "replication", "state", map[string]any{
		"schema_version": 2,
		"placement": map[string]any{
			"reserved": 0, "used": 1, "reservations": map[string]any{}, "commitments": map[string]any{
				"operation": placement.Commitment{
					OperationID: "operation", IntentVersion: 1, ContentReference: replicationTestReference(t, "cid"), TargetNode: principal,
					Size: 1, State: placement.CommitmentActive, LeaseStartsAt: time.Now().UTC(),
					LastObservedAt: time.Now().UTC(), LeaseExpiresAt: time.Now().UTC().Add(time.Hour),
				},
			},
		},
		"availability": map[string]any{},
	}))
	service := newInDir(dir)
	err := service.Load()
	require.ErrorContains(t, err, "availability collections are required")
	require.Empty(t, service.ReplicaPlacementState().Reservations)
	require.Empty(t, service.ReplicaPlacementState().Commitments)
}

func TestReplicationStateRejectsObsoletePeerIdentityFields(t *testing.T) {
	principal := replicationTestPrincipal("target")
	reference := replicationTestReference(t, "cid")
	for _, oldField := range []string{"peer_id", "PeerID"} {
		t.Run(oldField, func(t *testing.T) {
			dir := t.TempDir()
			commitment := map[string]any{
				"operation_id": "operation", "intent_version": 1, "content_reference": reference.String(),
				oldField: principal.String(), "size": 1, "state": placement.CommitmentActive,
				"lease_starts_at": time.Now().UTC(), "last_observed_at": time.Now().UTC(),
				"lease_expires_at": time.Now().UTC().Add(time.Hour),
			}
			require.NoError(t, storage.SaveJSON(storage.PathInDir(dir), "replication", "state", map[string]any{
				"schema_version": 2,
				"placement":      map[string]any{"reserved": 0, "used": 0, "reservations": map[string]any{}, "commitments": map[string]any{"operation": commitment}},
				"availability":   map[string]any{"intents": map[string]any{}, "snapshots": map[string]any{}, "repairs": map[string]any{}},
			}))

			service := newInDir(dir)
			err := service.Load()
			require.ErrorContains(t, err, "unknown field")
			require.Empty(t, service.ReplicaPlacementState().Commitments)
		})
	}
	t.Run("malformed target_node", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, storage.SaveJSON(storage.PathInDir(dir), "replication", "state", map[string]any{
			"schema_version": 2,
			"placement": map[string]any{"reserved": 0, "used": 0, "reservations": map[string]any{}, "commitments": map[string]any{
				"operation": map[string]any{"operation_id": "operation", "intent_version": 1, "content_reference": reference.String(), "target_node": "not-a-principal"},
			}},
			"availability": map[string]any{"intents": map[string]any{}, "snapshots": map[string]any{}, "repairs": map[string]any{}},
		}))
		service := newInDir(dir)
		require.Error(t, service.Load())
		require.Empty(t, service.ReplicaPlacementState().Commitments)
	})
}

func TestReplicaCommitmentJSONUsesCanonicalTargetNode(t *testing.T) {
	commitment := placement.Commitment{OperationID: "operation", IntentVersion: 1, ContentReference: replicationTestReference(t, "cid"), TargetNode: replicationTestPrincipal("target")}
	raw, err := json.Marshal(commitment)
	require.NoError(t, err)
	require.Contains(t, string(raw), `"target_node":"p1_`)
	require.Contains(t, string(raw), `"content_reference":"bafkrei`)
	require.False(t, strings.Contains(string(raw), "peer_id"))
	require.False(t, strings.Contains(string(raw), "PeerID"))
	require.False(t, strings.Contains(string(raw), "blob_id"))
	require.False(t, strings.Contains(string(raw), `"cid"`))
}

func TestReplicationStateRejectsObsoleteContentIdentityAndSchema(t *testing.T) {
	principal := replicationTestPrincipal("target")
	reference := replicationTestReference(t, "cid")
	for _, oldField := range []string{"blob_id", "BlobID", "cid", "CID"} {
		t.Run(oldField, func(t *testing.T) {
			dir := t.TempDir()
			commitment := map[string]any{
				"operation_id": "operation", "intent_version": 1, oldField: reference.String(),
				"target_node": principal.String(), "size": 1, "state": placement.CommitmentActive,
				"lease_starts_at": time.Now().UTC(), "last_observed_at": time.Now().UTC(), "lease_expires_at": time.Now().UTC().Add(time.Hour),
			}
			require.NoError(t, storage.SaveJSON(storage.PathInDir(dir), "replication", "state", map[string]any{
				"schema_version": 2,
				"placement":      map[string]any{"reserved": 0, "used": 0, "reservations": map[string]any{}, "commitments": map[string]any{"operation": commitment}},
				"availability":   map[string]any{"intents": map[string]any{}, "snapshots": map[string]any{}, "repairs": map[string]any{}},
			}))
			require.ErrorContains(t, newInDir(dir).Load(), "unknown field")
		})
	}
	for name, repair := range map[string]map[string]any{
		"missing repair reference": {"id": "repair"},
		"repair map key mismatch":  {"id": "other", "content_reference": reference.String()},
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			require.NoError(t, storage.SaveJSON(storage.PathInDir(dir), "replication", "state", map[string]any{
				"schema_version": 2,
				"placement":      map[string]any{"reserved": 0, "used": 0, "reservations": map[string]any{}, "commitments": map[string]any{}},
				"availability":   map[string]any{"intents": map[string]any{}, "snapshots": map[string]any{}, "repairs": map[string]any{"repair": repair}},
			}))
			require.ErrorContains(t, newInDir(dir).Load(), "repair Content Reference binding is invalid")
		})
	}
	dir := t.TempDir()
	require.NoError(t, storage.SaveJSON(storage.PathInDir(dir), "replication", "state", map[string]any{
		"schema_version": 1,
		"placement":      map[string]any{"reserved": 0, "used": 0, "reservations": map[string]any{}, "commitments": map[string]any{}},
		"availability":   map[string]any{"intents": map[string]any{}, "snapshots": map[string]any{}, "repairs": map[string]any{}},
	}))
	require.ErrorContains(t, newInDir(dir).Load(), "schema is unsupported")
}

func TestReplicaLeaseRenewalAndCorruptionPersistAcrossRestart(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UTC().Truncate(time.Second)
	service, commitment := committedReplicaFixture(t, dir, now)

	renewed, err := service.RenewReplicaCommitment(commitment.OperationID, now.Add(time.Minute), now.Add(24*time.Hour))
	require.NoError(t, err)
	require.Equal(t, now.Add(time.Minute), renewed.LastObservedAt)
	require.Equal(t, now.Add(24*time.Hour), renewed.LeaseExpiresAt)
	retained, ok := service.GetBlob(commitment.ContentReference.String())
	require.True(t, ok)
	require.Equal(t, renewed.LeaseExpiresAt, retained.ExpiresAt)

	corrupt, err := service.MarkReplicaCommitment(commitment.OperationID, placement.CommitmentCorrupt, now.Add(2*time.Minute), "ciphertext cid mismatch")
	require.NoError(t, err)
	require.Equal(t, placement.CommitmentCorrupt, corrupt.State)
	require.Equal(t, "replica integrity verification failed", corrupt.HealthReason)
	_, err = service.RenewReplicaCommitment(commitment.OperationID, now.Add(3*time.Minute), now.Add(25*time.Hour))
	require.ErrorContains(t, err, "not renewable")

	reloaded := newInDirWithConfig(dir, ContentConfig{MaxReplicaRetentionBytes: 1024})
	reloaded.SetLocalNodeID("target")
	require.NoError(t, reloaded.Load())
	require.Equal(t, corrupt, reloaded.ReplicaPlacementState().Commitments[commitment.OperationID])
}

func TestReplicaLeaseRenewalDoesNotMutateCommitmentWithoutPayload(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	service, commitment := committedReplicaFixture(t, t.TempDir(), now)
	_, err := service.DropBlob(commitment.ContentReference.String())
	require.NoError(t, err)
	require.False(t, service.HasCurrentReplicaCommitment(commitment.ContentReference.String()))

	_, err = service.RenewReplicaCommitment(commitment.OperationID, now.Add(time.Minute), now.Add(24*time.Hour))
	require.ErrorContains(t, err, "payload is not locally available")
	require.Equal(t, commitment, service.ReplicaPlacementState().Commitments[commitment.OperationID])
}

func committedReplicaFixture(t *testing.T, dir string, now time.Time) (*repositoryFixture, placement.Commitment) {
	t.Helper()
	ciphertext := []byte("renewable-encrypted-replica")
	hash, cid, err := payload.DeriveIdentity(ciphertext)
	require.NoError(t, err)
	blob := Blob{Reference: cid, Hash: hash, Size: int64(len(ciphertext)), MediaType: "application/octet-stream", Encrypted: true, Cipher: payload.AES256GCMCipher}
	offer := placement.ReservationOffer{
		OperationID: "renewable-operation", ProtocolVersion: placement.ReplicaProtocolVersion,
		IntentVersion: 1, ContentReference: cid, EncryptedSize: int64(len(ciphertext)),
		RequestedLease: 24 * time.Hour, ExpiresAt: now.Add(2 * time.Minute), Nonce: "renewable-nonce",
	}
	auth := placement.PeerAuthorization{NodePrincipal: replicationTestPrincipal("source"), Authenticated: true, Trusted: true, CapabilityValid: true, PolicyAllowed: true}
	service := newInDirWithConfig(dir, ContentConfig{MaxReplicaRetentionBytes: 1024})
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
