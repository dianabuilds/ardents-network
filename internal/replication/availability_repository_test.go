package replication

import (
	"bytes"
	"testing"
	"time"

	"ardents/internal/replication/placement"

	"github.com/stretchr/testify/require"
)

func TestReplicaIntentReconciliationPersistsRepairAndReachesTarget(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 7, 19, 20, 0, 0, 0, time.UTC)
	service := newInDir(dir)
	require.NoError(t, service.Load())
	service.SetLocalNodeID("owner-node")

	key := bytes.Repeat([]byte{0x61}, 32)
	blob, err := service.StoreEncryptedBlob(Blob{MediaType: "application/octet-stream", Retention: "durable"}, []byte("available payload"), key, "key-1")
	require.NoError(t, err)
	root, err := service.PublishManifest(Manifest{
		Kind: "blob-set", Owner: "owner-node", Encrypted: true, Retention: "durable",
		Refs: []Ref{{Kind: "blob", ID: blob.ID}},
	})
	require.NoError(t, err)

	intent, err := service.SetReplicaIntent(ReplicaIntent{
		ID: "intent-1", RootManifestID: root.ID, Version: 1,
		DesiredCopies: 2, MinimumCopies: 1, LeaseDuration: 24 * time.Hour,
		RenewalHorizon: 8 * time.Hour, Retention: "durable", CreatedAt: now, UpdatedAt: now,
	})
	require.NoError(t, err)
	require.Equal(t, uint64(1), intent.Version)

	result, err := service.ReconcileAvailability(root.ID, now)
	require.NoError(t, err)
	require.Equal(t, "degraded", result.Snapshot.State)
	require.Equal(t, 1, result.Snapshot.ValidCopies)
	require.Equal(t, 2, result.Snapshot.DesiredCopies)
	require.Len(t, result.DueRepairs, 1)
	require.Equal(t, "pending", result.DueRepairs[0].State)

	reloaded := newInDir(dir)
	require.NoError(t, reloaded.Load())
	reloaded.SetLocalNodeID("owner-node")
	persisted, ok := reloaded.GetAvailability(root.ID)
	require.True(t, ok)
	require.Equal(t, "degraded", persisted.State)
	persistedResult, err := reloaded.ReconcileAvailability(root.ID, now)
	require.NoError(t, err)
	require.Equal(t, result.DueRepairs[0].ID, persistedResult.DueRepairs[0].ID)

	_, err = reloaded.ObserveReplicaCommitment(placement.Commitment{
		OperationID: "commit-remote-1", IntentVersion: 1, BlobID: blob.ID, CID: blob.CID,
		PeerID: "remote-node", Size: blob.Size, State: placement.CommitmentActive,
		LeaseStartsAt: now, LastObservedAt: now, LeaseExpiresAt: now.Add(24 * time.Hour),
	}, now)
	require.NoError(t, err)

	satisfied, err := reloaded.ReconcileAvailability(root.ID, now.Add(time.Minute))
	require.NoError(t, err)
	require.Equal(t, "target-satisfied", satisfied.Snapshot.State)
	require.Equal(t, 2, satisfied.Snapshot.ValidCopies)
	require.Empty(t, satisfied.DueRepairs)
}

func TestReplicaRepairFailurePersistsTerminalLossAcrossRestart(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 7, 19, 21, 0, 0, 0, time.UTC)
	service := newInDir(dir)
	require.NoError(t, service.Load())
	service.SetLocalNodeID("owner-node")
	key := bytes.Repeat([]byte{0x62}, 32)
	blob, err := service.StoreEncryptedBlob(Blob{MediaType: "application/octet-stream", Retention: "durable"}, []byte("last copy"), key, "key-1")
	require.NoError(t, err)
	root, err := service.PublishManifest(Manifest{
		Kind: "blob-set", Owner: "owner-node", Encrypted: true, Retention: "durable",
		Refs: []Ref{{Kind: "blob", ID: blob.ID}},
	})
	require.NoError(t, err)
	_, err = service.SetReplicaIntent(ReplicaIntent{
		ID: "intent-loss", RootManifestID: root.ID, Version: 1,
		DesiredCopies: 1, MinimumCopies: 1, LeaseDuration: 24 * time.Hour,
		RenewalHorizon: 8 * time.Hour, Retention: "durable", CreatedAt: now, UpdatedAt: now,
	})
	require.NoError(t, err)
	require.NoError(t, service.Save())

	_, err = service.DropBlob(blob.ID)
	require.NoError(t, err)
	result, err := service.ReconcileAvailability(root.ID, now)
	require.NoError(t, err)
	require.Equal(t, "unavailable", result.Snapshot.State)
	require.Len(t, result.DueRepairs, 1)

	repair := result.DueRepairs[0]
	for attempt := 1; attempt <= 6; attempt++ {
		repair, err = service.RecordRepairFailure(repair.ID, repair.NextAttemptAt, "no eligible validated source")
		require.NoError(t, err)
		require.Equal(t, attempt, repair.Attempts)
	}
	require.Equal(t, "failed", repair.State)

	reloaded := newInDir(dir)
	require.NoError(t, reloaded.Load())
	reloaded.SetLocalNodeID("owner-node")
	lost, err := reloaded.ReconcileAvailability(root.ID, repair.LastAttemptAt)
	require.NoError(t, err)
	require.Equal(t, "lost", lost.Snapshot.State)
	require.Equal(t, "bounded repair exhausted without a validated copy", lost.Snapshot.Reason)
	require.Empty(t, lost.DueRepairs)
}

