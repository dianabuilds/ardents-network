package authority

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	identityapi "ardents/internal/identity"
	identitycapability "ardents/internal/identity/capability"
)

const maximumInitialGenerationValidity = 30 * 24 * time.Hour

func InitialGenerationDeliveryResource(realmID, requestID string) string {
	operationID, deliveryID := initialGenerationIDs(realmID, requestID)
	return generationDeliveryResource(realmID, operationID, deliveryID)
}

func GenerationDeliveryResource(realmID, operationID, deliveryID string) (string, bool) {
	resource := generationDeliveryResource(realmID, operationID, deliveryID)
	return resource, validGenerationDeliveryResource(resource)
}

func (s *Service) IssueInitialGeneration(
	ctx context.Context,
	command Command,
	request InitialGenerationRequest,
) (InitialGenerationResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := validateInitialGenerationRequest(request); err != nil {
		return InitialGenerationResult{}, err
	}
	if err := validateInitialGenerationCommand(command, request); err != nil {
		return InitialGenerationResult{}, err
	}
	if s.store == nil || s.signer == nil || s.repository == nil || s.policy == nil {
		return InitialGenerationResult{}, ErrUnavailable
	}
	if s.status.Readiness == ReadinessRecoveryRequired {
		return InitialGenerationResult{}, ErrRecoveryRequired
	}
	if err := s.policy.AdmitInitialGeneration(ctx, command); err != nil {
		return InitialGenerationResult{}, ErrPermissionDenied
	}
	now := s.clock().UTC().Truncate(time.Second)
	if err := identitycapability.VerifyDeliveryAttestation(request.RecipientAttestation, now); err != nil {
		return InitialGenerationResult{}, ErrInvalidArgument
	}
	if request.RecipientAttestation.SubjectPrincipal == "" {
		return InitialGenerationResult{}, ErrInvalidArgument
	}
	state, found, err := s.store.Load(ctx)
	if err != nil || !found {
		s.setUnavailable(ReasonStoreUnavailable)
		return InitialGenerationResult{}, ErrUnavailable
	}
	if err := validateLedger(state); err != nil {
		s.setRecovery(Status{}, ReasonPersistedStateInvalid)
		return InitialGenerationResult{}, ErrRecoveryRequired
	}
	if state.RealmID != request.RealmID {
		return InitialGenerationResult{}, ErrPermissionDenied
	}
	payloadHash := initialGenerationPayloadHash(request)
	for index := range state.InitialGenerationDeliveries {
		record := &state.InitialGenerationDeliveries[index]
		if record.RequestID != request.RequestID {
			continue
		}
		if record.PayloadHash != payloadHash {
			return InitialGenerationResult{}, ErrConflict
		}
		if state.Phase == PhaseCheckpointing {
			if err := s.reconcileLoaded(ctx, &state); err != nil {
				return InitialGenerationResult{}, err
			}
		}
		return initialGenerationResult(state, *record), nil
	}
	if len(state.Channels) != 0 || len(state.Members) != 0 ||
		len(state.InitialGenerationDeliveries) >= MaxOperations {
		return InitialGenerationResult{}, ErrConflict
	}
	principal, publicKey, err := s.signerBinding(ctx)
	if err != nil || principal != state.AuthorityPrincipal ||
		!ed25519.PublicKey(publicKey).Equal(ed25519.PublicKey(state.AuthorityPublicKey)) {
		s.setRecovery(statusFromLedger(state), ReasonSignerMismatch)
		return InitialGenerationResult{}, ErrRecoveryRequired
	}
	operationID, deliveryID := initialGenerationIDs(request.RealmID, request.RequestID)
	channelID, err := s.randomFixedID()
	if err != nil {
		return InitialGenerationResult{}, ErrUnavailable
	}
	grantID, err := s.randomFixedID()
	if err != nil {
		return InitialGenerationResult{}, ErrUnavailable
	}
	secretRaw, err := s.randomBytes(32)
	if err != nil {
		return InitialGenerationResult{}, ErrUnavailable
	}
	defer clear(secretRaw)
	secret, ok := identityapi.NewCapabilitySecret(secretRaw)
	if !ok {
		return InitialGenerationResult{}, ErrUnavailable
	}
	receiptKey, err := s.randomBytes(32)
	if err != nil {
		return InitialGenerationResult{}, ErrUnavailable
	}
	defer clear(receiptKey)
	notAfter := now.Add(request.ValidFor)
	grant := identityapi.CapabilityGrant{
		Version: ContractVersion, ChannelID: channelID, Generation: 1,
		Secret: secret, GrantID: grantID, IssuerPrincipal: state.AuthorityPrincipal,
		SubjectPrincipal: request.RecipientAttestation.SubjectPrincipal,
		Permissions:      request.Permissions, Scope: request.ChannelClass,
		NotBefore: now, NotAfter: notAfter,
	}
	grant, err = identitycapability.SignGrantWith(grant, func(message []byte) ([]byte, error) {
		return s.signer.Sign(ctx, message)
	})
	if err != nil {
		s.setUnavailable(ReasonSignerUnavailable)
		return InitialGenerationResult{}, ErrUnavailable
	}
	expiresAt := notAfter
	if request.RecipientAttestation.NotAfter.Before(expiresAt) {
		expiresAt = request.RecipientAttestation.NotAfter
	}
	nextSequence := state.AuthoritySequence + 1
	bundle := identitycapability.GenerationBundle{
		Version: ContractVersion, RealmID: state.RealmID,
		AuthorityPrincipal: state.AuthorityPrincipal, AuthorityEpoch: state.AuthorityEpoch,
		AuthoritySequence: nextSequence, OperationID: operationID, DeliveryID: deliveryID,
		ChannelID: channelID, ChannelClass: request.ChannelClass, Generation: 1,
		RecipientPrincipal: request.RecipientAttestation.SubjectPrincipal,
		DeliveryKeyDigest: identitycapability.DeliveryPublicKeyDigest(
			request.RecipientAttestation.DeliveryPublicKey,
		),
		SubjectGrant: grant, SenderGrants: []identityapi.CapabilityGrant{grant},
		Revocations:     []identityapi.CapabilityRevocation{},
		ActivationPhase: identitycapability.DeliveryPhaseInstalled,
		DrainDeadline:   now, ExpiresAt: expiresAt,
		ReceiptKey: append([]byte(nil), receiptKey...),
	}
	sealed, err := identitycapability.SealGenerationBundleForRecipient(
		bundle, request.RecipientAttestation, now,
		func(message []byte) ([]byte, error) { return s.signer.Sign(ctx, message) },
	)
	if err != nil {
		return InitialGenerationResult{}, ErrInvalidArgument
	}
	audit := newDeliveryAudit(
		deliveryAuditID(command.Action, deliveryID), command, operationID, state.AuditHead, now,
	)
	err = s.commitCheckpointTransition(ctx, &state, audit, now, func(next *Ledger, _ SignedCheckpoint) error {
		next.Members = append(next.Members, MemberRecord{
			Version: ContractVersion, Principal: request.RecipientAttestation.SubjectPrincipal,
		})
		next.Channels = append(next.Channels, ChannelRecord{
			Version: ContractVersion, ID: channelID, Class: string(request.ChannelClass),
			MemberCount: 1, CurrentGeneration: 1, OutstandingDeliveryCount: 1,
			Grant:         capabilityGrantRecord(grant),
			CurrentGrants: []CapabilityGrantRecord{capabilityGrantRecord(grant)},
		})
		next.InitialGenerationDeliveries = append(
			next.InitialGenerationDeliveries,
			InitialGenerationDeliveryRecord{
				Version: ContractVersion, RequestID: request.RequestID,
				PayloadHash: payloadHash, OperationID: operationID, DeliveryID: deliveryID,
				ChannelID: channelID, RecipientPrincipal: request.RecipientAttestation.SubjectPrincipal,
				Phase: DeliveryPhaseIssued, RetryGeneration: 0,
				ReceiptKey: append([]byte(nil), receiptKey...), Sealed: sealed,
				CreatedAt: now, Deadline: now.Add(MaxOperationLifetime),
			},
		)
		return nil
	})
	if err != nil {
		return InitialGenerationResult{}, err
	}
	return initialGenerationResult(state, state.InitialGenerationDeliveries[0]), nil
}

