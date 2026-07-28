package authority

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	identityapi "ardents/internal/identity"
	identitycapability "ardents/internal/identity/capability"
)

const MaximumPreviousGenerationDrain = 30 * 24 * time.Hour

func ChannelResource(realmID string, channelID [16]byte) string {
	return fmt.Sprintf("realm/%s/channel/%s", realmID, hex.EncodeToString(channelID[:]))
}

func OperationResource(realmID, operationID string) string {
	return fmt.Sprintf("realm/%s/operation/%s", realmID, operationID)
}

func validChannelResource(value string) bool {
	parts := strings.Split(value, "/")
	if len(parts) != 4 || parts[0] != "realm" || !ValidRealmID(parts[1]) ||
		parts[2] != "channel" || len(parts[3]) != 32 {
		return false
	}
	_, err := hex.DecodeString(parts[3])
	return err == nil
}

func ValidChannelResource(value string) bool { return validChannelResource(value) }

func validOperationResource(value string) bool {
	parts := strings.Split(value, "/")
	return len(parts) == 4 && parts[0] == "realm" && ValidRealmID(parts[1]) &&
		parts[2] == "operation" && operationIDPattern.MatchString(parts[3])
}

func ValidOperationResource(value string) bool { return validOperationResource(value) }

type RotationRequest struct {
	Version               uint32
	RequestID             string
	RealmID               string
	ChannelID             [16]byte
	RecipientAttestations []identityapi.CapabilityDeliveryAttestation
	ValidFor              time.Duration
	DrainFor              time.Duration
	MembershipChange      MembershipChangeKind
	TargetPrincipal       string
}

type RotationDelivery struct {
	DeliveryID         string
	RecipientPrincipal string
	Sealed             identitycapability.SealedGenerationDelivery
}

type RotationResult struct {
	Version            uint32
	RealmID            string
	OperationID        string
	AuthoritySequence  uint64
	ChannelID          [16]byte
	PreviousGeneration uint32
	PendingGeneration  uint32
	Phase              string
	Deliveries         []RotationDelivery
	MembershipChange   MembershipChangeRecord
}

type ActivationCommitRequest struct {
	Version     uint32
	RealmID     string
	OperationID string
}

type ActivationCommitResult struct {
	Version           uint32
	RealmID           string
	OperationID       string
	AuthoritySequence uint64
	Phase             string
	Activation        identitycapability.GenerationActivation
}

type ActivationAcknowledgeRequest struct {
	Version      uint32
	RealmID      string
	OperationID  string
	ApprovedHost bool
	Receipt      identitycapability.GenerationDeliveryReceipt
}

type ActivationAcknowledgeResult struct {
	Version            uint32
	RealmID            string
	OperationID        string
	AuthoritySequence  uint64
	Phase              string
	CurrentGeneration  uint32
	PreviousGeneration uint32
	DrainDeadline      time.Time
}

func (s *Service) RotateChannel(
	ctx context.Context,
	command Command,
	request RotationRequest,
) (RotationResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.rotateChannelLocked(ctx, command, request)
}

