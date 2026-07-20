package placement

import (
	"encoding/base64"
	"time"
)

type StoredReservation struct {
	Offer       ReservationOffer  `json:"offer"`
	PeerID      string            `json:"peer_id"`
	Result      ReservationResult `json:"result"`
	TokenDigest string            `json:"token_digest,omitempty"`
}

type State struct {
	Reserved     int64                        `json:"reserved"`
	Used         int64                        `json:"used"`
	Reservations map[string]StoredReservation `json:"reservations"`
	Commitments  map[string]Commitment        `json:"commitments"`
}

func (r *Receiver) Snapshot() State {
	r.mu.Lock()
	defer r.mu.Unlock()
	state := State{Reserved: r.reserved, Used: r.used, Reservations: map[string]StoredReservation{}, Commitments: map[string]Commitment{}}
	for id, item := range r.reservations {
		state.Reservations[id] = StoredReservation{
			Offer: item.offer, PeerID: item.peerID, Result: item.result,
			TokenDigest: base64.RawStdEncoding.EncodeToString(item.tokenDigest[:]),
		}
	}
	for id, item := range r.commitments {
		state.Commitments[id] = item
	}
	return state
}

func (r *Receiver) Restore(state State) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := r.cfg.Now().UTC()
	r.reserved, r.used = 0, state.Used
	r.reservations = map[string]reservation{}
	r.commitments = map[string]Commitment{}
	r.committing = map[string]bool{}
	for id, stored := range state.Reservations {
		item, err := restoreReservation(stored)
		if err != nil {
			return err
		}
		if item.result.Status == ReservationAccepted && !now.Before(item.result.ExpiresAt) {
			continue
		}
		r.reservations[id] = item
		if item.result.Status == ReservationAccepted {
			r.reserved += item.offer.EncryptedSize
		}
	}
	for id, item := range state.Commitments {
		r.commitments[id] = item
	}
	return nil
}

func restoreReservation(stored StoredReservation) (reservation, error) {
	item := reservation{offer: stored.Offer, peerID: stored.PeerID, result: stored.Result}
	if stored.TokenDigest == "" {
		return item, nil
	}
	raw, err := base64.RawStdEncoding.DecodeString(stored.TokenDigest)
	if err != nil || len(raw) != len(item.tokenDigest) {
		return reservation{}, &StateError{"reservation token digest is invalid"}
	}
	copy(item.tokenDigest[:], raw)
	return item, nil
}

func (r *Receiver) SetNodeID(nodeID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cfg.NodeID = nodeID
}

func (r *Receiver) ObserveCommitment(commitment Commitment, now time.Time) (Commitment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if commitment.OperationID == "" || commitment.IntentVersion == 0 || commitment.BlobID == "" ||
		commitment.CID == "" || commitment.BlobID != commitment.CID || commitment.PeerID == "" ||
		commitment.Size <= 0 || !validCommitmentState(commitment.State) {
		return Commitment{}, &StateError{"replica commitment is invalid"}
	}
	if commitment.LastObservedAt.IsZero() {
		commitment.LastObservedAt = commitment.LeaseStartsAt
	}
	if commitment.State == CommitmentActive && !commitment.LeaseExpiresAt.After(now.UTC()) {
		return Commitment{}, &StateError{"replica commitment lease is expired"}
	}
	if existing, ok := r.commitments[commitment.OperationID]; ok {
		if !sameCommitmentIdentity(existing, commitment) || commitment.LastObservedAt.Before(existing.LastObservedAt) ||
			((existing.State == CommitmentCorrupt || existing.State == CommitmentRevoked) && commitment.State == CommitmentActive) {
			return Commitment{}, &StateError{"replica commitment conflicts with existing operation"}
		}
	}
	r.commitments[commitment.OperationID] = commitment
	return commitment, nil
}

func validCommitmentState(state string) bool {
	return state == CommitmentActive || state == CommitmentStale || state == CommitmentCorrupt ||
		state == CommitmentRevoked || state == CommitmentExpired
}

func sameCommitmentIdentity(left, right Commitment) bool {
	return left.OperationID == right.OperationID && left.IntentVersion == right.IntentVersion &&
		left.BlobID == right.BlobID && left.CID == right.CID && left.PeerID == right.PeerID &&
		left.Size == right.Size && left.LeaseStartsAt.Equal(right.LeaseStartsAt)
}

type StateError struct{ Detail string }

func (e *StateError) Error() string { return e.Detail }