func TestReplicaAvailabilityIgnoresFreshSourceWithoutCommitment(t *testing.T) {
	now := time.Date(2026, 7, 19, 22, 0, 0, 0, time.UTC)
	service := newInDir(t.TempDir())
	require.NoError(t, service.Load())
	blob, err := service.AnnounceRemoteBlob(Blob{
		ID: "uncommitted-blob", CID: "uncommitted-blob", MediaType: "application/octet-stream", State: "available-remote",
	})
	require.NoError(t, err)
	_, err = service.ObserveBlobSource(blob.ID, BlobSourceRecord{
		NodeID: "announcing-peer", Trust: SourceTrust{Valid: true, Trusted: true, Usable: true},
		Usable: true, Transport: "waku", LastSeenAt: now,
	})
	require.NoError(t, err)
	root, err := service.PublishManifest(Manifest{Kind: "blob-set", Refs: []Ref{{Kind: "blob", ID: blob.ID}}})
	require.NoError(t, err)
	_, err = service.SetReplicaIntent(ReplicaIntent{
		ID: "intent-uncommitted", RootManifestID: root.ID, Version: 1, DesiredCopies: 1, MinimumCopies: 1,
		LeaseDuration: 24 * time.Hour, RenewalHorizon: 8 * time.Hour, CreatedAt: now, UpdatedAt: now,
	})
	require.NoError(t, err)

	result, err := service.ReconcileAvailability(root.ID, now)
	require.NoError(t, err)
	require.Zero(t, result.Snapshot.ValidCopies)
	require.Equal(t, "unavailable", result.Snapshot.State)
}

func TestReplicaAvailabilityDoesNotDeclareLossDuringCurrentLeasePartition(t *testing.T) {
	now := time.Date(2026, 7, 19, 23, 0, 0, 0, time.UTC)
	service := newInDir(t.TempDir())
	require.NoError(t, service.Load())
	blob, err := service.AnnounceRemoteBlob(Blob{
		ID: "partitioned-blob", CID: "partitioned-blob", MediaType: "application/octet-stream", State: "available-remote",
	})
	require.NoError(t, err)
	root, err := service.PublishManifest(Manifest{Kind: "blob-set", Refs: []Ref{{Kind: "blob", ID: blob.ID}}})
	require.NoError(t, err)
	_, err = service.SetReplicaIntent(ReplicaIntent{
		ID: "intent-partition", RootManifestID: root.ID, Version: 1, DesiredCopies: 1, MinimumCopies: 1,
		LeaseDuration: 24 * time.Hour, RenewalHorizon: 8 * time.Hour, CreatedAt: now, UpdatedAt: now,
	})
	require.NoError(t, err)
	leaseExpiry := now.Add(24 * time.Hour)
	_, err = service.ObserveReplicaCommitment(placement.Commitment{
		OperationID: "partitioned-commitment", IntentVersion: 1, BlobID: blob.ID, CID: blob.CID,
		PeerID: "partitioned-peer", Size: 1, State: placement.CommitmentActive,
		LeaseStartsAt: now.Add(-time.Hour), LastObservedAt: now.Add(-20 * time.Minute), LeaseExpiresAt: leaseExpiry,
	}, now)
	require.NoError(t, err)

	result, err := service.ReconcileAvailability(root.ID, now)
	require.NoError(t, err)
	require.Equal(t, "unavailable", result.Snapshot.State)
	require.Len(t, result.DueRepairs, 1)
	repair := result.DueRepairs[0]
	for range 6 {
		repair, err = service.RecordRepairFailure(repair.ID, repair.NextAttemptAt, "network partition")
		require.NoError(t, err)
	}
	require.Equal(t, 0, repair.PostLeaseAttempts)
	require.Equal(t, "pending", repair.State)
	require.Equal(t, leaseExpiry.Add(30*time.Minute), repair.DeadlineAt)

	partitioned, err := service.ReconcileAvailability(root.ID, repair.LastAttemptAt)
	require.NoError(t, err)
	require.Equal(t, "unavailable", partitioned.Snapshot.State)
	afterLease := leaseExpiry.Add(time.Second)
	expired, err := service.ReconcileAvailability(root.ID, afterLease)
	require.NoError(t, err)
	require.Equal(t, "unavailable", expired.Snapshot.State)
	require.Len(t, expired.DueRepairs, 1)
	repair = expired.DueRepairs[0]
	for range 6 {
		attemptAt := repair.NextAttemptAt
		if attemptAt.Before(afterLease) {
			attemptAt = afterLease
		}
		repair, err = service.RecordRepairFailure(repair.ID, attemptAt, "network partition")
		require.NoError(t, err)
	}
	require.Equal(t, 6, repair.PostLeaseAttempts)
	lost, err := service.ReconcileAvailability(root.ID, repair.LastAttemptAt)
	require.NoError(t, err)
	require.Equal(t, "lost", lost.Snapshot.State)
}