func (s *Service) AcknowledgeInitialGeneration(
	ctx context.Context,
	command Command,
	request InitialGenerationAcknowledgeRequest,
) (InitialGenerationAcknowledgeResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if request.Version != ContractVersion {
		return InitialGenerationAcknowledgeResult{}, ErrUnsupportedVersion
	}
	if !ValidRealmID(request.RealmID) ||
		command.Actor == "" || command.Actor != command.Effective ||
		command.Action != ActionAcknowledgeDelivery ||
		command.ResourceKind != ResourceKindGenerationDelivery ||
		!validGenerationDeliveryResource(command.ResourceID) {
		return InitialGenerationAcknowledgeResult{}, ErrPermissionDenied
	}
	if s.store == nil || s.signer == nil || s.repository == nil || s.policy == nil {
		return InitialGenerationAcknowledgeResult{}, ErrUnavailable
	}
	state, found, err := s.store.Load(ctx)
	if err != nil || !found {
		return InitialGenerationAcknowledgeResult{}, ErrUnavailable
	}
	if err := validateLedger(state); err != nil {
		s.setRecovery(Status{}, ReasonPersistedStateInvalid)
		return InitialGenerationAcknowledgeResult{}, ErrRecoveryRequired
	}
	if state.RealmID != request.RealmID {
		return InitialGenerationAcknowledgeResult{}, ErrPermissionDenied
	}
	if state.Phase == PhaseCheckpointing {
		if err := s.reconcileLoaded(ctx, &state); err != nil {
			return InitialGenerationAcknowledgeResult{}, err
		}
	}
	index := -1
	for candidate := range state.InitialGenerationDeliveries {
		if state.InitialGenerationDeliveries[candidate].DeliveryID == request.Receipt.DeliveryID {
			index = candidate
			break
		}
	}
	if index < 0 {
		return InitialGenerationAcknowledgeResult{}, ErrInvalidArgument
	}
	delivery := &state.InitialGenerationDeliveries[index]
	resource := generationDeliveryResource(
		state.RealmID, delivery.OperationID, delivery.DeliveryID,
	)
	if command.ResourceID != resource {
		return InitialGenerationAcknowledgeResult{}, ErrPermissionDenied
	}
	rotationDelivery := false
	for _, rotation := range state.Rotations {
		if containsDeliveryID(rotation.DeliveryIDs, delivery.DeliveryID) {
			rotationDelivery = true
			break
		}
	}
	if rotationDelivery {
		err = s.policy.AdmitChannelRotation(ctx, command)
	} else {
		err = s.policy.AdmitInitialGeneration(ctx, command)
	}
	if err != nil {
		return InitialGenerationAcknowledgeResult{}, ErrPermissionDenied
	}
	if delivery.Phase == DeliveryPhaseInstalled {
		if !receiptsEqual(delivery.Receipt, request.Receipt) {
			return InitialGenerationAcknowledgeResult{}, ErrConflict
		}
		return acknowledgeResult(state, *delivery), nil
	}
	now := s.clock().UTC().Truncate(time.Second)
	if identitycapability.VerifyGenerationDeliveryReceipt(request.Receipt, delivery.ReceiptKey) != nil ||
		request.Receipt.Phase != identitycapability.DeliveryPhaseInstalled ||
		request.Receipt.RealmID != delivery.Sealed.Binding.RealmID ||
		request.Receipt.AuthorityPrincipal != delivery.Sealed.Binding.AuthorityPrincipal ||
		request.Receipt.AuthorityEpoch != delivery.Sealed.Binding.AuthorityEpoch ||
		request.Receipt.OperationID != delivery.OperationID ||
		request.Receipt.DeliveryID != delivery.DeliveryID ||
		request.Receipt.EnvelopeDigest != delivery.Sealed.EnvelopeDigest ||
		request.Receipt.AuthoritySequence != delivery.Sealed.Binding.AuthoritySequence ||
		request.Receipt.ChannelID != delivery.ChannelID ||
		request.Receipt.ChannelClass != delivery.Sealed.Binding.ChannelClass ||
		request.Receipt.Generation != delivery.Sealed.Binding.Generation ||
		request.Receipt.DeliveryKeyDigest != delivery.Sealed.Binding.DeliveryKeyDigest ||
		request.Receipt.RecipientPrincipal != delivery.RecipientPrincipal ||
		request.Receipt.CreatedAt.Before(delivery.CreatedAt) ||
		request.Receipt.CreatedAt.After(now) ||
		!request.Receipt.CreatedAt.Before(delivery.Sealed.Binding.ExpiresAt) ||
		!request.Receipt.CreatedAt.Before(delivery.Deadline) ||
		!now.Before(delivery.Sealed.Binding.ExpiresAt) ||
		!now.Before(delivery.Deadline) {
		return InitialGenerationAcknowledgeResult{}, ErrInvalidArgument
	}
	audit := newDeliveryAudit(
		deliveryAuditID(command.Action, delivery.DeliveryID),
		command, delivery.OperationID, state.AuditHead, now,
	)
	err = s.commitCheckpointTransition(ctx, &state, audit, now, func(next *Ledger, _ SignedCheckpoint) error {
		next.InitialGenerationDeliveries[index].Phase = DeliveryPhaseInstalled
		next.InitialGenerationDeliveries[index].Receipt = request.Receipt
		for channelIndex := range next.Channels {
			if next.Channels[channelIndex].ID == delivery.ChannelID &&
				next.Channels[channelIndex].OutstandingDeliveryCount > 0 {
				next.Channels[channelIndex].OutstandingDeliveryCount--
				break
			}
		}
		for rotationIndex := range next.Rotations {
			rotation := &next.Rotations[rotationIndex]
			if !containsDeliveryID(rotation.DeliveryIDs, delivery.DeliveryID) {
				continue
			}
			if rotationDeliveriesAtPhase(*next, *rotation, DeliveryPhaseInstalled) {
				rotation.Phase = DeliveryPhaseInstalled
				setOperationPhase(next, rotation.OperationID, DeliveryPhaseInstalled)
			}
			break
		}
		return nil
	})
	if err != nil {
		return InitialGenerationAcknowledgeResult{}, err
	}
	return acknowledgeResult(state, state.InitialGenerationDeliveries[index]), nil
}

