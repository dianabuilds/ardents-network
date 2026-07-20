package data

import (
	"fmt"
	"time"

	placementpkg "ardents/internal/data/placement"
)

func (s *Service) ReserveReplica(offer placementpkg.ReservationOffer, auth placementpkg.PeerAuthorization) (placementpkg.ReservationResult, error) {
	result, err := s.placement.Reserve(offer, auth)
	if err != nil {
		return placementpkg.ReservationResult{}, err
	}
	if err := s.Save(); err != nil {
		return placementpkg.ReservationResult{}, err
	}
	return result, nil
}

func (s *Service) CommitReplica(request placementpkg.CommitRequest, auth placementpkg.PeerAuthorization) (placementpkg.Commitment, error) {
	commitment, err := s.placement.Commit(request, auth)
	if err != nil {
		return placementpkg.Commitment{}, err
	}
	if err := s.Save(); err != nil {
		return placementpkg.Commitment{}, err
	}
	return commitment, nil
}

func (s *Service) ReplicaPlacementState() placementpkg.State {
	return s.placement.Snapshot()
}

func (s *Service) ReplicaCapacity() placementpkg.Capacity {
	return s.placement.Capacity()
}

func (s *Service) ObserveReplicaCommitment(commitment placementpkg.Commitment, now time.Time) (placementpkg.Commitment, error) {
	observed, err := s.placement.ObserveCommitment(commitment, now)
	if err != nil {
		return placementpkg.Commitment{}, err
	}
	if err := s.Save(); err != nil {
		return placementpkg.Commitment{}, err
	}
	return observed, nil
}

func (s *Service) HasCurrentReplicaCommitment(blobID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	for _, commitment := range s.placement.Snapshot().Commitments {
		if commitment.BlobID == blobID && commitment.PeerID == s.localNodeID &&
			commitment.State == placementpkg.CommitmentActive && now.Before(commitment.LeaseExpiresAt) &&
			s.hasLocalPayloadLocked(blobID) {
			return true
		}
	}
	return false
}

func (s *Service) RenewReplicaCommitment(operationID string, observedAt, expiresAt time.Time) (placementpkg.Commitment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	commitment, ok := s.placement.Snapshot().Commitments[operationID]
	if !ok {
		return placementpkg.Commitment{}, fmt.Errorf("replica commitment not found")
	}
	blob, ok := s.blobs.Get(commitment.BlobID)
	if !ok || !s.hasLocalPayloadLocked(commitment.BlobID) {
		return placementpkg.Commitment{}, fmt.Errorf("replica payload is not locally available")
	}
	commitment, err := s.placement.RenewCommitment(operationID, observedAt, expiresAt)
	if err != nil {
		return placementpkg.Commitment{}, err
	}
	blob.ExpiresAt = commitment.LeaseExpiresAt
	s.blobs.Put(blob)
	return commitment, s.saveLocked()
}

func (s *Service) MarkReplicaCommitment(operationID, state string, observedAt time.Time, reason string) (placementpkg.Commitment, error) {
	marked, err := s.placement.MarkCommitment(operationID, state, observedAt)
	if err != nil {
		return placementpkg.Commitment{}, err
	}
	if err := s.Save(); err != nil {
		return placementpkg.Commitment{}, err
	}
	return marked, nil
}