func TestReplicaRepairBecomesTerminalAtThirtyMinuteDeadline(t *testing.T) {
	now := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)
	service := newInDir(t.TempDir())
	require.NoError(t, service.Load())
	service.SetLocalNodeID("owner-node")
	blob, err := service.StoreEncryptedBlob(
		Blob{MediaType: "application/octet-stream", Retention: "durable"},
		[]byte("repair deadline payload"), bytes.Repeat([]byte{0x63}, 32), "key-1",
	)
	require.NoError(t, err)
	root, err := service.PublishManifest(Manifest{Kind: "blob-set", Refs: []Ref{{Kind: "blob", ID: blob.ID}}})
	require.NoError(t, err)
	_, err = service.SetReplicaIntent(ReplicaIntent{
		ID: "intent-deadline", RootManifestID: root.ID, Version: 1, DesiredCopies: 1, MinimumCopies: 1,
		LeaseDuration: 24 * time.Hour, RenewalHorizon: 8 * time.Hour, CreatedAt: now, UpdatedAt: now,
	})
	require.NoError(t, err)
	_, err = service.DropBlob(blob.ID)
	require.NoError(t, err)

	initial, err := service.ReconcileAvailability(root.ID, now)
	require.NoError(t, err)
	require.Len(t, initial.DueRepairs, 1)
	require.Equal(t, now.Add(30*time.Minute), initial.DueRepairs[0].DeadlineAt)
	afterDeadline, err := service.ReconcileAvailability(root.ID, now.Add(30*time.Minute+time.Second))
	require.NoError(t, err)
	require.Empty(t, afterDeadline.DueRepairs)
	require.Equal(t, "lost", afterDeadline.Snapshot.State)
}

func TestReplicaAvailabilityNeverReportsLostWhileValidatedCopyRemains(t *testing.T) {
	now := time.Date(2026, 7, 20, 0, 30, 0, 0, time.UTC)
	service := newInDir(t.TempDir())
	require.NoError(t, service.Load())
	service.SetLocalNodeID("owner-node")
	blob, err := service.StoreEncryptedBlob(
		Blob{MediaType: "application/octet-stream"}, []byte("remaining valid copy"),
		bytes.Repeat([]byte{0x64}, 32), "key-1",
	)
	require.NoError(t, err)
	root, err := service.PublishManifest(Manifest{Kind: "blob-set", Refs: []Ref{{Kind: "blob", ID: blob.ID}}})
	require.NoError(t, err)
	_, err = service.SetReplicaIntent(ReplicaIntent{
		ID: "intent-partial", RootManifestID: root.ID, Version: 1, DesiredCopies: 2, MinimumCopies: 2,
		LeaseDuration: 24 * time.Hour, RenewalHorizon: 8 * time.Hour, CreatedAt: now, UpdatedAt: now,
	})
	require.NoError(t, err)
	result, err := service.ReconcileAvailability(root.ID, now)
	require.NoError(t, err)
	require.Len(t, result.DueRepairs, 1)
	repair := result.DueRepairs[0]
	for range 6 {
		repair, err = service.RecordRepairFailure(repair.ID, repair.NextAttemptAt, "insufficient capacity")
		require.NoError(t, err)
	}

	terminal, err := service.ReconcileAvailability(root.ID, repair.LastAttemptAt)
	require.NoError(t, err)
	require.Equal(t, 1, terminal.Snapshot.ValidCopies)
	require.Equal(t, "degraded", terminal.Snapshot.State)
	require.NotEqual(t, "lost", terminal.Snapshot.State)
}