func (s *Service) rotateChannelLocked(
	ctx context.Context,
	command Command,
	request RotationRequest,
) (RotationResult, error) {
	if err := validateRotationRequest(request); err != nil {
		return RotationResult{}, err
	}
	expectedAction := ActionRotateGeneration
	if request.MembershipChange != "" {
		expectedAction = ActionChangeMembership
	}
	if command.Actor == "" || command.Actor != command.Effective ||
		command.Action != expectedAction ||
		command.ResourceKind != ResourceKindChannel ||
		command.ResourceID != ChannelResource(request.RealmID, request.ChannelID) {
		return RotationResult{}, ErrPermissionDenied
	}
	if s.store == nil || s.signer == nil || s.repository == nil || s.policy == nil {
		return RotationResult{}, ErrUnavailable
	}
	var policyErr error
	if request.MembershipChange == "" {
		policyErr = s.policy.AdmitChannelRotation(ctx, command)
	} else {
		policyErr = s.policy.AdmitChannelMembership(ctx, command)
	}
	if policyErr != nil {
		return RotationResult{}, ErrPermissionDenied
	}
	state, found, err := s.store.Load(ctx)
	if err != nil || !found {
		return RotationResult{}, ErrUnavailable
	}
	if err := validateLedger(state); err != nil {
		s.setRecovery(Status{}, ReasonPersistedStateInvalid)
		return RotationResult{}, ErrRecoveryRequired
	}
	if state.RealmID != request.RealmID {
		return RotationResult{}, ErrPermissionDenied
	}
	if state.Phase == PhaseCheckpointing {
		if err := s.reconcileLoaded(ctx, &state); err != nil {
			return RotationResult{}, err
		}
	}
	payloadHash := rotationPayloadHash(request)
	for _, rotation := range state.Rotations {
		if rotation.RequestID != request.RequestID {
			continue
		}
		if rotation.PayloadHash != payloadHash {
			return RotationResult{}, ErrConflict
		}
		return rotationResult(state, rotation), nil
	}
	channelIndex := channelRecordIndex(state, request.ChannelID)
	if channelIndex < 0 {
		return RotationResult{}, ErrInvalidArgument
	}
	channel := state.Channels[channelIndex]
	if channel.PendingGenerationCount != 0 || incompleteRotationForChannel(state, request.ChannelID) {
		return RotationResult{}, ErrConflict
	}
	var capacityErr error
	if request.MembershipChange == "" {
		capacityErr = rotationCapacity(state, len(request.RecipientAttestations))
	} else {
		capacityErr = membershipCapacity(state, len(request.RecipientAttestations))
	}
	if capacityErr != nil {
		return RotationResult{}, capacityErr
	}
	currentGrants := channelCurrentGrants(channel)
	if len(currentGrants) == 0 || len(currentGrants) != int(channel.MemberCount) {
		return RotationResult{}, ErrConflict
	}
	if request.MembershipChange == MembershipChangeAdd &&
		len(state.Members) >= MaxRealmMembers {
		return RotationResult{}, ErrResourceExhausted
	}
	attestations := append([]identityapi.CapabilityDeliveryAttestation(nil), request.RecipientAttestations...)
	sort.Slice(attestations, func(left, right int) bool {
		return attestations[left].SubjectPrincipal < attestations[right].SubjectPrincipal
	})
	currentByPrincipal := make(map[string]identityapi.CapabilityGrant, len(currentGrants))
	for _, stored := range currentGrants {
		grant, ok := stored.restore()
		if !ok {
			return RotationResult{}, ErrRecoveryRequired
		}
		currentByPrincipal[grant.SubjectPrincipal] = grant
	}
	now := s.clock().UTC().Truncate(time.Second)
	seenAttestations := make(map[string]struct{}, len(attestations))
	for _, attestation := range attestations {
		if identitycapability.VerifyDeliveryAttestation(attestation, now) != nil {
			return RotationResult{}, ErrInvalidArgument
		}
		if _, duplicate := seenAttestations[attestation.SubjectPrincipal]; duplicate {
			return RotationResult{}, ErrInvalidArgument
		}
		seenAttestations[attestation.SubjectPrincipal] = struct{}{}
		_, currentMember := currentByPrincipal[attestation.SubjectPrincipal]
		if request.MembershipChange == "" && !currentMember {
			return RotationResult{}, ErrPermissionDenied
		}
	}
	if !validMembershipRecipients(
		request.MembershipChange, request.TargetPrincipal,
		currentByPrincipal, seenAttestations,
	) {
		return RotationResult{}, ErrConflict
	}
	principal, publicKey, err := s.signerBinding(ctx)
	if err != nil || principal != state.AuthorityPrincipal ||
		!ed25519.PublicKey(publicKey).Equal(ed25519.PublicKey(state.AuthorityPublicKey)) {
		return RotationResult{}, ErrRecoveryRequired
	}
	operationID := rotationOperationID(state.RealmID, request.RequestID)
	secretRaw, err := s.randomBytes(32)
	if err != nil {
		return RotationResult{}, ErrUnavailable
	}
	defer clear(secretRaw)
	secret, ok := identityapi.NewCapabilitySecret(secretRaw)
	if !ok {
		return RotationResult{}, ErrUnavailable
	}
	pendingGeneration := channel.CurrentGeneration + 1
	drainDeadline := now.Add(request.DrainFor)
	for _, current := range currentGrants {
		if current.NotAfter.Before(drainDeadline) {
			drainDeadline = current.NotAfter
		}
	}
	if !drainDeadline.After(now) {
		return RotationResult{}, ErrConflict
	}
	nextSequence := state.AuthoritySequence + 1
	pendingGrants := make([]CapabilityGrantRecord, 0, len(attestations))
	pendingByPrincipal := make(map[string]identityapi.CapabilityGrant, len(attestations))
	template := currentGrants[0]
	for _, attestation := range attestations {
		current, currentMember := currentByPrincipal[attestation.SubjectPrincipal]
		if !currentMember {
			restored, restoreOK := template.restore()
			if !restoreOK {
				return RotationResult{}, ErrRecoveryRequired
			}
			current = restored
			current.SubjectPrincipal = attestation.SubjectPrincipal
		}
		grantID, err := s.randomFixedID()
		if err != nil {
			return RotationResult{}, ErrUnavailable
		}
		grant := identityapi.CapabilityGrant{
			Version: ContractVersion, ChannelID: channel.ID, Generation: pendingGeneration,
			Secret: secret, GrantID: grantID, IssuerPrincipal: state.AuthorityPrincipal,
			SubjectPrincipal: current.SubjectPrincipal, Permissions: current.Permissions,
			Scope: current.Scope, NotBefore: now, NotAfter: now.Add(request.ValidFor),
		}
		grant, err = identitycapability.SignGrantWith(grant, func(message []byte) ([]byte, error) {
			return s.signer.Sign(ctx, message)
		})
		if err != nil {
			return RotationResult{}, ErrUnavailable
		}
		pendingGrants = append(pendingGrants, capabilityGrantRecord(grant))
		pendingByPrincipal[grant.SubjectPrincipal] = grant
	}
	senderSnapshot := make([]identityapi.CapabilityGrant, 0, len(attestations))
	for _, attestation := range attestations {
		senderSnapshot = append(senderSnapshot, pendingByPrincipal[attestation.SubjectPrincipal])
	}
	revocations := []identityapi.CapabilityRevocation{}
	if request.MembershipChange == MembershipChangeRemove {
		removed := currentByPrincipal[request.TargetPrincipal]
		revocation, signErr := identitycapability.SignRevocationWith(
			identityapi.CapabilityRevocation{
				Version: ContractVersion, GrantID: removed.GrantID,
				IssuerPrincipal: state.AuthorityPrincipal, RevokedAt: now,
			},
			func(message []byte) ([]byte, error) { return s.signer.Sign(ctx, message) },
		)
		if signErr != nil {
			return RotationResult{}, ErrUnavailable
		}
		revocations = append(revocations, revocation)
	}
	deliveries := make([]InitialGenerationDeliveryRecord, 0, len(attestations))
	for _, attestation := range attestations {
		grant := pendingByPrincipal[attestation.SubjectPrincipal]
		receiptKey, err := s.randomBytes(32)
		if err != nil {
			return RotationResult{}, ErrUnavailable
		}
		deliveryID := rotationDeliveryID(operationID, attestation.SubjectPrincipal)
		expiresAt := grant.NotAfter
		if attestation.NotAfter.Before(expiresAt) {
			expiresAt = attestation.NotAfter
		}
		bundle := identitycapability.GenerationBundle{
			Version: ContractVersion, RealmID: state.RealmID,
			AuthorityPrincipal: state.AuthorityPrincipal, AuthorityEpoch: state.AuthorityEpoch,
			AuthoritySequence: nextSequence, OperationID: operationID, DeliveryID: deliveryID,
			ChannelID: channel.ID, ChannelClass: identityapi.CapabilityScope(channel.Class),
			Generation: pendingGeneration, RecipientPrincipal: attestation.SubjectPrincipal,
			DeliveryKeyDigest: identitycapability.DeliveryPublicKeyDigest(attestation.DeliveryPublicKey),
			SubjectGrant:      grant, SenderGrants: append([]identityapi.CapabilityGrant(nil), senderSnapshot...),
			Revocations:     append([]identityapi.CapabilityRevocation(nil), revocations...),
			ActivationPhase: identitycapability.DeliveryPhaseInstalled,
			Candidate: request.MembershipChange == MembershipChangeAdd &&
				attestation.SubjectPrincipal == request.TargetPrincipal,
			DrainDeadline: drainDeadline, ExpiresAt: expiresAt,
			ReceiptKey: append([]byte(nil), receiptKey...),
		}
		sealed, err := identitycapability.SealGenerationBundleForRecipient(
			bundle, attestation, now,
			func(message []byte) ([]byte, error) { return s.signer.Sign(ctx, message) },
		)
		if err != nil {
			clear(receiptKey)
			return RotationResult{}, ErrInvalidArgument
		}
		deliveries = append(deliveries, InitialGenerationDeliveryRecord{
			Version: ContractVersion, RequestID: request.RequestID,
			PayloadHash: payloadHash, OperationID: operationID, DeliveryID: deliveryID,
			ChannelID: channel.ID, RecipientPrincipal: attestation.SubjectPrincipal,
			Phase: DeliveryPhaseIssued, ReceiptKey: append([]byte(nil), receiptKey...),
			Sealed: sealed, CreatedAt: now, Deadline: now.Add(MaxOperationLifetime),
		})
		clear(receiptKey)
	}
	audit := newDeliveryAudit(
		rotationAuditID(command.Action, operationID), command, operationID, state.AuditHead, now,
	)
	if request.MembershipChange != "" {
		audit.TargetPrincipal = request.TargetPrincipal
		audit.Hash = auditHash(audit)
	}
	err = s.commitCheckpointTransition(ctx, &state, audit, now, func(next *Ledger, checkpoint SignedCheckpoint) error {
		channel := &next.Channels[channelIndex]
		channel.PendingGenerationCount = 1
		channel.PendingGrants = cloneGrantRecords(pendingGrants)
		channel.OutstandingDeliveryCount = uint32(len(deliveries))
		deliveryIDs := make([]string, len(deliveries))
		for index := range deliveries {
			deliveryIDs[index] = deliveries[index].DeliveryID
		}
		next.InitialGenerationDeliveries = append(next.InitialGenerationDeliveries, deliveries...)
		membership := membershipChangeRecord(state, request)
		next.Rotations = append(next.Rotations, RotationRecord{
			Version: ContractVersion, RequestID: request.RequestID, PayloadHash: payloadHash,
			OperationID: operationID, ChannelID: request.ChannelID,
			PreviousGeneration: channel.CurrentGeneration, PendingGeneration: pendingGeneration,
			PrepareSequence: checkpoint.AuthoritySequence,
			Phase:           DeliveryPhaseDelivering, DeliveryIDs: deliveryIDs,
			CreatedAt: now, Deadline: now.Add(MaxOperationLifetime), DrainDeadline: drainDeadline,
			MembershipChange: membership,
		})
		next.Operations = append(next.Operations, OperationRecord{
			Version: ContractVersion, ID: operationID, RequestID: request.RequestID,
			Kind: operationKind(request), Phase: DeliveryPhaseDelivering,
			CreatedAt: now, Deadline: now.Add(MaxOperationLifetime),
		})
		return nil
	})
	if err != nil {
		return RotationResult{}, err
	}
	return rotationResult(state, state.Rotations[len(state.Rotations)-1]), nil
}

