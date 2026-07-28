package authority

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	identitycapability "ardents/internal/identity/capability"
	identityprincipal "ardents/internal/identity/principal"
)

const authorityTransitionDomain = "ardents:realm-authority-transition:v1\x00"

type PlanAuthorityTransitionRequest struct {
	Version           uint32 `json:"version"`
	RequestID         string `json:"request_id"`
	RealmID           string `json:"realm_id"`
	AuthoritySequence uint64 `json:"authority_sequence"`
	CheckpointDigest  string `json:"checkpoint_digest"`
}

type AuthorityTransition struct {
	Version                uint32    `json:"version"`
	RealmID                string    `json:"realm_id"`
	FromAuthorityPrincipal string    `json:"from_authority_principal"`
	FromAuthorityPublicKey []byte    `json:"from_authority_public_key"`
	ToAuthorityPrincipal   string    `json:"to_authority_principal"`
	ToAuthorityPublicKey   []byte    `json:"to_authority_public_key"`
	FromAuthorityEpoch     uint64    `json:"from_authority_epoch"`
	ToAuthorityEpoch       uint64    `json:"to_authority_epoch"`
	AuthoritySequence      uint64    `json:"authority_sequence"`
	CheckpointDigest       string    `json:"checkpoint_digest"`
	CreatedAt              time.Time `json:"created_at"`
	FromSignature          []byte    `json:"from_signature"`
	ToSignature            []byte    `json:"to_signature"`
}

type authorityTransitionBody struct {
	Version                uint32    `json:"version"`
	RealmID                string    `json:"realm_id"`
	FromAuthorityPrincipal string    `json:"from_authority_principal"`
	FromAuthorityPublicKey []byte    `json:"from_authority_public_key"`
	ToAuthorityPrincipal   string    `json:"to_authority_principal"`
	ToAuthorityPublicKey   []byte    `json:"to_authority_public_key"`
	FromAuthorityEpoch     uint64    `json:"from_authority_epoch"`
	ToAuthorityEpoch       uint64    `json:"to_authority_epoch"`
	AuthoritySequence      uint64    `json:"authority_sequence"`
	CheckpointDigest       string    `json:"checkpoint_digest"`
	CreatedAt              time.Time `json:"created_at"`
}

