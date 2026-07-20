package replication

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	appdata "ardents/internal/data"
	"ardents/internal/data/placement"
)

const replicaHealthRefresh = 5 * time.Minute

func (s *Service) reconcileLoop(ctx context.Context) {
	if err := s.ReconcileOnce(ctx); err != nil {
		s.recordReconcileFailure(err)
	}
	ticker := time.NewTicker(s.cfg.RepairInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.ReconcileOnce(ctx); err != nil {
				s.recordReconcileFailure(err)
			}
		}
	}
}

func (s *Service) ReconcileOnce(ctx context.Context) error {
	intents := s.cfg.Data.ListReplicaIntents()
	if len(intents) == 0 {
		return nil
	}
	now := s.cfg.Now().UTC()
	var failures []error
	for _, intent := range intents {
		failures = append(failures, s.refreshIntentCommitments(ctx, intent, now)...)
	}
	var due []appdata.RepairRecord
	for _, intent := range intents {
		result, err := s.cfg.Data.ReconcileAvailability(intent.RootManifestID, now)
		if err != nil {
			failures = append(failures, err)
			continue
		}
		due = append(due, result.DueRepairs...)
	}
	failures = append(failures, s.runDueRepairs(ctx, due)...)
	for _, intent := range intents {
		result, err := s.cfg.Data.ReconcileAvailability(intent.RootManifestID, s.cfg.Now().UTC())
		if err != nil {
			failures = append(failures, err)
			continue
		}
		s.recordAvailability(result)
	}
	return errors.Join(failures...)
}

func (s *Service) recordAvailability(result appdata.AvailabilityReconcileResult) {
	if s.cfg.Diagnostics == nil {
		return
	}
	snapshot := result.Snapshot
	s.cfg.Diagnostics.RecordEvent("data", "availability_observed", snapshot.RootManifestID,
		"data availability reconciled", "", map[string]any{
			"state": snapshot.State, "reason": snapshot.Reason,
			"desired_copies": snapshot.DesiredCopies, "minimum_copies": snapshot.MinimumCopies,
			"valid_copies": snapshot.ValidCopies, "current_leases": snapshot.CurrentLeases,
			"stale_copies": snapshot.StaleCopies, "expired_copies": snapshot.ExpiredCopies,
			"corrupt_copies": snapshot.CorruptCopies, "pending_repairs": snapshot.PendingRepairs,
		})
}

func (s *Service) refreshIntentCommitments(ctx context.Context, intent appdata.ReplicaIntent, now time.Time) []error {
	var failures []error
	for _, commitment := range s.cfg.Data.ReplicaPlacementState().Commitments {
		if commitment.IntentVersion != intent.Version || commitment.PeerID == s.cfg.LocalNodeID || commitment.State != placement.CommitmentActive {
			continue
		}
		if err := s.refreshCommitment(ctx, commitment, intent, now); err != nil {
			failures = append(failures, err)
		}
	}
	return failures
}

func (s *Service) refreshCommitment(ctx context.Context, commitment placement.Commitment, intent appdata.ReplicaIntent, now time.Time) error {
	if !s.targetUsable(commitment.PeerID) {
		_, err := s.cfg.Data.MarkReplicaCommitment(commitment.OperationID, placement.CommitmentStale, now, "peer presence unavailable")
		return err
	}
	if now.Sub(commitment.LastObservedAt) < replicaHealthRefresh && commitment.LeaseExpiresAt.Sub(now) > intent.RenewalHorizon {
		return nil
	}
	probeCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	_, err := s.ProbeReplica(probeCtx, commitment)
	if err == nil {
		return nil
	}
	current := s.cfg.Data.ReplicaPlacementState().Commitments[commitment.OperationID]
	if current.State == placement.CommitmentCorrupt || current.State == placement.CommitmentRevoked {
		return err
	}
	_, markErr := s.cfg.Data.MarkReplicaCommitment(commitment.OperationID, placement.CommitmentStale, s.cfg.Now().UTC(), "health observation failed")
	return errors.Join(err, markErr)
}

func (s *Service) targetUsable(target string) bool {
	entry, outcome, ok := s.cfg.Discovery.Resolve(target, "node")
	return ok && outcome == "found" && s.cfg.Trust.Evaluate(entry.Record).Usable
}

func (s *Service) runDueRepairs(ctx context.Context, repairs []appdata.RepairRecord) []error {
	if len(repairs) == 0 {
		return nil
	}
	batches := batchDueRepairs(repairs)
	jobs := make(chan repairBatch)
	failures := make(chan error, len(repairs))
	var workers sync.WaitGroup
	count := s.cfg.RepairConcurrency
	if len(batches) < count {
		count = len(batches)
	}
	workers.Add(count)
	for range count {
		go func() {
			defer workers.Done()
			for batch := range jobs {
				for _, repair := range batch.repairs {
					s.runRepair(ctx, repair, failures)
				}
			}
		}()
	}
	for _, batch := range batches {
		jobs <- batch
	}
	close(jobs)
	workers.Wait()
	close(failures)
	var out []error
	for err := range failures {
		out = append(out, err)
	}
	return out
}

func (s *Service) runRepair(ctx context.Context, repair appdata.RepairRecord, failures chan<- error) {
	attemptCtx, cancel := context.WithTimeout(ctx, s.cfg.RepairAttemptTimeout)
	defer cancel()
	outcome, err := s.PlaceAvailable(attemptCtx, repair.BlobID, 1, repair.IntentVersion)
	if err != nil && retryablePlacementFailure(outcome.Decision.Denials) && waitPlacementRetry(attemptCtx) {
		_, err = s.PlaceAvailable(attemptCtx, repair.BlobID, 1, repair.IntentVersion)
	}
	if err == nil {
		if s.cfg.Diagnostics != nil {
			s.cfg.Diagnostics.RecordEvent("data", "replica_repaired", repair.ID, "replica repair committed", "", map[string]any{
				"root_manifest_id": repair.RootManifestID, "intent_version": repair.IntentVersion,
				"missing_ordinal": repair.MissingOrdinal,
			})
		}
		return
	}
	if _, recordErr := s.cfg.Data.RecordRepairFailure(repair.ID, s.cfg.Now().UTC(), safeReason(err)); recordErr != nil {
		err = errors.Join(err, recordErr)
	}
	failures <- fmt.Errorf("repair %s: %w", repair.ID, err)
}

func retryablePlacementFailure(denials []placement.Denial) bool {
	transient := false
	for _, denial := range denials {
		switch denial.Reason {
		case placement.ReasonExisting:
		case placement.ReasonObservation, reasonReplicaControlRejected:
			transient = true
		default:
			return false
		}
	}
	return transient
}

func waitPlacementRetry(ctx context.Context) bool {
	timer := time.NewTimer(capacityRetryDelay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func (s *Service) recordReconcileFailure(err error) {
	if s.cfg.Diagnostics != nil {
		s.cfg.Diagnostics.RecordEvent("data", "availability_reconcile_failed", "availability", "availability reconciliation failed", safeReason(err), nil)
	}
}