func rotationCapacity(state Ledger, memberCount int) error {
	requiredAuditRecords := 2*memberCount + 2
	if memberCount <= 0 || memberCount > MaxMembersPerChannel ||
		len(state.Operations) >= MaxOperations || len(state.Rotations) >= MaxOperations ||
		len(state.AuditLog)+requiredAuditRecords > MaxAuditRecords ||
		len(state.AuditOutbox)+requiredAuditRecords > MaxAuditOutboxRecords {
		return ErrResourceExhausted
	}
	return nil
}

func (s *Service) CommitChannelActivation(
	ctx context.Context,
	command Command,
	request ActivationCommitRequest,
) (ActivationCommitResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if request.Version != ContractVersion {
		return ActivationCommitResult{}, ErrUnsupportedVersion
	}
	if !ValidRealmID(request.RealmID) || !operationIDPattern.MatchString(request.OperationID) ||
		command.Actor == "" || command.Actor != command.Effective ||
		command.Action != ActionCommitActivation ||
		command.ResourceKind != ResourceKindOperation ||
		command.ResourceID != OperationResource(request.RealmID, request.OperationID) {
		return ActivationCommitResult{}, ErrPermissionDenied
	}
	if s.store == nil || s.signer == nil || s.repository == nil || s.policy == nil {
		return ActivationCommitResult{}, ErrUnavailable
	}
	state, found, err := s.store.Load(ctx)
	if err != nil || !found {
		return ActivationCommitResult{}, ErrUnavailable
	}
	if err := validateLedger(state); err != nil {
		return ActivationCommitResult{}, ErrRecoveryRequired
	}
	if state.RealmID != request.RealmID {
		return ActivationCommitResult{}, ErrPermissionDenied
	}
	if state.Phase == PhaseCheckpointing {
		if err := s.reconcileLoaded(ctx, &state); err != nil {
			return ActivationCommitResult{}, err
		}
	}
	rotationIndex := rotationRecordIndex(state, request.OperationID)
	if rotationIndex < 0 {
		return ActivationCommitResult{}, ErrInvalidArgument
	}
	rotation := state.Rotations[rotationIndex]
	if rotation.MembershipChange.Version != 0 {
		err = s.policy.AdmitChannelMembership(ctx, command)
	} else {
		err = s.policy.AdmitChannelRotation(ctx, command)
	}
	if err != nil {
		return ActivationCommitResult{}, ErrPermissionDenied
	}
	if rotation.Phase == DeliveryPhaseActivationCommitted || rotation.Phase == DeliveryPhaseCompleted {
		return activationCommitResult(state, rotation), nil
	}
	if rotation.Phase != DeliveryPhaseInstalled ||
		!rotationDeliveriesAtPhase(state, rotation, DeliveryPhaseInstalled) {
		return ActivationCommitResult{}, ErrConflict
	}
	now := s.clock().UTC().Truncate(time.Second)
	if !now.Before(rotation.Deadline) || !now.Before(rotation.DrainDeadline) {
		return ActivationCommitResult{}, ErrInvalidArgument
	}
	channelIndex := channelRecordIndex(state, rotation.ChannelID)
	if channelIndex < 0 {
		return ActivationCommitResult{}, ErrRecoveryRequired
	}
	audit := newDeliveryAudit(
		rotationAuditID(command.Action, rotation.OperationID),
		command, rotation.OperationID, state.AuditHead, now,
	)
	err = s.commitCheckpointTransition(ctx, &state, audit, now, func(next *Ledger, checkpoint SignedCheckpoint) error {
		channel := &next.Channels[channelIndex]
		activation, err := identitycapability.SignGenerationActivationWith(
			identitycapability.GenerationActivation{
				Version: ContractVersion, RealmID: next.RealmID,
				AuthorityPrincipal: next.AuthorityPrincipal, AuthorityEpoch: next.AuthorityEpoch,
				AuthoritySequence: checkpoint.AuthoritySequence,
				OperationID:       rotation.OperationID, ChannelID: rotation.ChannelID,
				ChannelClass:       identityapi.CapabilityScope(channel.Class),
				PreviousGeneration: rotation.PreviousGeneration,
				Generation:         rotation.PendingGeneration, EffectiveAt: now,
				DrainDeadline: rotation.DrainDeadline, CheckpointDigest: checkpoint.Digest,
			},
			func(message []byte) ([]byte, error) { return s.signer.Sign(ctx, message) },
		)
		if err != nil {
			return ErrUnavailable
		}
		channel.PreviousGrants = cloneGrantRecords(channelCurrentGrants(*channel))
		channel.PreviousReceiveGenerationCount = 1
		channel.PreviousDrainDeadline = rotation.DrainDeadline
		channel.CurrentGrants = cloneGrantRecords(channel.PendingGrants)
		channel.Grant = channel.CurrentGrants[0]
		channel.MemberCount = uint32(len(channel.CurrentGrants))
		channel.CurrentGeneration = rotation.PendingGeneration
		channel.PendingGrants = nil
		channel.PendingGenerationCount = 0
		next.Rotations[rotationIndex].Phase = DeliveryPhaseActivationCommitted
		next.Rotations[rotationIndex].Activation = activation
		applyMembershipTruth(next, next.Rotations[rotationIndex].MembershipChange)
		setOperationPhase(next, rotation.OperationID, DeliveryPhaseActivationCommitted)
		return nil
	})
	if err != nil {
		return ActivationCommitResult{}, err
	}
	return activationCommitResult(state, state.Rotations[rotationIndex]), nil
}