func (s *Service) PlanAuthorityTransition(
	ctx context.Context,
	command Command,
	request PlanAuthorityTransitionRequest,
	next Signer,
) (AuthorityTransition, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := validatePlanAuthorityTransitionRequest(request); err != nil {
		return AuthorityTransition{}, err
	}
	if err := validatePlanAuthorityTransitionCommand(command, request.RealmID); err != nil {
		return AuthorityTransition{}, err
	}
	if s.store == nil || s.signer == nil || s.repository == nil || s.policy == nil || next == nil {
		return AuthorityTransition{}, ErrUnavailable
	}
	if err := s.policy.AdmitAuthorityRecovery(ctx, command); err != nil {
		return AuthorityTransition{}, ErrPermissionDenied
	}

	state, found, err := s.store.Load(ctx)
	if err != nil || !found {
		s.setUnavailable(ReasonStoreUnavailable)
		return AuthorityTransition{}, ErrUnavailable
	}
	if err := validateLedger(state); err != nil {
		s.setRecovery(statusFromLedger(state), ReasonPersistedStateInvalid)
		return AuthorityTransition{}, ErrRecoveryRequired
	}
	if state.Transition != nil &&
		request.RequestID == state.Transition.RequestID {
		proof := state.Transition.Proof
		toPrincipal, toPublic, identityErr := signerIdentity(ctx, next)
		if identityErr != nil ||
			request.RealmID != proof.RealmID ||
			request.RequestID != state.Transition.RequestID ||
			request.AuthoritySequence != proof.AuthoritySequence ||
			request.CheckpointDigest != proof.CheckpointDigest ||
			toPrincipal != proof.ToAuthorityPrincipal ||
			!toPublic.Equal(ed25519.PublicKey(proof.ToAuthorityPublicKey)) {
			return AuthorityTransition{}, ErrConflict
		}
		if state.Phase == PhaseCheckpointing {
			s.signer, s.transitionSigner = next, next
			if err := s.reconcileLoaded(ctx, &state); err != nil {
				return AuthorityTransition{}, err
			}
		} else if state.Phase != PhaseReady {
			return AuthorityTransition{}, ErrRecoveryRequired
		}
		return proof, nil
	}
	if state.Phase != PhaseReady {
		s.setRecovery(statusFromLedger(state), ReasonPersistedStateInvalid)
		return AuthorityTransition{}, ErrRecoveryRequired
	}
	if state.Transition != nil {
		return AuthorityTransition{}, ErrConflict
	}
	if err := s.mutationFence(); err != nil {
		return AuthorityTransition{}, err
	}
	if state.RealmID != request.RealmID ||
		state.AuthoritySequence != request.AuthoritySequence ||
		state.Checkpoint.Digest != request.CheckpointDigest {
		return AuthorityTransition{}, ErrConflict
	}

	fromPrincipal, fromPublic, err := s.signerBinding(ctx)
	if err != nil {
		s.setUnavailable(ReasonSignerUnavailable)
		return AuthorityTransition{}, ErrUnavailable
	}
	if fromPrincipal != state.AuthorityPrincipal ||
		!fromPublic.Equal(ed25519.PublicKey(state.AuthorityPublicKey)) {
		s.markRecovery(ctx, &state, ReasonSignerMismatch)
		return AuthorityTransition{}, ErrRecoveryRequired
	}
	head, found, err := s.repository.ReadHead(ctx, state.RealmID)
	if err != nil {
		if errors.Is(err, ErrCorruptState) || errors.Is(err, ErrUnsupportedVersion) {
			s.markRecovery(ctx, &state, ReasonCheckpointMismatch)
			return AuthorityTransition{}, ErrRecoveryRequired
		}
		s.status = statusFromLedger(state)
		s.status.Readiness, s.status.Reason = ReadinessUnavailable, ReasonRepositoryUnavailable
		return AuthorityTransition{}, ErrUnavailable
	}
	if !found || !checkpointsEqual(head, state.Checkpoint) {
		s.markRecovery(ctx, &state, ReasonCheckpointMismatch)
		return AuthorityTransition{}, ErrRecoveryRequired
	}

	toPrincipal, toPublic, err := signerIdentity(ctx, next)
	if err != nil {
		return AuthorityTransition{}, ErrUnavailable
	}
	if toPrincipal == fromPrincipal || toPublic.Equal(fromPublic) {
		return AuthorityTransition{}, ErrConflict
	}
	transition := AuthorityTransition{
		Version: ContractVersion, RealmID: state.RealmID,
		FromAuthorityPrincipal: fromPrincipal,
		FromAuthorityPublicKey: append([]byte(nil), fromPublic...),
		ToAuthorityPrincipal:   toPrincipal,
		ToAuthorityPublicKey:   append([]byte(nil), toPublic...),
		FromAuthorityEpoch:     state.AuthorityEpoch,
		ToAuthorityEpoch:       state.AuthorityEpoch + 1,
		AuthoritySequence:      state.AuthoritySequence,
		CheckpointDigest:       state.Checkpoint.Digest,
		CreatedAt:              s.clock().UTC().Truncate(time.Second),
	}
	digest, err := authorityTransitionDigest(transition)
	if err != nil {
		return AuthorityTransition{}, err
	}
	transition.FromSignature, err = s.signer.Sign(ctx, digest)
	if err != nil {
		return AuthorityTransition{}, ErrUnavailable
	}
	transition.ToSignature, err = next.Sign(ctx, digest)
	if err != nil {
		return AuthorityTransition{}, ErrUnavailable
	}
	if err := ValidateAuthorityTransition(transition); err != nil {
		return AuthorityTransition{}, err
	}
	if err := checkpointTransitionCapacity(state); err != nil {
		return AuthorityTransition{}, err
	}
	if len(state.Operations) >= MaxOperations {
		return AuthorityTransition{}, ErrResourceExhausted
	}
	operationID := "rao1_" + hex.EncodeToString(digest[:16])
	auditIDSum := sha256.Sum256(append(
		[]byte("ardents:authority-transition-audit:v1\x00"), digest...,
	))
	auditID := "raa1_" + hex.EncodeToString(auditIDSum[:16])
	audit := AuditRecord{
		Version: ContractVersion, ID: auditID,
		Actor: command.Actor, Effective: command.Effective,
		Action: command.Action, ResourceKind: command.ResourceKind,
		ResourceID: command.ResourceID, OperationID: operationID,
		Outcome: "accepted", PreviousHash: state.AuditHead, CreatedAt: transition.CreatedAt,
	}
	audit.Hash = auditHash(audit)
	checkpoint, err := SignCheckpoint(ctx, next, Checkpoint{
		Version: ContractVersion, SchemaVersion: SchemaVersion,
		RealmID:            state.RealmID,
		AuthorityPrincipal: transition.ToAuthorityPrincipal,
		AuthorityPublicKey: append([]byte(nil), transition.ToAuthorityPublicKey...),
		AuthorityEpoch:     transition.ToAuthorityEpoch,
		AuthoritySequence:  state.AuthoritySequence + 1,
		PreviousDigest:     state.Checkpoint.Digest,
		AuditHead:          audit.Hash, CreatedAt: transition.CreatedAt,
		AuthorityTransition: &transition,
	})
	if err != nil {
		return AuthorityTransition{}, ErrUnavailable
	}
	required := make([][16]byte, 0, len(state.Channels))
	for _, channel := range state.Channels {
		required = append(required, channel.ID)
	}
	sortChannelIDs(required)
	previousSequence := state.AuthoritySequence
	expectedRevision := state.Revision
	state.Revision++
	state.AuthorityPrincipal = transition.ToAuthorityPrincipal
	state.AuthorityPublicKey = append([]byte(nil), transition.ToAuthorityPublicKey...)
	state.AuthorityEpoch = transition.ToAuthorityEpoch
	state.AuthoritySequence = checkpoint.AuthoritySequence
	state.AuditHead, state.Checkpoint = audit.Hash, checkpoint
	state.Phase, state.Readiness, state.Reason =
		PhaseCheckpointing, ReadinessDegraded, ReasonCheckpointMissing
	state.AuditLog = append(state.AuditLog, audit)
	state.AuditOutbox = append(state.AuditOutbox, audit)
	state.Operations = append(state.Operations, OperationRecord{
		Version: ContractVersion, ID: operationID, RequestID: request.RequestID,
		Kind: "authority_transition", Phase: PhaseCheckpointing,
		CreatedAt: transition.CreatedAt,
		Deadline:  transition.CreatedAt.Add(MaxOperationLifetime),
	})
	state.Transition = &AuthorityTransitionRecord{
		Version: ContractVersion, RequestID: request.RequestID,
		OperationID: operationID, Proof: transition,
		RequiredRotationChannelIDs: required,
	}
	if err := s.store.Save(ctx, expectedRevision, state); err != nil {
		s.setUnavailable(ReasonStoreUnavailable)
		return AuthorityTransition{}, ErrUnavailable
	}
	s.signer, s.transitionSigner = next, next
	s.status = statusFromLedger(state)
	if _, err := s.repository.CompareAndAppend(
		ctx, state.RealmID, previousSequence, checkpoint,
	); err != nil {
		if errors.Is(err, ErrConflict) || errors.Is(err, ErrCorruptState) {
			s.markRecovery(ctx, &state, ReasonCheckpointMismatch)
			return AuthorityTransition{}, ErrRecoveryRequired
		}
		s.status.Readiness, s.status.Reason =
			ReadinessUnavailable, ReasonRepositoryUnavailable
		return AuthorityTransition{}, ErrUnavailable
	}
	expectedRevision = state.Revision
	state.Revision++
	state.Phase, state.Readiness, state.Reason = PhaseReady, ReadinessReady, ReasonNone
	setOperationPhase(&state, operationID, PhaseReady)
	if err := s.store.Save(ctx, expectedRevision, state); err != nil {
		s.setUnavailable(ReasonStoreUnavailable)
		return AuthorityTransition{}, ErrUnavailable
	}
	s.status = statusFromLedger(state)
	s.flushAudit(ctx, &state)
	s.applyTransitionStatus(state)
	return transition, nil
}

