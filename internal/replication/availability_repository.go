// Package replication owns replica intent, placement coordination, leases, health, and repair.
// It does not own payload storage or transfer protocol.
package replication

import (
	"fmt"
	"time"

	"ardents/internal/content/catalog"
	identityprincipal "ardents/internal/identity/principal"
	"ardents/internal/replication/availability"
	"ardents/internal/replication/placement"
)

const replicaObservationFreshness = 15 * time.Minute

func (r *Repository) GetAvailability(owner identityprincipal.ID, rootManifestID string) (availability.Snapshot, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	snapshot, ok := r.availability.Snapshots[catalog.RecordStorageKey(owner, rootManifestID)]
	return snapshot, ok
}

func (r *Repository) ReconcileAvailability(owner identityprincipal.ID, rootManifestID string, now time.Time) (availability.ReconcileResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var selected availability.ReplicaIntent
	found := false
	for _, intent := range r.availability.Intents {
		if intent.RootManifestOwner.Equal(owner) && intent.RootManifestID == rootManifestID && (!found || intent.Version > selected.Version) {
			selected, found = intent, true
		}
	}
	if !found {
		return availability.ReconcileResult{}, fmt.Errorf("replica intent not found")
	}
	return r.reconcileAvailabilityLocked(selected, now)
}

func (r *Repository) reconcileAvailabilityLocked(intent availability.ReplicaIntent, now time.Time) (availability.ReconcileResult, error) {
	rootManifestID := intent.RootManifestID
	blobIDs, err := r.resolveManifestBlobIDsLocked(intent.RootManifestOwner, rootManifestID)
	if err != nil {
		return availability.ReconcileResult{}, err
	}
	references := make([]catalog.ContentReference, 0, len(blobIDs))
	for _, blobID := range blobIDs {
		reference, parseErr := catalog.ParseContentReference(blobID)
		if parseErr != nil {
			return availability.ReconcileResult{}, fmt.Errorf("manifest contains invalid content reference: %w", parseErr)
		}
		references = append(references, reference)
	}
	result := r.reconcileIntentLocked(intent, references, now.UTC())
	if err := r.saveLocked(); err != nil {
		return availability.ReconcileResult{}, err
	}
	return result, nil
}

func (r *Repository) reconcileIntentLocked(intent availability.ReplicaIntent, references []catalog.ContentReference, now time.Time) availability.ReconcileResult {
	placementState := r.placement.Snapshot()
	snapshot := availability.Snapshot{
		RootManifestOwner: intent.RootManifestOwner, RootManifestID: intent.RootManifestID, IntentID: intent.ID, IntentVersion: intent.Version,
		DesiredCopies: intent.DesiredCopies, MinimumCopies: intent.MinimumCopies,
		ValidCopies: intent.DesiredCopies, CurrentLeases: intent.DesiredCopies, ObservedAt: now,
	}
	activeRepairIDs := map[string]bool{}
	for _, reference := range references {
		truth := r.blobReplicaTruthLocked(reference, intent.Version, now, placementState)
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
			repair := r.ensureRepairLocked(intent, reference, ordinal, now, truth.lossEligibleAt)
			activeRepairIDs[repair.ID] = true
		}
	}
	r.completeObsoleteRepairsLocked(intent, activeRepairIDs, now)
	due := r.dueRepairsLocked(intent, activeRepairIDs, now)
	snapshot.PendingRepairs, snapshot.State, snapshot.Reason = r.repairAvailabilityLocked(snapshot, activeRepairIDs)
	for key, current := range r.availability.Snapshots {
		if current.RootManifestOwner.Equal(intent.RootManifestOwner) && current.RootManifestID == intent.RootManifestID {
			delete(r.availability.Snapshots, key)
		}
	}
	r.availability.Snapshots[catalog.RecordStorageKey(intent.RootManifestOwner, intent.RootManifestID)] = snapshot
	return availability.ReconcileResult{Snapshot: snapshot, DueRepairs: due}
}

type blobReplicaTruth struct {
	valid, currentLeases, stale, expired, corrupt int
	nextExpiry, lossEligibleAt                    time.Time
}

func (r *Repository) blobReplicaTruthLocked(reference catalog.ContentReference, intentVersion uint64, now time.Time, state placement.State) blobReplicaTruth {
	truth := blobReplicaTruth{}
	peers := map[identityprincipal.ID]bool{}
	if r.localReplicaValidLocked(reference.String(), now) {
		peers[r.localNodePrincipal] = true
		truth.valid++
	}
	for _, commitment := range state.Commitments {
		if !commitment.ContentReference.Equal(reference) || commitment.IntentVersion != intentVersion || peers[commitment.TargetNode] {
			continue
		}
		if truth.observeCommitment(commitment, now) {
			peers[commitment.TargetNode] = true
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
