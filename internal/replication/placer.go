package replication

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	model "ardents/internal/content/catalog"
	"ardents/internal/replication/placement"
	"ardents/internal/transfer"
)

const (
	replicaControlAttemptTimeout = 3 * time.Second
	replicaControlAttempts       = 3
	replicaControlRetryDelay     = 250 * time.Millisecond
)

func (s *Service) PlaceBlob(ctx context.Context, blobID, target string, intentVersion uint64) (placement.Commitment, error) {
	blob, ciphertext, err := s.loadPlacementBlob(blobID)
	if err != nil {
		return placement.Commitment{}, err
	}
	if err := s.authorizeTarget(target, blob); err != nil {
		return placement.Commitment{}, err
	}
	operationID, nonce, err := operationIdentity()
	if err != nil {
		return placement.Commitment{}, err
	}
	responses, unregister, err := s.cfg.Exchange.RegisterReplicaResponses(operationID)
	if err != nil {
		return placement.Commitment{}, err
	}
	defer unregister()
	now := s.cfg.Now().UTC()
	offer := placement.ReservationOffer{
		OperationID: operationID, ProtocolVersion: placement.ReplicaProtocolVersion,
		IntentVersion: intentVersion, BlobID: blob.ID,
		CID: blob.CID, EncryptedSize: int64(len(ciphertext)), RequestedLease: 24 * time.Hour,
		ExpiresAt: now.Add(2 * time.Minute), Nonce: nonce,
	}
	result, err := s.reserve(ctx, responses, target, offer, blob)
	if err != nil {
		return placement.Commitment{}, fmt.Errorf("reserve replica: %w", err)
	}
	commitment, err := s.commit(ctx, responses, target, result, offer, blob, ciphertext, now.Add(24*time.Hour))
	if err != nil {
		return placement.Commitment{}, fmt.Errorf("commit replica: %w", err)
	}
	return s.cfg.Data.ObserveReplicaCommitment(commitment, s.cfg.Now().UTC())
}

func (s *Service) loadPlacementBlob(blobID string) (model.Blob, []byte, error) {
	blob, ok := s.cfg.Data.GetBlob(blobID)
	if !ok {
		return model.Blob{}, nil, fmt.Errorf("blob not found")
	}
	if !blob.Encrypted {
		return model.Blob{}, nil, fmt.Errorf("plaintext replica placement is forbidden")
	}
	if !validReplicaBlobMetadata(blob) {
		return model.Blob{}, nil, fmt.Errorf("replica blob metadata is invalid")
	}
	ciphertext, err := s.cfg.Data.GetBlobPayload(blobID)
	if err != nil {
		return model.Blob{}, nil, err
	}
	if int64(len(ciphertext)) > placement.MaxInlineReplicaBytes {
		return model.Blob{}, nil, fmt.Errorf("replica transfer requires chunking")
	}
	return blob, ciphertext, nil
}

func (s *Service) reserve(ctx context.Context, responses <-chan transfer.ReplicaControlMessage, target string, offer placement.ReservationOffer, blob model.Blob) (placement.ReservationResult, error) {
	var result placement.ReservationResult
	err := retryReplicaControl(ctx, func(attemptCtx context.Context) error {
		if err := s.publishControl(attemptCtx, actionReserveOffer, offer.OperationID, target, reserveOfferBody{Offer: offer, Blob: blob}); err != nil {
			return err
		}
		wire, err := s.awaitControl(attemptCtx, responses, actionReserveResult, target)
		if err != nil {
			return err
		}
		var body reserveResultBody
		if err := json.Unmarshal(wire.Body, &body); err != nil {
			return err
		}
		if body.Result.OperationID != offer.OperationID || !body.Result.ExpiresAt.Equal(offer.ExpiresAt) {
			return fmt.Errorf("replica reservation response binding is invalid")
		}
		if body.Result.Status != placement.ReservationAccepted || body.Result.Token == "" {
			return fmt.Errorf("replica reservation rejected: %s", body.Result.Reason)
		}
		result = body.Result
		return nil
	})
	return result, err
}