func sortChannelIDs(ids [][16]byte) {
	sort.Slice(ids, func(i, j int) bool {
		return bytes.Compare(ids[i][:], ids[j][:]) < 0
	})
}

func ValidateAuthorityTransition(transition AuthorityTransition) error {
	if transition.Version != ContractVersion ||
		!ValidRealmID(transition.RealmID) ||
		len(transition.FromAuthorityPublicKey) != ed25519.PublicKeySize ||
		len(transition.ToAuthorityPublicKey) != ed25519.PublicKeySize ||
		transition.FromAuthorityEpoch == 0 ||
		transition.ToAuthorityEpoch != transition.FromAuthorityEpoch+1 ||
		transition.AuthoritySequence == 0 ||
		!digestPattern.MatchString(transition.CheckpointDigest) ||
		!canonicalSecond(transition.CreatedAt) ||
		len(transition.FromSignature) != ed25519.SignatureSize ||
		len(transition.ToSignature) != ed25519.SignatureSize {
		return ErrInvalidArgument
	}
	fromPrincipal, err := identityprincipal.FromEd25519PublicKey(
		ed25519.PublicKey(transition.FromAuthorityPublicKey),
	)
	if err != nil || fromPrincipal.String() != transition.FromAuthorityPrincipal {
		return ErrInvalidArgument
	}
	toPrincipal, err := identityprincipal.FromEd25519PublicKey(
		ed25519.PublicKey(transition.ToAuthorityPublicKey),
	)
	if err != nil || toPrincipal.String() != transition.ToAuthorityPrincipal ||
		toPrincipal.String() == fromPrincipal.String() {
		return ErrInvalidArgument
	}
	digest, err := authorityTransitionDigest(transition)
	if err != nil {
		return err
	}
	if !ed25519.Verify(ed25519.PublicKey(transition.FromAuthorityPublicKey), digest, transition.FromSignature) ||
		!ed25519.Verify(ed25519.PublicKey(transition.ToAuthorityPublicKey), digest, transition.ToSignature) {
		return ErrPermissionDenied
	}
	return nil
}

