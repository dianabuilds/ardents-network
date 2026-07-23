// Package placement owns reservation, commitment, lease, and capacity state.
// It does not own transfer execution or payload persistence.
package placement

import (
	"fmt"
	"time"

	"ardents/internal/content/payload"
	identityprincipal "ardents/internal/identity/principal"
)

func (r *Receiver) Commit(request CommitRequest, auth PeerAuthorization) (Commitment, error) {
	reservation, existing, err := r.beginCommit(request, auth)
	if err != nil || existing.State != "" {
		return existing, err
	}
	if r.cfg.Store == nil {
		r.finishCommitAttempt(request.OperationID)
		return Commitment{}, fmt.Errorf("durable replica store is unavailable")
	}
	if err := r.cfg.Store(request.Blob, append([]byte(nil), request.Ciphertext...), request.LeaseExpiresAt.UTC()); err != nil {
		r.finishCommitAttempt(request.OperationID)
		return Commitment{}, fmt.Errorf("durable replica store: %w", err)
	}
	return r.recordCommit(reservation, request)
}

func (r *Receiver) beginCommit(request CommitRequest, auth PeerAuthorization) (reservation, Commitment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	reservation, ok := r.reservations[request.OperationID]
	if !ok || reservation.result.Status != ReservationAccepted {
		return reservation, Commitment{}, fmt.Errorf("accepted reservation not found")
	}
	if !reservation.principal.Equal(auth.NodePrincipal) || authorizationDenial(auth) != "" {
		return reservation, Commitment{}, fmt.Errorf("commit peer is not authorized")
	}
	if !tokenMatches(reservation.tokenDigest, request.Token) {
		return reservation, Commitment{}, fmt.Errorf("commit token is invalid")
	}
	if existing, ok := r.commitments[request.OperationID]; ok {
		if sameCommit(existing, request, auth.NodePrincipal) {
			return reservation, existing, nil
		}
		return reservation, Commitment{}, fmt.Errorf("commit replay conflicts with existing operation")
	}
	if r.committing[request.OperationID] {
		return reservation, Commitment{}, fmt.Errorf("commit operation is already in progress")
	}
	now := r.cfg.Now().UTC()
	if !now.Before(reservation.result.ExpiresAt) {
		return reservation, Commitment{}, fmt.Errorf("reservation expired")
	}
	if err := validateCommit(reservation.offer, request, now); err != nil {
		return reservation, Commitment{}, err
	}
	r.committing[request.OperationID] = true
	return reservation, Commitment{}, nil
}

func (r *Receiver) recordCommit(reservation reservation, request CommitRequest) (Commitment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := r.cfg.Now().UTC()
	commitment := Commitment{
		OperationID: request.OperationID, IntentVersion: reservation.offer.IntentVersion,
		BlobID: request.Blob.ID, CID: request.Blob.CID, TargetNode: r.cfg.NodePrincipal,
		Size: int64(len(request.Ciphertext)), State: CommitmentActive,
		LeaseStartsAt: now, LastObservedAt: now, LeaseExpiresAt: request.LeaseExpiresAt.UTC(),
	}
	r.commitments[request.OperationID] = commitment
	delete(r.committing, request.OperationID)
	r.reserved -= reservation.offer.EncryptedSize
	r.used += reservation.offer.EncryptedSize
	return commitment, nil
}

func (r *Receiver) finishCommitAttempt(operationID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.committing, operationID)
}

func validateCommit(offer ReservationOffer, request CommitRequest, now time.Time) error {
	if len(request.Ciphertext) == 0 {
		return fmt.Errorf("commit ciphertext is incomplete")
	}
	if !request.Blob.Encrypted {
		return fmt.Errorf("plaintext replica commit is forbidden")
	}
	if request.Blob.Cipher != payload.AES256GCMCipher {
		return fmt.Errorf("unsupported replica cipher")
	}
	if int64(len(request.Ciphertext)) != offer.EncryptedSize || request.Blob.Size != offer.EncryptedSize {
		return fmt.Errorf("commit encrypted size does not match reservation")
	}
	hash, cid, err := payload.DeriveIdentity(request.Ciphertext)
	if err != nil {
		return err
	}
	if cid != offer.CID || request.Blob.ID != cid || request.Blob.CID != cid || request.Blob.Hash != hash {
		return fmt.Errorf("commit content identity does not match reservation")
	}
	if !request.LeaseExpiresAt.After(now) || request.LeaseExpiresAt.After(now.Add(offer.RequestedLease)) {
		return fmt.Errorf("commit lease expiry is invalid")
	}
	return nil
}

func sameCommit(existing Commitment, request CommitRequest, principal identityprincipal.ID) bool {
	return existing.OperationID == request.OperationID && existing.BlobID == request.Blob.ID &&
		existing.CID == request.Blob.CID && existing.Size == int64(len(request.Ciphertext)) &&
		existing.LeaseExpiresAt.Equal(request.LeaseExpiresAt.UTC()) && principal.String() != ""
}
