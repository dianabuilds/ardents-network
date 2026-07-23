package placement

import (
	"crypto/subtle"
	"encoding/base64"
	"maps"
	"time"

	identityprincipal "ardents/internal/identity/principal"
)

type StoredReservation struct {
	Offer         ReservationOffer     `json:"offer"`
	NodePrincipal identityprincipal.ID `json:"node_principal"`
	Result        ReservationResult    `json:"result"`
	TokenDigest   string               `json:"token_digest,omitempty"`
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
		result := item.result
		result.Token = ""
		tokenDigest := ""
		if result.Status == ReservationAccepted {
			tokenDigest = base64.RawStdEncoding.EncodeToString(item.tokenDigest[:])
		}
		state.Reservations[id] = StoredReservation{
			Offer: item.offer, NodePrincipal: item.principal, Result: result, TokenDigest: tokenDigest,
		}
	}
	maps.Copy(state.Commitments, r.commitments)
	return state
}

func (r *Receiver) Restore(state State) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := r.cfg.Now().UTC()
	if state.Reserved < 0 || state.Used < 0 {
		return &StateError{"replica placement byte counters are invalid"}
	}
	if state.Reservations == nil || state.Commitments == nil {
		return &StateError{"replica placement collections are required"}
	}
	reserved := int64(0)
	persistedReserved := int64(0)
	reservations := map[string]reservation{}
	commitments := map[string]Commitment{}
	for id, commitment := range state.Commitments {
		if id == "" || commitment.OperationID != id || !validRetainedCommitment(commitment) {
			return &StateError{"replica commitment is invalid"}
		}
		commitments[id] = commitment
	}
	for id, stored := range state.Reservations {
		item, err := restoreReservation(stored)
		if err != nil {
			return err
		}
		if id == "" || item.offer.OperationID != id {
			return &StateError{"reservation operation binding is invalid"}
		}
		_, committed := commitments[id]
		countsAsReserved := item.result.Status == ReservationAccepted && !committed
		if countsAsReserved {
			persistedReserved += item.offer.EncryptedSize
		}
		if countsAsReserved && !now.Before(item.result.ExpiresAt) {
			continue
		}
		reservations[id] = item
		if countsAsReserved {
			reserved += item.offer.EncryptedSize
		}
	}
	if state.Reserved != persistedReserved {
		return &StateError{"replica reservation byte count is invalid"}
	}
	r.reserved, r.used = reserved, state.Used
	r.reservations, r.commitments = reservations, commitments
	r.committing = map[string]bool{}
	return nil
}

func validRetainedCommitment(commitment Commitment) bool {
	return commitment.OperationID != "" && commitment.IntentVersion != 0 && commitment.ContentReference.String() != "" &&
		commitment.TargetNode.String() != "" &&
		commitment.Size > 0 && validCommitmentState(commitment.State) && !commitment.LeaseStartsAt.IsZero() &&
		!commitment.LeaseExpiresAt.IsZero() && commitment.LeaseExpiresAt.After(commitment.LeaseStartsAt)
}

func restoreReservation(stored StoredReservation) (reservation, error) {
	item := reservation{offer: stored.Offer, principal: stored.NodePrincipal, result: stored.Result}
	if item.principal.String() == "" {
		return reservation{}, &StateError{"reservation Node Principal is invalid"}
	}
	if !validStoredOffer(item.offer) {
		return reservation{}, &StateError{"reservation offer is invalid"}
	}
	if item.result.OperationID != item.offer.OperationID || !item.result.ExpiresAt.Equal(item.offer.ExpiresAt) {
		return reservation{}, &StateError{"reservation result binding is invalid"}
	}
	if item.result.Token != "" {
		return reservation{}, &StateError{"reservation plaintext token is forbidden"}
	}
	switch item.result.Status {
	case ReservationAccepted:
		if item.result.Reason != "" || item.offer.ProtocolVersion != ReplicaProtocolVersion ||
			item.offer.EncryptedSize > MaxInlineReplicaBytes || item.offer.RequestedLease <= 0 ||
			item.offer.RequestedLease > defaultLeaseTTL || stored.TokenDigest == "" {
			return reservation{}, &StateError{"accepted reservation is invalid"}
		}
	case ReservationRejected:
		if !validRejectionReason(item.result.Reason) || stored.TokenDigest != "" {
			return reservation{}, &StateError{"rejected reservation is invalid"}
		}
		return item, nil
	default:
		return reservation{}, &StateError{"reservation status is invalid"}
	}
	raw, err := base64.RawStdEncoding.DecodeString(stored.TokenDigest)
	if err != nil || len(raw) != len(item.tokenDigest) || base64.RawStdEncoding.EncodeToString(raw) != stored.TokenDigest {
		return reservation{}, &StateError{"reservation token digest is invalid"}
	}
	copy(item.tokenDigest[:], raw)
	var zero [32]byte
	if subtle.ConstantTimeCompare(item.tokenDigest[:], zero[:]) == 1 {
		return reservation{}, &StateError{"reservation token digest is invalid"}
	}
	return item, nil
}

func validStoredOffer(offer ReservationOffer) bool {
	return offer.OperationID != "" && offer.ProtocolVersion != 0 && offer.IntentVersion != 0 &&
		offer.ContentReference.String() != "" && offer.EncryptedSize > 0 &&
		!offer.ExpiresAt.IsZero() && offer.Nonce != ""
}

func validRejectionReason(reason string) bool {
	switch reason {
	case ReasonQuota, ReasonUntrusted, ReasonCapability, ReasonPolicy, ReasonLease, ReasonUnsupported,
		ReasonObservation, ReasonExisting:
		return true
	default:
		return false
	}
}

func (r *Receiver) SetNodePrincipal(principal identityprincipal.ID) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cfg.NodePrincipal = principal
}

func (r *Receiver) ObserveCommitment(commitment Commitment, now time.Time) (Commitment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if commitment.OperationID == "" || commitment.IntentVersion == 0 || commitment.ContentReference.String() == "" ||
		commitment.TargetNode.String() == "" ||
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
		left.ContentReference.Equal(right.ContentReference) && left.TargetNode.Equal(right.TargetNode) &&
		left.Size == right.Size && left.LeaseStartsAt.Equal(right.LeaseStartsAt)
}

type StateError struct{ Detail string }

func (e *StateError) Error() string { return e.Detail }
