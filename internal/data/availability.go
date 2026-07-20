package data

import (
	"fmt"
	"time"

	availabilitypkg "ardents/internal/data/availability"
	datapayload "ardents/internal/data/payload"
	"ardents/internal/data/placement"
)

const replicaObservationFreshness = 15 * time.Minute

func (s *Service) GetAvailability(rootManifestID string) (AvailabilitySnapshot, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	snapshot, ok := s.availability.Snapshots[rootManifestID]
	return snapshot, ok
}

func (s *Service) ReconcileAvailability(rootManifestID string, now time.Time) (AvailabilityReconcileResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	intent, ok := s.intentForRootLocked(rootManifestID)
	if !ok {
		return AvailabilityReconcileResult{}, fmt.Errorf("replica intent not found")
	}
	blobIDs, err := s.resolveManifestBlobIDsLocked(rootManifestID)
	if err != nil {
		return AvailabilityReconcileResult{}, err
	}
	result := s.reconcileIntentLocked(intent, blobIDs, now.UTC())
	if err := s.saveLocked(); err != nil {
		return AvailabilityReconcileResult{}, err
	}
	return result, nil
}

func (s *Service) reconcileIntentLocked(intent ReplicaIntent, blobIDs []string, now time.Time) AvailabilityReconcileResult {
	placementState := s.placement.Snapshot()
	snapshot := availabilitypkg.Snapshot{
		RootManifestID: intent.RootManifestID, IntentID: intent.ID, IntentVersion: intent.Version,
		DesiredCopies: intent.DesiredCopies, MinimumCopies: intent.MinimumCopies,
		ValidCopies: intent.DesiredCopies, CurrentLeases: intent.DesiredCopies, ObservedAt: now,
	}
	activeRepairIDs := map[string]bool{}
	for _, blobID := range blobIDs {
		truth := s.blobReplicaTruthLocked(blobID, intent.Version, now, placementState)
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
			repair := s.ensureRepairLocked(intent, blobID, ordinal, now, truth.lossEligibleAt)
			activeRepairIDs[repair.ID] = true
		}
	}
	s.completeObsoleteRepairsLocked(intent, activeRepairIDs, now)
	due := s.dueRepairsLocked(intent, activeRepairIDs, now)
	snapshot.PendingRepairs, snapshot.State, snapshot.Reason = s.repairAvailabilityLocked(snapshot, activeRepairIDs)
	s.availability.Snapshots[intent.RootManifestID] = snapshot
	return availabilitypkg.ReconcileResult{Snapshot: snapshot, DueRepairs: due}
}

type blobReplicaTruth struct {
	valid, currentLeases, stale, expired, corrupt int
	nextExpiry, lossEligibleAt                    time.Time
}

func (s *Service) blobReplicaTruthLocked(blobID string, intentVersion uint64, now time.Time, state placement.State) blobReplicaTruth {
	truth := blobReplicaTruth{}
	peers := map[string]bool{}
	if s.localReplicaValidLocked(blobID, now) {
		peers[s.localNodeID] = true
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

func (s *Service) localReplicaValidLocked(blobID string, now time.Time) bool {
	blob, ok := s.blobs.Get(blobID)
	return ok && datapayload.StateRequiresLocalPayload(blob.State) &&
		(blob.ExpiresAt.IsZero() || blob.ExpiresAt.After(now)) && s.hasLocalPayloadLocked(blobID)
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