func (s *Service) commitCheckpointTransition(
	ctx context.Context,
	state *Ledger,
	audit AuditRecord,
	now time.Time,
	mutate func(*Ledger, SignedCheckpoint) error,
) error {
	if err := checkpointTransitionCapacity(*state); err != nil {
		return err
	}
	previousSequence, previousDigest := state.AuthoritySequence, state.Checkpoint.Digest
	checkpoint, err := SignCheckpoint(ctx, s.signer, Checkpoint{
		Version: ContractVersion, SchemaVersion: SchemaVersion,
		RealmID: state.RealmID, AuthorityPrincipal: state.AuthorityPrincipal,
		AuthorityPublicKey: append([]byte(nil), state.AuthorityPublicKey...),
		AuthorityEpoch:     state.AuthorityEpoch, AuthoritySequence: previousSequence + 1,
		PreviousDigest: previousDigest, AuditHead: audit.Hash, CreatedAt: now,
	})
	if err != nil {
		return err
	}
	if state.GenesisCheckpointDigest == "" {
		state.GenesisCheckpointDigest = previousDigest
	}
	expectedRevision := state.Revision
	state.Revision++
	state.AuthoritySequence = previousSequence + 1
	state.Phase, state.Readiness, state.Reason =
		PhaseCheckpointing, ReadinessDegraded, ReasonCheckpointMissing
	state.AuditHead, state.Checkpoint = audit.Hash, checkpoint
	if err := mutate(state, checkpoint); err != nil {
		return err
	}
	state.AuditLog = append(state.AuditLog, audit)
	state.AuditOutbox = append(state.AuditOutbox, audit)
	if err := s.store.Save(ctx, expectedRevision, *state); err != nil {
		s.setUnavailable(ReasonStoreUnavailable)
		return ErrUnavailable
	}
	s.status = statusFromLedger(*state)
	if err := s.crash(CrashAfterLedgerCommit); err != nil {
		return ErrUnavailable
	}
	if _, err := s.repository.CompareAndAppend(
		ctx, state.RealmID, previousSequence, checkpoint,
	); err != nil {
		if errors.Is(err, ErrConflict) || errors.Is(err, ErrCorruptState) {
			s.markRecovery(ctx, state, ReasonCheckpointMismatch)
			return ErrRecoveryRequired
		}
		s.status.Readiness, s.status.Reason = ReadinessUnavailable, ReasonRepositoryUnavailable
		return ErrUnavailable
	}
	if err := s.crash(CrashAfterCheckpointCreate); err != nil {
		return ErrUnavailable
	}
	expectedRevision = state.Revision
	state.Revision++
	state.Phase, state.Readiness, state.Reason = PhaseReady, ReadinessReady, ReasonNone
	if err := s.store.Save(ctx, expectedRevision, *state); err != nil {
		s.setUnavailable(ReasonStoreUnavailable)
		return ErrUnavailable
	}
	s.status = statusFromLedger(*state)
	s.flushAudit(ctx, state)
	return nil
}