func (s *Service) AcknowledgeChannelActivation(
	ctx context.Context,
	command Command,
	request ActivationAcknowledgeRequest,
) (ActivationAcknowledgeResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if request.Version != ContractVersion {
		return ActivationAcknowledgeResult{}, ErrUnsupportedVersion
	}
	if !request.ApprovedHost {
		return ActivationAcknowledgeResult{}, ErrPermissionDenied
	}
	if !ValidRealmID(request.RealmID) || !operationIDPattern.MatchString(request.OperationID) ||
		command.Actor == "" || command.Actor != command.Effective ||
		command.Action != ActionAcknowledgeActivation ||
		command.ResourceKind != ResourceKindGenerationDelivery ||
		!validGenerationDeliveryResource(command.ResourceID) {
		return ActivationAcknowledgeResult{}, ErrPermissionDenied
	}
	if s.store == nil || s.signer == nil || s.repository == nil || s.policy == nil {
		return ActivationAcknowledgeResult{}, ErrUnavailable
	}
	state, found, err := s.store.Load(ctx)
	if err != nil || !found {
		return ActivationAcknowledgeResult{}, ErrUnavailable
	}
	if err := validateLedger(state); err != nil {
		return ActivationAcknowledgeResult{}, ErrRecoveryRequired
	}
	if state.RealmID != request.RealmID {
		return ActivationAcknowledgeResult{}, ErrPermissionDenied
	}
	if state.Phase == PhaseCheckpointing {
		if err := s.reconcileLoaded(ctx, &state); err != nil {
			return ActivationAcknowledgeResult{}, err
		}
	}
	rotationIndex := rotationRecordIndex(state, request.OperationID)
	if rotationIndex < 0 {
		return ActivationAcknowledgeResult{}, ErrInvalidArgument
	}
	rotation := state.Rotations[rotationIndex]
	if rotation.MembershipChange.Version != 0 {
		err = s.policy.AdmitChannelMembership(ctx, command)
	} else {
		err = s.policy.AdmitChannelRotation(ctx, command)
	}
	if err != nil {
		return ActivationAcknowledgeResult{}, ErrPermissionDenied
	}
	deliveryIndex := deliveryRecordIndex(state, request.Receipt.DeliveryID)
	if deliveryIndex < 0 || !containsDeliveryID(rotation.DeliveryIDs, request.Receipt.DeliveryID) {
		return ActivationAcknowledgeResult{}, ErrInvalidArgument
	}
	delivery := state.InitialGenerationDeliveries[deliveryIndex]
	resource := generationDeliveryResource(state.RealmID, rotation.OperationID, delivery.DeliveryID)
	if command.ResourceID != resource {
		return ActivationAcknowledgeResult{}, ErrPermissionDenied
	}
	if rotation.Phase == DeliveryPhaseCompleted {
		if !receiptsEqual(delivery.ActiveReceipt, request.Receipt) {
			return ActivationAcknowledgeResult{}, ErrConflict
		}
		return activationAcknowledgeResult(state, rotation), nil
	}
	if rotation.Phase != DeliveryPhaseActivationCommitted {
		return ActivationAcknowledgeResult{}, ErrConflict
	}
	now := s.clock().UTC().Truncate(time.Second)
	if identitycapability.VerifyGenerationDeliveryReceipt(request.Receipt, delivery.ReceiptKey) != nil ||
		request.Receipt.Phase != identitycapability.DeliveryPhaseActive ||
		request.Receipt.RealmID != state.RealmID ||
		request.Receipt.AuthorityPrincipal != state.AuthorityPrincipal ||
		request.Receipt.AuthorityEpoch != state.AuthorityEpoch ||
		request.Receipt.AuthoritySequence != rotation.Activation.AuthoritySequence ||
		request.Receipt.OperationID != rotation.OperationID ||
		request.Receipt.DeliveryID != delivery.DeliveryID ||
		request.Receipt.EnvelopeDigest != delivery.Sealed.EnvelopeDigest ||
		request.Receipt.ChannelID != rotation.ChannelID ||
		request.Receipt.ChannelClass != delivery.Sealed.Binding.ChannelClass ||
		request.Receipt.Generation != rotation.PendingGeneration ||
		request.Receipt.RecipientPrincipal != delivery.RecipientPrincipal ||
		request.Receipt.DeliveryKeyDigest != delivery.Sealed.Binding.DeliveryKeyDigest ||
		request.Receipt.CreatedAt.Before(rotation.Activation.EffectiveAt) ||
		request.Receipt.CreatedAt.After(now) ||
		!request.Receipt.CreatedAt.Before(delivery.Deadline) ||
		!request.Receipt.CreatedAt.Before(delivery.Sealed.Binding.ExpiresAt) ||
		!now.Before(delivery.Deadline) ||
		!now.Before(delivery.Sealed.Binding.ExpiresAt) ||
		!now.Before(rotation.DrainDeadline) {
		return ActivationAcknowledgeResult{}, ErrInvalidArgument
	}
	audit := newDeliveryAudit(
		rotationAuditID(command.Action+"-active", delivery.DeliveryID),
		command, rotation.OperationID, state.AuditHead, now,
	)
	err = s.commitCheckpointTransition(ctx, &state, audit, now, func(next *Ledger, checkpoint SignedCheckpoint) error {
		next.InitialGenerationDeliveries[deliveryIndex].ActiveReceipt = request.Receipt
		if membershipCompletionSatisfied(*next, next.Rotations[rotationIndex]) {
			completeMembershipRotation(
				next, &next.Rotations[rotationIndex], checkpoint.AuthoritySequence,
			)
		}
		return nil
	})
	if err != nil {
		return ActivationAcknowledgeResult{}, err
	}
	return activationAcknowledgeResult(state, state.Rotations[rotationIndex]), nil
}

