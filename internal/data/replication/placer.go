package replication

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	appdata "ardents/internal/data"
	"ardents/internal/data/placement"
	"ardents/internal/data/transfer"
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
		return placement.Commitment{}, err
	}
	commitment, err := s.commit(ctx, responses, target, result, offer, blob, ciphertext, now.Add(24*time.Hour))
	if err != nil {
		return placement.Commitment{}, err
	}
	return s.cfg.Data.ObserveReplicaCommitment(commitment, s.cfg.Now().UTC())
}

func (s *Service) loadPlacementBlob(blobID string) (appdata.Blob, []byte, error) {
	blob, ok := s.cfg.Data.GetBlob(blobID)
	if !ok {
		return appdata.Blob{}, nil, fmt.Errorf("blob not found")
	}
	if !blob.Encrypted {
		return appdata.Blob{}, nil, fmt.Errorf("plaintext replica placement is forbidden")
	}
	if !validReplicaBlobMetadata(blob) {
		return appdata.Blob{}, nil, fmt.Errorf("replica blob metadata is invalid")
	}
	ciphertext, err := s.cfg.Data.GetBlobPayload(blobID)
	if err != nil {
		return appdata.Blob{}, nil, err
	}
	if int64(len(ciphertext)) > placement.MaxInlineReplicaBytes {
		return appdata.Blob{}, nil, fmt.Errorf("replica transfer requires chunking")
	}
	return blob, ciphertext, nil
}

func (s *Service) reserve(ctx context.Context, responses <-chan transfer.ReplicaControlMessage, target string, offer placement.ReservationOffer, blob appdata.Blob) (placement.ReservationResult, error) {
	if err := s.publishControl(ctx, actionReserveOffer, offer.OperationID, target, reserveOfferBody{Offer: offer, Blob: blob}); err != nil {
		return placement.ReservationResult{}, err
	}
	wire, err := s.awaitControl(ctx, responses, actionReserveResult, target)
	if err != nil {
		return placement.ReservationResult{}, err
	}
	var body reserveResultBody
	if err := json.Unmarshal(wire.Body, &body); err != nil {
		return placement.ReservationResult{}, err
	}
	if body.Result.OperationID != offer.OperationID || !body.Result.ExpiresAt.Equal(offer.ExpiresAt) {
		return placement.ReservationResult{}, fmt.Errorf("replica reservation response binding is invalid")
	}
	if body.Result.Status != placement.ReservationAccepted || body.Result.Token == "" {
		return placement.ReservationResult{}, fmt.Errorf("replica reservation rejected: %s", body.Result.Reason)
	}
	return body.Result, nil
}

func (s *Service) commit(ctx context.Context, responses <-chan transfer.ReplicaControlMessage, target string, result placement.ReservationResult, offer placement.ReservationOffer, blob appdata.Blob, ciphertext []byte, leaseExpiresAt time.Time) (placement.Commitment, error) {
	request := placement.CommitRequest{OperationID: offer.OperationID, Token: result.Token, Blob: blob, LeaseExpiresAt: leaseExpiresAt}
	body := commitRequestBody{Request: request, Ciphertext: base64.RawStdEncoding.EncodeToString(ciphertext)}
	if err := s.publishControl(ctx, actionCommitRequest, offer.OperationID, target, body); err != nil {
		return placement.Commitment{}, err
	}
	wire, err := s.awaitControl(ctx, responses, actionCommitResult, target)
	if err != nil {
		return placement.Commitment{}, err
	}
	var response commitResultBody
	if err := json.Unmarshal(wire.Body, &response); err != nil {
		return placement.Commitment{}, err
	}
	if response.Status != "committed" {
		return placement.Commitment{}, fmt.Errorf("replica commit rejected: %s", response.Reason)
	}
	if response.Commitment.OperationID != offer.OperationID || response.Commitment.IntentVersion != offer.IntentVersion ||
		response.Commitment.BlobID != offer.BlobID || response.Commitment.CID != offer.CID ||
		response.Commitment.PeerID != target || response.Commitment.Size != offer.EncryptedSize ||
		response.Commitment.State != placement.CommitmentActive || response.Commitment.LeaseStartsAt.IsZero() ||
		response.Commitment.LeaseStartsAt.After(response.Commitment.LeaseExpiresAt) ||
		!response.Commitment.LeaseExpiresAt.Equal(leaseExpiresAt) {
		return placement.Commitment{}, fmt.Errorf("replica commitment response binding is invalid")
	}
	return response.Commitment, nil
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

func (s *Service) authorizeTarget(target string, blob appdata.Blob) error {
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