func checkpointTransitionCapacity(state Ledger) error {
	if len(state.AuditLog) >= MaxAuditRecords ||
		len(state.AuditOutbox) >= MaxAuditOutboxRecords {
		return ErrResourceExhausted
	}
	return nil
}

func validateInitialGenerationRequest(request InitialGenerationRequest) error {
	if request.Version != ContractVersion {
		return ErrUnsupportedVersion
	}
	if len(request.RequestID) == 0 || len(request.RequestID) > MaxRequestIDBytes ||
		strings.TrimSpace(request.RequestID) != request.RequestID ||
		!ValidRealmID(request.RealmID) ||
		request.ValidFor <= 0 || request.ValidFor > maximumInitialGenerationValidity ||
		request.ValidFor%time.Second != 0 ||
		request.Permissions == 0 ||
		request.Permissions&^identityapi.CapabilityKnownPermissions != 0 {
		return ErrInvalidArgument
	}
	switch request.ChannelClass {
	case identityapi.CapabilityRealmDiscovery, identityapi.CapabilityDataExchange,
		identityapi.CapabilityApplication, identityapi.CapabilityControl:
	default:
		return ErrInvalidArgument
	}
	return nil
}

func validateInitialGenerationCommand(command Command, request InitialGenerationRequest) error {
	if command.Actor == "" || command.Actor != command.Effective ||
		command.Action != ActionIssueDelivery ||
		command.ResourceKind != ResourceKindGenerationDelivery ||
		command.ResourceID != InitialGenerationDeliveryResource(request.RealmID, request.RequestID) {
		return ErrPermissionDenied
	}
	return nil
}