func authorityTransitionDigest(transition AuthorityTransition) ([]byte, error) {
	body := authorityTransitionBody{
		Version: transition.Version, RealmID: transition.RealmID,
		FromAuthorityPrincipal: transition.FromAuthorityPrincipal,
		FromAuthorityPublicKey: transition.FromAuthorityPublicKey,
		ToAuthorityPrincipal:   transition.ToAuthorityPrincipal,
		ToAuthorityPublicKey:   transition.ToAuthorityPublicKey,
		FromAuthorityEpoch:     transition.FromAuthorityEpoch,
		ToAuthorityEpoch:       transition.ToAuthorityEpoch,
		AuthoritySequence:      transition.AuthoritySequence,
		CheckpointDigest:       transition.CheckpointDigest,
		CreatedAt:              transition.CreatedAt,
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, ErrInvalidArgument
	}
	sum := sha256.Sum256(append([]byte(authorityTransitionDomain), raw...))
	return sum[:], nil
}

func signerIdentity(ctx context.Context, signer Signer) (string, ed25519.PublicKey, error) {
	if signer == nil {
		return "", nil, ErrUnavailable
	}
	principal, err := signer.Principal(ctx)
	if err != nil {
		return "", nil, err
	}
	publicKey, err := signer.PublicKey(ctx)
	if err != nil || len(publicKey) != ed25519.PublicKeySize {
		return "", nil, ErrUnavailable
	}
	derived, err := identityprincipal.FromEd25519PublicKey(publicKey)
	if err != nil || derived.String() != principal {
		return "", nil, ErrPermissionDenied
	}
	return principal, append(ed25519.PublicKey(nil), publicKey...), nil
}

func validatePlanAuthorityTransitionRequest(request PlanAuthorityTransitionRequest) error {
	if request.Version != ContractVersion {
		return ErrUnsupportedVersion
	}
	if len(request.RequestID) == 0 || len(request.RequestID) > MaxRequestIDBytes ||
		strings.TrimSpace(request.RequestID) != request.RequestID ||
		!ValidRealmID(request.RealmID) || request.AuthoritySequence == 0 ||
		!digestPattern.MatchString(request.CheckpointDigest) {
		return ErrInvalidArgument
	}
	return nil
}

func validatePlanAuthorityTransitionCommand(command Command, realmID string) error {
	if command.Actor == "" || command.Actor != command.Effective ||
		command.Action != ActionPlanTransition ||
		command.ResourceKind != ResourceKindRealm ||
		command.ResourceID != realmID {
		return ErrPermissionDenied
	}
	return nil
}