func validateRotationRequest(request RotationRequest) error {
	if request.Version != ContractVersion {
		return ErrUnsupportedVersion
	}
	if len(request.RequestID) == 0 || len(request.RequestID) > MaxRequestIDBytes ||
		strings.TrimSpace(request.RequestID) != request.RequestID ||
		!ValidRealmID(request.RealmID) || zeroFixedID(request.ChannelID) ||
		len(request.RecipientAttestations) == 0 ||
		len(request.RecipientAttestations) > MaxMembersPerChannel ||
		request.ValidFor <= 0 || request.ValidFor > maximumInitialGenerationValidity ||
		request.ValidFor%time.Second != 0 ||
		request.DrainFor <= 0 || request.DrainFor > MaximumPreviousGenerationDrain ||
		request.DrainFor%time.Second != 0 ||
		!validMembershipChangeInput(request.MembershipChange, request.TargetPrincipal) {
		return ErrInvalidArgument
	}
	return nil
}

func rotationPayloadHash(request RotationRequest) string {
	raw, _ := json.Marshal(struct {
		Version          uint32
		RealmID          string
		ChannelID        [16]byte
		Attestations     []identityapi.CapabilityDeliveryAttestation
		ValidFor         int64
		DrainFor         int64
		MembershipChange MembershipChangeKind
		TargetPrincipal  string
	}{
		request.Version, request.RealmID, request.ChannelID,
		request.RecipientAttestations, int64(request.ValidFor / time.Second),
		int64(request.DrainFor / time.Second), request.MembershipChange, request.TargetPrincipal,
	})
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func rotationOperationID(realmID, requestID string) string {
	sum := sha256.Sum256([]byte("ardents:channel-rotation-operation:v1\x00" + realmID + "\x00" + requestID))
	return "rao1_" + hex.EncodeToString(sum[:16])
}

func rotationDeliveryID(operationID, recipient string) string {
	sum := sha256.Sum256([]byte("ardents:channel-rotation-delivery:v1\x00" + operationID + "\x00" + recipient))
	return "rad1_" + hex.EncodeToString(sum[:16])
}

func rotationAuditID(action, identity string) string {
	sum := sha256.Sum256([]byte("ardents:channel-rotation-audit:v1\x00" + action + "\x00" + identity))
	return "raa1_" + hex.EncodeToString(sum[:16])
}

func rotationResult(state Ledger, rotation RotationRecord) RotationResult {
	deliveries := make([]RotationDelivery, 0, len(rotation.DeliveryIDs))
	for _, deliveryID := range rotation.DeliveryIDs {
		index := deliveryRecordIndex(state, deliveryID)
		if index < 0 {
			continue
		}
		record := state.InitialGenerationDeliveries[index]
		sealed := record.Sealed
		sealed.Envelope = append([]byte(nil), sealed.Envelope...)
		deliveries = append(deliveries, RotationDelivery{
			DeliveryID: record.DeliveryID, RecipientPrincipal: record.RecipientPrincipal,
			Sealed: sealed,
		})
	}
	return RotationResult{
		Version: ContractVersion, RealmID: state.RealmID, OperationID: rotation.OperationID,
		AuthoritySequence: rotation.PrepareSequence, ChannelID: rotation.ChannelID,
		PreviousGeneration: rotation.PreviousGeneration, PendingGeneration: rotation.PendingGeneration,
		Phase: rotation.Phase, Deliveries: deliveries,
		MembershipChange: rotation.MembershipChange,
	}
}

func activationCommitResult(state Ledger, rotation RotationRecord) ActivationCommitResult {
	activation := rotation.Activation
	activation.Signature = append([]byte(nil), activation.Signature...)
	return ActivationCommitResult{
		Version: ContractVersion, RealmID: state.RealmID, OperationID: rotation.OperationID,
		AuthoritySequence: activation.AuthoritySequence, Phase: rotation.Phase,
		Activation: activation,
	}
}

func activationAcknowledgeResult(state Ledger, rotation RotationRecord) ActivationAcknowledgeResult {
	sequence := state.AuthoritySequence
	if rotation.CompletionSequence != 0 {
		sequence = rotation.CompletionSequence
	}
	return ActivationAcknowledgeResult{
		Version: ContractVersion, RealmID: state.RealmID, OperationID: rotation.OperationID,
		AuthoritySequence: sequence, Phase: rotation.Phase,
		CurrentGeneration:  rotation.PendingGeneration,
		PreviousGeneration: rotation.PreviousGeneration, DrainDeadline: rotation.DrainDeadline,
	}
}

func channelRecordIndex(state Ledger, channelID [16]byte) int {
	for index := range state.Channels {
		if state.Channels[index].ID == channelID {
			return index
		}
	}
	return -1
}

func rotationRecordIndex(state Ledger, operationID string) int {
	for index := range state.Rotations {
		if state.Rotations[index].OperationID == operationID {
			return index
		}
	}
	return -1
}

func deliveryRecordIndex(state Ledger, deliveryID string) int {
	for index := range state.InitialGenerationDeliveries {
		if state.InitialGenerationDeliveries[index].DeliveryID == deliveryID {
			return index
		}
	}
	return -1
}

func channelCurrentGrants(channel ChannelRecord) []CapabilityGrantRecord {
	if len(channel.CurrentGrants) > 0 {
		return channel.CurrentGrants
	}
	if channel.Grant.Version != 0 {
		return []CapabilityGrantRecord{channel.Grant}
	}
	return nil
}

func cloneGrantRecords(records []CapabilityGrantRecord) []CapabilityGrantRecord {
	out := append([]CapabilityGrantRecord(nil), records...)
	for index := range out {
		out[index].Secret = append([]byte(nil), out[index].Secret...)
		out[index].Signature = append([]byte(nil), out[index].Signature...)
	}
	return out
}

func incompleteRotationForChannel(state Ledger, channelID [16]byte) bool {
	for _, rotation := range state.Rotations {
		if rotation.ChannelID == channelID && rotation.Phase != DeliveryPhaseCompleted {
			return true
		}
	}
	return false
}

func containsDeliveryID(deliveryIDs []string, deliveryID string) bool {
	for _, candidate := range deliveryIDs {
		if candidate == deliveryID {
			return true
		}
	}
	return false
}

func rotationDeliveriesAtPhase(state Ledger, rotation RotationRecord, phase string) bool {
	for _, deliveryID := range rotation.DeliveryIDs {
		index := deliveryRecordIndex(state, deliveryID)
		if index < 0 || state.InitialGenerationDeliveries[index].Phase != phase {
			return false
		}
	}
	return len(rotation.DeliveryIDs) > 0
}

func rotationActiveReceiptsComplete(state Ledger, rotation RotationRecord) bool {
	for _, deliveryID := range rotation.DeliveryIDs {
		index := deliveryRecordIndex(state, deliveryID)
		if index < 0 ||
			state.InitialGenerationDeliveries[index].ActiveReceipt.Phase !=
				identitycapability.DeliveryPhaseActive {
			return false
		}
	}
	return len(rotation.DeliveryIDs) > 0
}

func setOperationPhase(state *Ledger, operationID, phase string) {
	for index := range state.Operations {
		if state.Operations[index].ID == operationID {
			state.Operations[index].Phase = phase
			return
		}
	}
}

func zeroFixedID(value [16]byte) bool {
	var combined byte
	for _, item := range value {
		combined |= item
	}
	return combined == 0
}
