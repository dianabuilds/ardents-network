package replication

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	"ardents/internal/content"
	model "ardents/internal/content/catalog"
	datapayload "ardents/internal/content/payload"
	identityprincipal "ardents/internal/identity/principal"
	privacy "ardents/internal/messaging"
	"ardents/internal/replication/placement"
	"ardents/internal/transfer"
)

func (s *Service) handle(ctx context.Context, message transfer.ReplicaControlMessage) error {
	target, err := controlTarget(message.Payload)
	if err != nil {
		return fmt.Errorf("replica control routing is invalid")
	}
	if !target.Equal(s.cfg.LocalNodePrincipal) {
		return nil
	}
	wire, err := verifyControl(message.Payload, s.cfg.LocalNodePrincipal)
	if err != nil || wire.Action != message.Action || wire.OperationID != message.OperationID {
		return fmt.Errorf("replica control authentication failed")
	}
	switch wire.Action {
	case actionReserveOffer:
		return s.handleReserve(ctx, wire)
	case actionCommitRequest:
		return s.handleCommit(ctx, wire)
	case actionCapacityQuery:
		return s.handleCapacityQuery(ctx, wire)
	case actionHealthQuery:
		return s.handleHealthQuery(ctx, wire)
	default:
		return fmt.Errorf("replica control action is invalid")
	}
}

func controlTarget(payload []byte) (identityprincipal.ID, error) {
	var route controlWire
	if err := decodeControlBody(payload, &route); err != nil || route.Target.String() == "" {
		return identityprincipal.ID{}, fmt.Errorf("replica control target is missing")
	}
	return route.Target, nil
}

func (s *Service) handleCapacityQuery(ctx context.Context, wire controlWire) error {
	var body capacityQueryBody
	if err := decodeControlBody(wire.Body, &body); err != nil {
		return err
	}
	if !validReplicaBlobMetadata(body.Blob) {
		return fmt.Errorf("replica capacity query content binding is invalid")
	}
	result := capacityResultBody{Status: "available"}
	auth := s.authorizePeer(wire.Source, body.Blob, s.cfg.Now().UTC().Add(24*time.Hour))
	if reason := placementAuthorizationDenial(auth); reason != "" {
		result.Status, result.Reason = "rejected", reason
	} else {
		capacity := s.cfg.Data.ReplicaCapacity()
		result.Capacity = &capacity
	}
	return s.publishControl(ctx, actionCapacityResult, wire.OperationID, wire.Source, result)
}

func validReplicaBlobMetadata(blob model.Blob) bool {
	return blob.Encrypted && blob.Reference.String() != "" && blob.Hash != "" &&
		blob.Cipher == datapayload.AES256GCMCipher && blob.Size > 0
}

func placementAuthorizationDenial(auth placement.PeerAuthorization) string {
	switch {
	case !auth.Authenticated || !auth.Trusted:
		return placement.ReasonUntrusted
	case !auth.PermissionValid:
		return placement.ReasonPermission
	case !auth.PolicyAllowed:
		return placement.ReasonPolicy
	default:
		return ""
	}
}

func (s *Service) handleReserve(ctx context.Context, wire controlWire) error {
	var body reserveOfferBody
	if err := decodeControlBody(wire.Body, &body); err != nil {
		return err
	}
	offer := body.offer(wire.OperationID)
	if offer.EncryptedSize != body.Blob.Size || !validReplicaBlobMetadata(body.Blob) {
		return fmt.Errorf("replica reservation content binding is invalid")
	}
	auth := s.authorizePeer(wire.Source, body.Blob, offer.ExpiresAt)
	result, err := s.cfg.Data.ReserveReplica(offer, auth)
	if err != nil {
		result = placement.ReservationResult{OperationID: wire.OperationID, Status: placement.ReservationRejected, Reason: safeReason(err)}
	}
	if publishErr := s.publishControl(ctx, actionReserveResult, wire.OperationID, wire.Source, reserveResultBody{Result: result}); publishErr != nil {
		return publishErr
	}
	return err
}

func (s *Service) handleCommit(ctx context.Context, wire controlWire) error {
	var body commitRequestBody
	if err := decodeControlBody(wire.Body, &body); err != nil {
		return err
	}
	if body.Request.OperationID != wire.OperationID {
		return fmt.Errorf("replica commit operation binding is invalid")
	}
	ciphertext, err := base64.RawStdEncoding.DecodeString(body.Ciphertext)
	if err != nil {
		return fmt.Errorf("replica ciphertext encoding is invalid")
	}
	body.Request.Ciphertext = ciphertext
	auth := s.authorizePeer(wire.Source, body.Request.Blob, body.Request.LeaseExpiresAt)
	commitment, commitErr := s.cfg.Data.CommitReplica(body.Request, auth)
	result := commitResultBody{Commitment: &commitment, Status: "committed"}
	if commitErr != nil {
		result.Commitment, result.Status, result.Reason = nil, "rejected", safeReason(commitErr)
	}
	if err := s.publishControl(ctx, actionCommitResult, wire.OperationID, wire.Source, result); err != nil {
		return err
	}
	return commitErr
}

func (s *Service) authorizePeer(principal identityprincipal.ID, blob model.Blob, expiresAt time.Time) placement.PeerAuthorization {
	auth := placement.PeerAuthorization{NodePrincipal: principal, Authenticated: principal.String() != "", PermissionValid: true}
	entry, outcome, ok := s.cfg.Discovery.Resolve(principal.String(), "node")
	if ok && outcome == "found" {
		trust := s.cfg.Trust.Evaluate(entry.Record)
		auth.Trusted = trust.Valid && trust.Trusted && trust.Usable
	}
	auth.PolicyAllowed = s.cfg.Policy.AllowBlobRetention(blobView(blob), true, expiresAt, s.cfg.Now().UTC()) == nil
	return auth
}

func blobView(blob model.Blob) content.BlobPolicyView {
	return content.BlobPolicyView{State: blob.State, Retention: blob.Retention, Encrypted: blob.Encrypted}
}

func (s *Service) publishControl(ctx context.Context, action, operationID string, target identityprincipal.ID, body any) error {
	identity := s.cfg.Identity()
	payload, err := signControl(action, operationID, s.cfg.LocalNodePrincipal, target, identity.PublicKey, body, s.cfg.PrivateKey())
	if err != nil {
		return err
	}
	return s.cfg.Exchange.Publish(ctx, privacy.MessageClassBlobReplicaControl, payload)
}