func (s *Service) commit(ctx context.Context, responses <-chan transfer.ReplicaControlMessage, target string, result placement.ReservationResult, offer placement.ReservationOffer, blob model.Blob, ciphertext []byte, leaseExpiresAt time.Time) (placement.Commitment, error) {
	request := placement.CommitRequest{OperationID: offer.OperationID, Token: result.Token, Blob: blob, LeaseExpiresAt: leaseExpiresAt}
	body := commitRequestBody{Request: request, Ciphertext: base64.RawStdEncoding.EncodeToString(ciphertext)}
	var commitment placement.Commitment
	err := retryReplicaControl(ctx, func(attemptCtx context.Context) error {
		if err := s.publishControl(attemptCtx, actionCommitRequest, offer.OperationID, target, body); err != nil {
			return err
		}
		wire, err := s.awaitControl(attemptCtx, responses, actionCommitResult, target)
		if err != nil {
			return err
		}
		var response commitResultBody
		if err := json.Unmarshal(wire.Body, &response); err != nil {
			return err
		}
		if response.Status != "committed" {
			return fmt.Errorf("replica commit rejected: %s", response.Reason)
		}
		if response.Commitment.OperationID != offer.OperationID || response.Commitment.IntentVersion != offer.IntentVersion ||
			response.Commitment.BlobID != offer.BlobID || response.Commitment.CID != offer.CID ||
			response.Commitment.PeerID != target || response.Commitment.Size != offer.EncryptedSize ||
			response.Commitment.State != placement.CommitmentActive || response.Commitment.LeaseStartsAt.IsZero() ||
			response.Commitment.LeaseStartsAt.After(response.Commitment.LeaseExpiresAt) ||
			!response.Commitment.LeaseExpiresAt.Equal(leaseExpiresAt) {
			return fmt.Errorf("replica commitment response binding is invalid")
		}
		commitment = response.Commitment
		return nil
	})
	return commitment, err
}

func retryReplicaControl(ctx context.Context, roundTrip func(context.Context) error) error {
	var lastErr error
	for attempt := range replicaControlAttempts {
		attemptCtx, cancel := context.WithTimeout(ctx, replicaControlAttemptTimeout)
		lastErr = roundTrip(attemptCtx)
		cancel()
		if lastErr == nil {
			return nil
		}
		if ctx.Err() != nil {
			return errors.Join(lastErr, ctx.Err())
		}
		if !errors.Is(lastErr, context.DeadlineExceeded) || attempt+1 == replicaControlAttempts {
			return lastErr
		}
		timer := time.NewTimer(replicaControlRetryDelay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return errors.Join(lastErr, ctx.Err())
		case <-timer.C:
		}
	}
	return lastErr
}

func (s *Service) awaitControl(ctx context.Context, responses <-chan transfer.ReplicaControlMessage, action, source string) (controlWire, error) {
	for {
		select {
		case <-ctx.Done():
			return controlWire{}, ctx.Err()
		case message := <-responses:
			if message.Action != action {
				continue
			}
			wire, err := verifyControl(message.Payload, s.cfg.LocalNodeID)
			if err == nil && wire.Source == source && wire.OperationID == message.OperationID {
				return wire, nil
			}
		}
	}
}

func (s *Service) authorizeTarget(target string, blob model.Blob) error {
	entry, outcome, ok := s.cfg.Discovery.Resolve(target, "node")
	if !ok || outcome != "found" || !s.cfg.Trust.Evaluate(entry.Record).Usable {
		return fmt.Errorf("replica target is not trusted and usable")
	}
	return s.cfg.Policy.AllowPeerBlobReserving(blobView(blob))
}

func operationIdentity() (string, string, error) {
	var operation [16]byte
	var nonce [16]byte
	if _, err := rand.Read(operation[:]); err != nil {
		return "", "", err
	}
	if _, err := rand.Read(nonce[:]); err != nil {
		return "", "", err
	}
	return hex.EncodeToString(operation[:]), hex.EncodeToString(nonce[:]), nil
}