func initialGenerationPayloadHash(request InitialGenerationRequest) string {
	raw, _ := json.Marshal(struct {
		Version      uint32
		RealmID      string
		ChannelClass identityapi.CapabilityScope
		Permissions  identityapi.CapabilityPermission
		Attestation  identityapi.CapabilityDeliveryAttestation
		ValidFor     int64
	}{
		request.Version, request.RealmID, request.ChannelClass, request.Permissions,
		request.RecipientAttestation, int64(request.ValidFor / time.Second),
	})
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func initialGenerationIDs(realmID, requestID string) (string, string) {
	operation := sha256.Sum256([]byte("ardents:initial-generation-operation:v1\x00" + realmID + "\x00" + requestID))
	delivery := sha256.Sum256([]byte("ardents:initial-generation-delivery:v1\x00" + realmID + "\x00" + requestID))
	return "rao1_" + hex.EncodeToString(operation[:16]),
		"rad1_" + hex.EncodeToString(delivery[:16])
}

func deliveryAuditID(action, deliveryID string) string {
	sum := sha256.Sum256([]byte("ardents:delivery-audit:v1\x00" + action + "\x00" + deliveryID))
	return "raa1_" + hex.EncodeToString(sum[:16])
}

func generationDeliveryResource(realmID, operationID, deliveryID string) string {
	return fmt.Sprintf("realm/%s/operation/%s/delivery/%s", realmID, operationID, deliveryID)
}

func validGenerationDeliveryResource(value string) bool {
	parts := strings.Split(value, "/")
	return len(parts) == 6 && parts[0] == "realm" && ValidRealmID(parts[1]) &&
		parts[2] == "operation" && operationIDPattern.MatchString(parts[3]) &&
		parts[4] == "delivery" && deliveryIDPattern.MatchString(parts[5])
}

func (s *Service) randomBytes(size int) ([]byte, error) {
	raw := make([]byte, size)
	if _, err := io.ReadFull(s.random, raw); err != nil {
		return nil, err
	}
	var combined byte
	for _, value := range raw {
		combined |= value
	}
	if combined == 0 {
		return nil, ErrUnavailable
	}
	return raw, nil
}

func (s *Service) randomFixedID() ([16]byte, error) {
	raw, err := s.randomBytes(16)
	if err != nil {
		return [16]byte{}, err
	}
	var id [16]byte
	copy(id[:], raw)
	clear(raw)
	return id, nil
}

func newDeliveryAudit(
	id string, command Command, operationID, previousHash string, createdAt time.Time,
) AuditRecord {
	record := AuditRecord{
		Version: ContractVersion, ID: id, Actor: command.Actor, Effective: command.Effective,
		Action: command.Action, ResourceKind: command.ResourceKind, ResourceID: command.ResourceID,
		OperationID: operationID, Outcome: "accepted", PreviousHash: previousHash,
		CreatedAt: createdAt,
	}
	record.Hash = auditHash(record)
	return record
}

func initialGenerationResult(state Ledger, record InitialGenerationDeliveryRecord) InitialGenerationResult {
	return InitialGenerationResult{
		Version: ContractVersion, RealmID: state.RealmID,
		OperationID: record.OperationID, DeliveryID: record.DeliveryID,
		AuthoritySequence: record.Sealed.Binding.AuthoritySequence,
		ChannelID:         record.ChannelID, Generation: record.Sealed.Binding.Generation,
		Sealed: record.Sealed,
	}
}

func acknowledgeResult(state Ledger, record InitialGenerationDeliveryRecord) InitialGenerationAcknowledgeResult {
	return InitialGenerationAcknowledgeResult{
		Version: ContractVersion, RealmID: state.RealmID, DeliveryID: record.DeliveryID,
		AuthoritySequence: state.AuthoritySequence, Phase: record.Phase,
	}
}

func receiptsEqual(
	left, right identitycapability.GenerationDeliveryReceipt,
) bool {
	leftRaw, leftErr := json.Marshal(left)
	rightRaw, rightErr := json.Marshal(right)
	return leftErr == nil && rightErr == nil && bytes.Equal(leftRaw, rightRaw)
}