func validateAuthorityTransitionRecord(state Ledger) error {
	record := state.Transition
	if record == nil {
		if state.AuthorityEpoch != 1 {
			return ErrCorruptState
		}
		return nil
	}
	if record.Version != ContractVersion ||
		len(record.RequestID) == 0 || len(record.RequestID) > MaxRequestIDBytes ||
		!operationIDPattern.MatchString(record.OperationID) ||
		ValidateAuthorityTransition(record.Proof) != nil ||
		record.Proof.RealmID != state.RealmID ||
		record.Proof.ToAuthorityPrincipal != state.AuthorityPrincipal ||
		!equalBytes(record.Proof.ToAuthorityPublicKey, state.AuthorityPublicKey) ||
		record.Proof.ToAuthorityEpoch != state.AuthorityEpoch ||
		record.Proof.AuthoritySequence >= state.AuthoritySequence ||
		len(record.RequiredRotationChannelIDs) != len(state.Channels) ||
		len(record.RotatedChannelIDs) > len(record.RequiredRotationChannelIDs) {
		return ErrCorruptState
	}
	operationIndex := transitionOperationIndex(state, record.OperationID)
	if operationIndex < 0 ||
		state.Operations[operationIndex].Kind != "authority_transition" ||
		state.Operations[operationIndex].RequestID != record.RequestID {
		return ErrCorruptState
	}
	required := make(map[[16]byte]struct{}, len(record.RequiredRotationChannelIDs))
	for index, channelID := range record.RequiredRotationChannelIDs {
		if zeroFixedID(channelID) ||
			(index > 0 && bytes.Compare(
				record.RequiredRotationChannelIDs[index-1][:], channelID[:],
			) >= 0) {
			return ErrCorruptState
		}
		required[channelID] = struct{}{}
	}
	for _, channel := range state.Channels {
		if _, ok := required[channel.ID]; !ok {
			return ErrCorruptState
		}
	}
	for index, channelID := range record.RotatedChannelIDs {
		if _, ok := required[channelID]; !ok ||
			(index > 0 && bytes.Compare(
				record.RotatedChannelIDs[index-1][:], channelID[:],
			) >= 0) {
			return ErrCorruptState
		}
	}
	return nil
}

func transitionOperationIndex(state Ledger, operationID string) int {
	for index := range state.Operations {
		if state.Operations[index].ID == operationID {
			return index
		}
	}
	return -1
}

func authorityTransitionPending(record *AuthorityTransitionRecord) bool {
	return record != nil &&
		len(record.RotatedChannelIDs) < len(record.RequiredRotationChannelIDs)
}

func authorityTransitionRotationAllowed(
	record *AuthorityTransitionRecord,
	channelID [16]byte,
) bool {
	if record == nil {
		return false
	}
	required := false
	for _, candidate := range record.RequiredRotationChannelIDs {
		required = required || candidate == channelID
	}
	for _, completed := range record.RotatedChannelIDs {
		if completed == channelID {
			return false
		}
	}
	return required
}

func completeAuthorityTransitionRotation(state *Ledger, channelID [16]byte) {
	if state.Transition == nil ||
		!authorityTransitionRotationAllowed(state.Transition, channelID) {
		return
	}
	state.Transition.RotatedChannelIDs = append(
		state.Transition.RotatedChannelIDs, channelID,
	)
	sortChannelIDs(state.Transition.RotatedChannelIDs)
}

func cloneAuthorityTransitionValue(value AuthorityTransition) AuthorityTransition {
	value.FromAuthorityPublicKey = append([]byte(nil), value.FromAuthorityPublicKey...)
	value.ToAuthorityPublicKey = append([]byte(nil), value.ToAuthorityPublicKey...)
	value.FromSignature = append([]byte(nil), value.FromSignature...)
	value.ToSignature = append([]byte(nil), value.ToSignature...)
	return value
}

// AdoptMemberAuthorityTransition verifies the dual-signed authority proof and
// atomically teaches a member capability service to accept the successor for
// the required post-transition channel rotations.
func AdoptMemberAuthorityTransition(
	member *identitycapability.Service,
	transition AuthorityTransition,
) error {
	if member == nil {
		return ErrInvalidArgument
	}
	if err := ValidateAuthorityTransition(transition); err != nil {
		return err
	}
	if err := member.AdoptChannelIssuerTransition(
		transition.FromAuthorityPrincipal,
		ed25519.PublicKey(transition.FromAuthorityPublicKey),
		transition.ToAuthorityPrincipal,
		ed25519.PublicKey(transition.ToAuthorityPublicKey),
	); err != nil {
		return ErrPermissionDenied
	}
	return nil
}

// FinalizeMemberAuthorityTransition retires predecessor issuance trust only
// after the durable authority record proves every required channel completed.
func FinalizeMemberAuthorityTransition(
	member *identitycapability.Service,
	record AuthorityTransitionRecord,
) error {
	if member == nil || authorityTransitionPending(&record) ||
		ValidateAuthorityTransition(record.Proof) != nil {
		return ErrInvalidArgument
	}
	if err := member.FinalizeChannelIssuerTransition(
		record.Proof.FromAuthorityPrincipal,
		record.Proof.ToAuthorityPrincipal,
	); err != nil {
		return ErrPermissionDenied
	}
	return nil
}
