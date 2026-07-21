// Package replication owns replica intent, placement coordination, leases, health, and repair.
// It does not own payload storage or transfer protocol.
package replication

import (
	"fmt"
	"time"

	"ardents/internal/replication/availability"
	"ardents/internal/replication/placement"
)

const replicaObservationFreshness = 15 * time.Minute

func (r *Repository) GetAvailability(rootManifestID string) (availability.Snapshot, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	snapshot, ok := r.availability.Snapshots[rootManifestID]
	return snapshot, ok
}

func (r *Repository) ReconcileAvailability(rootManifestID string, now time.Time) (availability.ReconcileResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	intent, ok := r.intentForRootLocked(rootManifestID)
	if !ok {
		return availability.ReconcileResult{}, fmt.Errorf("replica intent not found")
	}
	blobIDs, err := r.resolveManifestBlobIDsLocked(rootManifestID)
	if err != nil {
		return availability.ReconcileResult{}, err
	}
	result := r.reconcileIntentLocked(intent, blobIDs, now.UTC())
	if err := r.saveLocked(); err != nil {
		return availability.ReconcileResult{}, err
	}
	return result, nil
}

func (r *Repository) reconcileIntentLocked(intent availability.ReplicaIntent, blobIDs []string, now time.Time) availability.ReconcileResult {
	placementState := r.placement.Snapshot()
	snapshot := availability.Snapshot{
		RootManifestID: intent.RootManifestID, IntentID: intent.ID, IntentVersion: intent.Version,
		DesiredCopies: intent.DesiredCopies, MinimumCopies: intent.MinimumCopies,
		ValidCopies: intent.DesiredCopies, CurrentLeases: intent.DesiredCopies, ObservedAt: now,
	}
	activeRepairIDs := map[string]bool{}
	for _, blobID := range blobIDs {
		truth := r.blobReplicaTruthLocked(blobID, intent.Version, now, placementState)
		if truth.valid < snapshot.ValidCopies {
			snapshot.ValidCopies = truth.valid
		}
		if truth.currentLeases < snapshot.CurrentLeases {
			snapshot.CurrentLeases = truth.currentLeases
		}
		snapshot.StaleCopies += truth.stale
		snapshot.ExpiredCopies += truth.expired
		snapshot.CorruptCopies += truth.corrupt
		if !truth.nextExpiry.IsZero() && (snapshot.NextLeaseExpiry.IsZero() || truth.nextExpiry.Before(snapshot.NextLeaseExpiry)) {
			snapshot.NextLeaseExpiry = truth.nextExpiry
		}
		for ordinal := truth.valid; ordinal < intent.DesiredCopies; ordinal++ {
			repair := r.ensureRepairLocked(intent, blobID, ordinal, now, truth.lossEligibleAt)
			activeRepairIDs[repair.ID] = true
		}
	}
	r.completeObsoleteRepairsLocked(intent, activeRepairIDs, now)
	due := r.dueRepairsLocked(intent, activeRepairIDs, now)
	snapshot.PendingRepairs, snapshot.State, snapshot.Reason = r.repairAvailabilityLocked(snapshot, activeRepairIDs)
	r.availability.Snapshots[intent.RootManifestID] = snapshot
	return availability.ReconcileResult{Snapshot: snapshot, DueRepairs: due}
}

type blobReplicaTruth struct {
	valid, currentLeases, stale, expired, corrupt int
	nextExpiry, lossEligibleAt                    time.Time
}

func (r *Repository) blobReplicaTruthLocked(blobID string, intentVersion uint64, now time.Time, state placement.State) blobReplicaTruth {
	truth := blobReplicaTruth{}
	peers := map[string]bool{}
	if r.localReplicaValidLocked(blobID, now) {
		peers[r.localNodeID] = true
		truth.valid++
	}
	for _, commitment := range state.Commitments {
		if commitment.BlobID != blobID || commitment.IntentVersion != intentVersion || peers[commitment.PeerID] {
			continue
		}
		if truth.observeCommitment(commitment, now) {
			peers[commitment.PeerID] = true
		}
	}
	return truth
}

func (r *Repository) localReplicaValidLocked(blobID string, now time.Time) bool {
	blob, ok := r.content.GetBlob(blobID)
	if !ok || (!blob.ExpiresAt.IsZero() && !blob.ExpiresAt.After(now)) {
		return false
	}
	_, err := r.content.GetBlobPayload(blobID)
	return err == nil
}

func (truth *blobReplicaTruth) observeCommitment(commitment placement.Commitment, now time.Time) bool {
	if commitment.State == placement.CommitmentCorrupt {
		truth.corrupt++
		return false
	}
	if commitment.State != placement.CommitmentActive && commitment.State != placement.CommitmentStale {
		truth.stale++
		return false
	}
	if !commitment.LeaseExpiresAt.After(now) {
		truth.expired++
		return false
	}
	truth.currentLeases++
	if truth.nextExpiry.IsZero() || commitment.LeaseExpiresAt.Before(truth.nextExpiry) {
		truth.nextExpiry = commitment.LeaseExpiresAt
	}
	if commitment.LeaseExpiresAt.After(truth.lossEligibleAt) {
		truth.lossEligibleAt = commitment.LeaseExpiresAt
	}
	observed := commitment.LastObservedAt
	if observed.IsZero() {
		observed = commitment.LeaseStartsAt
	}
	if commitment.State != placement.CommitmentActive || observed.IsZero() ||
		observed.After(now.Add(2*time.Minute)) || now.Sub(observed) > replicaObservationFreshness {
		truth.stale++
		return false
	}
	truth.valid++
	return true
}
