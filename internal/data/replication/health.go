package replication

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"ardents/internal/data/payload"
	"ardents/internal/data/placement"
)

const replicaLeaseDuration = 24 * time.Hour

func (s *Service) ProbeReplica(ctx context.Context, commitment placement.Commitment) (placement.Commitment, error) {
	if commitment.State != placement.CommitmentActive || commitment.PeerID == "" || commitment.PeerID == s.cfg.LocalNodeID {
		return placement.Commitment{}, fmt.Errorf("replica health target is invalid")
	}
	operationID, _, err := operationIdentity()
	if err != nil {
		return placement.Commitment{}, err
	}
	responses, unregister, err := s.cfg.Exchange.RegisterReplicaResponses(operationID)
	if err != nil {
		return placement.Commitment{}, err
	}
	defer unregister()
	now := s.cfg.Now().UTC()
	requestedExpiry := now.Add(replicaLeaseDuration)
	body := healthQueryBody{Commitment: commitment, RequestedLeaseExpiresAt: requestedExpiry}
	if err := s.publishControl(ctx, actionHealthQuery, operationID, commitment.PeerID, body); err != nil {
		return placement.Commitment{}, s.markProbeStale(commitment, err)
	}
	wire, err := s.awaitControl(ctx, responses, actionHealthResult, commitment.PeerID)
	if err != nil {
		return placement.Commitment{}, s.markProbeStale(commitment, err)
	}
	var result healthResultBody
	if err := json.Unmarshal(wire.Body, &result); err != nil {
		return placement.Commitment{}, s.markProbeStale(commitment, err)
	}
	if err := validateHealthResult(commitment, requestedExpiry, result, now); err != nil {
		return placement.Commitment{}, s.markProbeStale(commitment, err)
	}
	observed, err := s.cfg.Data.ObserveReplicaCommitment(result.Commitment, now)
	if err != nil {
		return placement.Commitment{}, err
	}
	if result.Status != "healthy" {
		return observed, fmt.Errorf("replica health rejected: %s", result.Reason)
	}
	return observed, nil
}

func (s *Service) markProbeStale(commitment placement.Commitment, cause error) error {
	_, markErr := s.cfg.Data.MarkReplicaCommitment(
		commitment.OperationID, placement.CommitmentStale, s.cfg.Now().UTC(), "health observation failed",
	)
	if s.cfg.Diagnostics != nil {
		s.cfg.Diagnostics.RecordEvent("data", "replica_health_stale", commitment.OperationID,
			"replica health observation failed", "data.replica.health_stale", map[string]any{
				"intent_version": commitment.IntentVersion,
			})
	}
	return errors.Join(cause, markErr)
}

func validateHealthResult(previous placement.Commitment, requestedExpiry time.Time, result healthResultBody, now time.Time) error {
	if !sameHealthCommitment(previous, result.Commitment) || result.Commitment.LastObservedAt.Before(now) {
		return fmt.Errorf("replica health response binding is invalid")
	}
	switch result.Status {
	case "healthy":
		if result.Commitment.State != placement.CommitmentActive || !result.Commitment.LeaseExpiresAt.Equal(requestedExpiry) || result.Reason != "" {
			return fmt.Errorf("replica health renewal is invalid")
		}
	case "corrupt":
		if result.Commitment.State != placement.CommitmentCorrupt || result.Reason != "replica_integrity_failed" {
			return fmt.Errorf("replica corrupt response is invalid")
		}
	case "revoked":
		if result.Commitment.State != placement.CommitmentRevoked || result.Reason != "replica_authorization_revoked" {
			return fmt.Errorf("replica revoked response is invalid")
		}
	default:
		return fmt.Errorf("replica health response status is invalid")
	}
	return nil
}

func (s *Service) handleHealthQuery(ctx context.Context, wire controlWire) error {
	var query healthQueryBody
	if err := json.Unmarshal(wire.Body, &query); err != nil {
		return err
	}
	now := s.cfg.Now().UTC()
	if query.Commitment.PeerID != s.cfg.LocalNodeID || !query.RequestedLeaseExpiresAt.After(now) ||
		query.RequestedLeaseExpiresAt.After(now.Add(replicaLeaseDuration)) {
		return fmt.Errorf("replica health query binding is invalid")
	}
	current, ok := s.cfg.Data.ReplicaPlacementState().Commitments[query.Commitment.OperationID]
	if !ok || !sameHealthCommitment(current, query.Commitment) {
		return fmt.Errorf("replica health commitment is unknown")
	}
	blob, ok := s.cfg.Data.GetBlob(current.BlobID)
	if !ok || !validReplicaBlobMetadata(blob) {
		return s.publishUnhealthyReplica(ctx, wire, current, placement.CommitmentCorrupt, "replica_integrity_failed", now)
	}
	auth := s.authorizePeer(wire.Source, blob, query.RequestedLeaseExpiresAt)
	if placementAuthorizationDenial(auth) != "" {
		return s.publishUnhealthyReplica(ctx, wire, current, placement.CommitmentRevoked, "replica_authorization_revoked", now)
	}
	raw, err := s.cfg.Data.GetBlobPayload(current.BlobID)
	if err != nil {
		return s.publishUnhealthyReplica(ctx, wire, current, placement.CommitmentCorrupt, "replica_integrity_failed", now)
	}
	hash, cid, err := payload.DeriveIdentity(raw)
	if err != nil || hash != blob.Hash || cid != current.CID {
		return s.publishUnhealthyReplica(ctx, wire, current, placement.CommitmentCorrupt, "replica_integrity_failed", now)
	}
	renewed, err := s.cfg.Data.RenewReplicaCommitment(current.OperationID, now, query.RequestedLeaseExpiresAt)
	if err != nil {
		return err
	}
	return s.publishControl(ctx, actionHealthResult, wire.OperationID, wire.Source, healthResultBody{Commitment: renewed, Status: "healthy"})
}

func (s *Service) publishUnhealthyReplica(ctx context.Context, wire controlWire, current placement.Commitment, state, reason string, now time.Time) error {
	marked, err := s.cfg.Data.MarkReplicaCommitment(current.OperationID, state, now, reason)
	if err != nil {
		return err
	}
	return s.publishControl(ctx, actionHealthResult, wire.OperationID, wire.Source, healthResultBody{
		Commitment: marked, Status: healthStatus(state), Reason: reason,
	})
}

func healthStatus(state string) string {
	if state == placement.CommitmentRevoked {
		return "revoked"
	}
	return "corrupt"
}

func sameHealthCommitment(left, right placement.Commitment) bool {
	return left.OperationID == right.OperationID && left.IntentVersion == right.IntentVersion &&
		left.BlobID == right.BlobID && left.CID == right.CID && left.PeerID == right.PeerID &&
		left.Size == right.Size && left.LeaseStartsAt.Equal(right.LeaseStartsAt)
}
