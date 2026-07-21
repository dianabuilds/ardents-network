package placement

import (
	"fmt"
	"time"
)

func (r *Receiver) RenewCommitment(operationID string, observedAt, expiresAt time.Time) (Commitment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	commitment, ok := r.commitments[operationID]
	if !ok {
		return Commitment{}, fmt.Errorf("replica commitment not found")
	}
	if commitment.State != CommitmentActive {
		return Commitment{}, fmt.Errorf("replica commitment is not renewable")
	}
	observedAt, expiresAt = observedAt.UTC(), expiresAt.UTC()
	if observedAt.Before(commitment.LastObservedAt) || !expiresAt.After(observedAt) || expiresAt.After(observedAt.Add(defaultLeaseTTL)) {
		return Commitment{}, fmt.Errorf("replica renewal lease is invalid")
	}
	commitment.LastObservedAt = observedAt
	commitment.LeaseExpiresAt = expiresAt
	commitment.HealthReason = ""
	r.commitments[operationID] = commitment
	return commitment, nil
}

func (r *Receiver) MarkCommitment(operationID, state string, observedAt time.Time) (Commitment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	commitment, ok := r.commitments[operationID]
	if !ok {
		return Commitment{}, fmt.Errorf("replica commitment not found")
	}
	if state != CommitmentStale && state != CommitmentCorrupt && state != CommitmentRevoked && state != CommitmentExpired {
		return Commitment{}, fmt.Errorf("replica commitment health state is invalid")
	}
	observedAt = observedAt.UTC()
	if observedAt.Before(commitment.LastObservedAt) {
		return Commitment{}, fmt.Errorf("replica commitment observation is stale")
	}
	commitment.State = state
	commitment.LastObservedAt = observedAt
	commitment.HealthReason = healthReason(state)
	r.commitments[operationID] = commitment
	return commitment, nil
}

func healthReason(state string) string {
	switch state {
	case CommitmentCorrupt:
		return "replica integrity verification failed"
	case CommitmentRevoked:
		return "replica authorization revoked"
	case CommitmentExpired:
		return "replica lease expired"
	default:
		return "replica observation is stale"
	}
}