func TestReplicaAvailabilityTreatsExpiredLeaseAsUnavailable(t *testing.T) {
	now := time.Date(2026, 7, 20, 1, 0, 0, 0, time.UTC)
	service, root, blob := remoteAvailabilityFixture(t, now, "expired")
	_, err := service.ObserveReplicaCommitment(placement.Commitment{
		OperationID: "expired-commitment", IntentVersion: 1, BlobID: blob.ID, CID: blob.CID,
		PeerID: "expired-peer", Size: 1, State: placement.CommitmentActive,
		LeaseStartsAt: now.Add(-time.Hour), LastObservedAt: now, LeaseExpiresAt: now.Add(time.Minute),
	}, now)
	require.NoError(t, err)

	result, err := service.ReconcileAvailability(root.ID, now.Add(2*time.Minute))
	require.NoError(t, err)
	require.Zero(t, result.Snapshot.ValidCopies)
	require.Equal(t, 1, result.Snapshot.ExpiredCopies)
	require.Equal(t, "unavailable", result.Snapshot.State)
}

func TestReplicaAvailabilityRecoversAfterPartitionRejoin(t *testing.T) {
	now := time.Date(2026, 7, 20, 2, 0, 0, 0, time.UTC)
	service, root, blob := remoteAvailabilityFixture(t, now, "rejoin")
	commitment := placement.Commitment{
		OperationID: "rejoined-commitment", IntentVersion: 1, BlobID: blob.ID, CID: blob.CID,
		PeerID: "rejoined-peer", Size: 1, State: placement.CommitmentStale,
		LeaseStartsAt: now.Add(-time.Hour), LastObservedAt: now.Add(-20 * time.Minute), LeaseExpiresAt: now.Add(24 * time.Hour),
	}
	_, err := service.ObserveReplicaCommitment(commitment, now)
	require.NoError(t, err)
	partitioned, err := service.ReconcileAvailability(root.ID, now)
	require.NoError(t, err)
	require.Equal(t, "unavailable", partitioned.Snapshot.State)

	commitment.State, commitment.LastObservedAt = placement.CommitmentActive, now.Add(time.Minute)
	_, err = service.ObserveReplicaCommitment(commitment, commitment.LastObservedAt)
	require.NoError(t, err)
	rejoined, err := service.ReconcileAvailability(root.ID, commitment.LastObservedAt)
	require.NoError(t, err)
	require.Equal(t, "best-effort", rejoined.Snapshot.State)
	require.Equal(t, 1, rejoined.Snapshot.ValidCopies)
	require.Empty(t, rejoined.DueRepairs)
}

func TestReplicaIntentUsesConfiguredCopyDefaults(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	service := newInDirWithConfig(t.TempDir(), ContentConfig{
		DefaultDesiredReplicas: 4,
		DefaultMinimumReplicas: 2,
	})
	blob, err := service.AnnounceRemoteBlob(Blob{
		ID: "defaults-blob", CID: "defaults-blob", MediaType: "application/octet-stream", State: "available-remote",
	})
	require.NoError(t, err)
	root, err := service.PublishManifest(Manifest{Kind: "blob-set", Refs: []Ref{{Kind: "blob", ID: blob.ID}}})
	require.NoError(t, err)
	intent, err := service.SetReplicaIntent(ReplicaIntent{
		ID: "intent-defaults", RootManifestID: root.ID, Version: 1,
		LeaseDuration: time.Hour, RenewalHorizon: 10 * time.Minute,
		Retention: "replica", CreatedAt: now, UpdatedAt: now,
	})
	require.NoError(t, err)
	require.Equal(t, 4, intent.DesiredCopies)
	require.Equal(t, 2, intent.MinimumCopies)
}

func remoteAvailabilityFixture(t *testing.T, now time.Time, suffix string) (*repositoryFixture, Manifest, Blob) {
	t.Helper()
	service := newInDir(t.TempDir())
	require.NoError(t, service.Load())
	blob, err := service.AnnounceRemoteBlob(Blob{
		ID: suffix + "-blob", CID: suffix + "-blob", MediaType: "application/octet-stream", State: "available-remote",
	})
	require.NoError(t, err)
	root, err := service.PublishManifest(Manifest{Kind: "blob-set", Refs: []Ref{{Kind: "blob", ID: blob.ID}}})
	require.NoError(t, err)
	_, err = service.SetReplicaIntent(ReplicaIntent{
		ID: "intent-" + suffix, RootManifestID: root.ID, Version: 1, DesiredCopies: 1, MinimumCopies: 1,
		LeaseDuration: 24 * time.Hour, RenewalHorizon: 8 * time.Hour, CreatedAt: now, UpdatedAt: now,
	})
	require.NoError(t, err)
	return service, root, blob
}
