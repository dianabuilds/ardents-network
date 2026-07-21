package replication

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"time"

	"ardents/internal/replication/availability"
)

const maxRepairDuration = 30 * time.Minute

func (r *Repository) ensureRepairLocked(intent availability.ReplicaIntent, blobID string, ordinal int, now, leaseBarrier time.Time) availability.RepairRecord {
	id := repairID(intent.Version, blobID, ordinal)
	repair, ok := r.availability.Repairs[id]
	if !ok || repair.State == "completed" {
		lossEligibleAt := now
		if leaseBarrier.After(lossEligibleAt) {
			lossEligibleAt = leaseBarrier
		}
		repair = availability.RepairRecord{
			ID: id, IntentID: intent.ID, IntentVersion: intent.Version, RootManifestID: intent.RootManifestID,
			BlobID: blobID, MissingOrdinal: ordinal, State: "pending", StartedAt: now,
			LossEligibleAt: lossEligibleAt, DeadlineAt: lossEligibleAt.Add(maxRepairDuration), NextAttemptAt: now,
		}
		r.availability.Repairs[id] = repair
	} else if repair.LossEligibleAt.IsZero() || repair.DeadlineAt.IsZero() {
		if repair.StartedAt.IsZero() {
			repair.StartedAt = now
		}
		if repair.LossEligibleAt.IsZero() {
			repair.LossEligibleAt = repair.StartedAt
		}
		repair.DeadlineAt = repair.LossEligibleAt.Add(maxRepairDuration)
		r.availability.Repairs[id] = repair
	} else if leaseBarrier.After(repair.LossEligibleAt) {
		repair.LossEligibleAt = leaseBarrier
		repair.DeadlineAt = leaseBarrier.Add(maxRepairDuration)
		r.availability.Repairs[id] = repair
	}
	return repair
}

func (r *Repository) completeObsoleteRepairsLocked(intent availability.ReplicaIntent, active map[string]bool, now time.Time) {
	for id, repair := range r.availability.Repairs {
		if repair.IntentID != intent.ID || repair.IntentVersion != intent.Version || active[id] || repair.State == "completed" {
			continue
		}
		repair.State, repair.FinishedAt, repair.Reason = "completed", new(now), "replication target satisfied"
		r.availability.Repairs[id] = repair
	}
}

func (r *Repository) dueRepairsLocked(_ availability.ReplicaIntent, active map[string]bool, now time.Time) []availability.RepairRecord {
	out := make([]availability.RepairRecord, 0, len(active))
	for id := range active {
		repair := r.availability.Repairs[id]
		if (repair.State == "pending" || repair.State == "running") && !repair.DeadlineAt.IsZero() && !now.Before(repair.DeadlineAt) {
			repair.State, repair.FinishedAt, repair.Reason = "failed", new(now), "repair deadline exhausted"
			r.availability.Repairs[id] = repair
			continue
		}
		if repair.State == "pending" && !repair.NextAttemptAt.After(now) {
			out = append(out, repair)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (r *Repository) RecordRepairFailure(repairID string, at time.Time, _ string) (availability.RepairRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	repair, ok := r.availability.Repairs[repairID]
	if !ok || (repair.State != "pending" && repair.State != "running") {
		return availability.RepairRecord{}, fmt.Errorf("repair attempt is not pending")
	}
	at = at.UTC()
	if at.Before(repair.NextAttemptAt) {
		return availability.RepairRecord{}, fmt.Errorf("repair retry is not due")
	}
	repair.Attempts++
	if !at.Before(repair.LossEligibleAt) {
		repair.PostLeaseAttempts++
	}
	repair.LastAttemptAt = at
	repair.Reason = "repair candidate unavailable"
	if repair.PostLeaseAttempts >= 6 || (!repair.DeadlineAt.IsZero() && !at.Before(repair.DeadlineAt)) {
		repair.State, repair.FinishedAt = "failed", new(at)
	} else {
		repair.State = "pending"
		repair.NextAttemptAt = at.Add(repairBackoff(repair.ID, repair.Attempts))
	}
	r.availability.Repairs[repairID] = repair
	return repair, r.saveLocked()
}

func (r *Repository) ListReplicaRepairs(rootManifestID string) []availability.RepairRecord {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]availability.RepairRecord, 0, len(r.availability.Repairs))
	for _, repair := range r.availability.Repairs {
		if rootManifestID == "" || repair.RootManifestID == rootManifestID {
			out = append(out, repair)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func repairBackoff(repairID string, attempt int) time.Duration {
	backoff := 5 * time.Second
	for index := 1; index < attempt; index++ {
		backoff *= 2
		if backoff >= 5*time.Minute {
			return 5 * time.Minute
		}
	}
	sum := sha256.Sum256(fmt.Appendf(nil, "%s:%d", repairID, attempt))
	jitter := time.Duration(sum[0]) * (backoff / 5) / 255
	if backoff+jitter > 5*time.Minute {
		return 5 * time.Minute
	}
	return backoff + jitter
}

func (r *Repository) repairAvailabilityLocked(snapshot availability.Snapshot, active map[string]bool) (int, string, string) {
	pending, terminal := 0, len(active) > 0
	for id := range active {
		if r.availability.Repairs[id].State != "failed" {
			terminal = false
			pending++
		}
	}
	state, reason := availabilityState(snapshot, terminal && snapshot.CurrentLeases == 0)
	return pending, state, reason
}

func availabilityState(snapshot availability.Snapshot, repairTerminal bool) (string, string) {
	switch {
	case snapshot.DesiredCopies < 2 && snapshot.ValidCopies > 0:
		return "best-effort", "no multi-copy replica intent is active"
	case snapshot.ValidCopies >= snapshot.DesiredCopies:
		return "target-satisfied", "lease-backed replica target is satisfied"
	case snapshot.ValidCopies >= snapshot.MinimumCopies:
		return "degraded", "valid committed copies are below desired count"
	case snapshot.ValidCopies > 0:
		return "degraded", "validated copies remain below minimum and desired counts"
	case repairTerminal:
		return "lost", "bounded repair exhausted without a validated copy"
	default:
		return "unavailable", "valid committed copies are below minimum count"
	}
}

func repairID(intentVersion uint64, blobID string, ordinal int) string {
	sum := sha256.Sum256(fmt.Appendf(nil, "%d:%s:%d", intentVersion, blobID, ordinal))
	return hex.EncodeToString(sum[:])
}
