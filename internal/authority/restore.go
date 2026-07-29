package authority

import (
	"context"
	"crypto/ed25519"
	"errors"
)

type VerifyRestoreRequest struct {
	Version           uint32 `json:"version"`
	RealmID           string `json:"realm_id"`
	AuthoritySequence uint64 `json:"authority_sequence"`
	CheckpointDigest  string `json:"checkpoint_digest"`
}

type VerifyRestoreResult struct {
	Version           uint32 `json:"version"`
	RealmID           string `json:"realm_id"`
	AuthorityEpoch    uint64 `json:"authority_epoch"`
	AuthoritySequence uint64 `json:"authority_sequence"`
	CheckpointDigest  string `json:"checkpoint_digest"`
	Phase             string `json:"phase"`
	Readiness         string `json:"readiness"`
}

// VerifyRestoredAuthority opens a recovery-only instance after proving that
// the restored ledger, retained signer, and independently administered
// repository head describe the exact same authority sequence. It never repairs
// either persistence boundary.
func (s *Service) VerifyRestoredAuthority(
	ctx context.Context,
	command Command,
	request VerifyRestoreRequest,
) (VerifyRestoreResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := validateVerifyRestoreRequest(request); err != nil {
		return VerifyRestoreResult{}, err
	}
	if err := validateVerifyRestoreCommand(command, request.RealmID); err != nil {
		return VerifyRestoreResult{}, err
	}
	if !s.recoveryOnly {
		return VerifyRestoreResult{}, ErrConflict
	}
	if s.store == nil || s.signer == nil || s.repository == nil || s.policy == nil {
		return VerifyRestoreResult{}, ErrUnavailable
	}
	if err := s.policy.AdmitAuthorityRecovery(ctx, command); err != nil {
		return VerifyRestoreResult{}, ErrPermissionDenied
	}

	state, err := s.readExactRestore(ctx)
	if err != nil {
		return VerifyRestoreResult{}, err
	}
	if state.RealmID != request.RealmID ||
		state.AuthoritySequence != request.AuthoritySequence ||
		state.Checkpoint.Digest != request.CheckpointDigest {
		s.setRecovery(statusFromLedger(state), ReasonCheckpointMismatch)
		return VerifyRestoreResult{}, ErrRecoveryRequired
	}

	s.recoveryOnly = false
	s.status = statusFromLedger(state)
	return VerifyRestoreResult{
		Version:           ContractVersion,
		RealmID:           state.RealmID,
		AuthorityEpoch:    state.AuthorityEpoch,
		AuthoritySequence: state.AuthoritySequence,
		CheckpointDigest:  state.Checkpoint.Digest,
		Phase:             state.Phase,
		Readiness:         state.Readiness,
	}, nil
}

func (s *Service) reconcileRecoveryOnly(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()

	state, err := s.readExactRestore(ctx)
	if err != nil {
		return
	}
	s.status = statusFromLedger(state)
	s.applyRecoveryOnlyStatus()
}

func (s *Service) readExactRestore(ctx context.Context) (Ledger, error) {
	if s.store == nil {
		s.setUnavailable(ReasonStoreUnavailable)
		return Ledger{}, ErrUnavailable
	}
	if s.signer == nil {
		s.setUnavailable(ReasonSignerUnavailable)
		return Ledger{}, ErrUnavailable
	}
	if s.repository == nil {
		s.setUnavailable(ReasonRepositoryUnavailable)
		return Ledger{}, ErrUnavailable
	}

	state, found, err := s.store.Load(ctx)
	if err != nil {
		if errors.Is(err, ErrCorruptState) || errors.Is(err, ErrUnsupportedVersion) {
			s.setRecovery(Status{}, ReasonPersistedStateInvalid)
			return Ledger{}, ErrRecoveryRequired
		}
		s.setUnavailable(ReasonStoreUnavailable)
		return Ledger{}, ErrUnavailable
	}
	if !found {
		s.setRecovery(Status{}, ReasonPersistedStateInvalid)
		return Ledger{}, ErrRecoveryRequired
	}
	if err := validateLedger(state); err != nil || state.Phase != PhaseReady {
		s.setRecovery(statusFromLedger(state), ReasonPersistedStateInvalid)
		return Ledger{}, ErrRecoveryRequired
	}

	principal, publicKey, err := s.signerBinding(ctx)
	if err != nil {
		if errors.Is(err, ErrRecoveryRequired) {
			s.setRecovery(statusFromLedger(state), ReasonSignerMismatch)
			return Ledger{}, ErrRecoveryRequired
		}
		s.setUnavailable(ReasonSignerUnavailable)
		return Ledger{}, ErrUnavailable
	}
	if principal != state.AuthorityPrincipal ||
		!ed25519.PublicKey(publicKey).Equal(ed25519.PublicKey(state.AuthorityPublicKey)) {
		s.setRecovery(statusFromLedger(state), ReasonSignerMismatch)
		return Ledger{}, ErrRecoveryRequired
	}

	head, found, err := s.repository.ReadHead(ctx, state.RealmID)
	if err != nil {
		switch {
		case errors.Is(err, ErrCheckpointHistoryPartial):
			s.setRecovery(statusFromLedger(state), ReasonCheckpointHistoryPartial)
			return Ledger{}, ErrRecoveryRequired
		case errors.Is(err, ErrCheckpointHistoryFork):
			s.setRecovery(statusFromLedger(state), ReasonCheckpointHistoryFork)
			return Ledger{}, ErrRecoveryRequired
		case errors.Is(err, ErrAuthorityGenerationMismatch):
			s.setRecovery(statusFromLedger(state), ReasonAuthorityGenerationMismatch)
			return Ledger{}, ErrRecoveryRequired
		case errors.Is(err, ErrCorruptState), errors.Is(err, ErrUnsupportedVersion):
			s.setRecovery(statusFromLedger(state), ReasonCheckpointMismatch)
			return Ledger{}, ErrRecoveryRequired
		}
		s.status = statusFromLedger(state)
		s.status.Readiness, s.status.Reason = ReadinessUnavailable, ReasonRepositoryUnavailable
		return Ledger{}, ErrUnavailable
	}
	if !found {
		s.setRecovery(statusFromLedger(state), ReasonCheckpointMissing)
		return Ledger{}, ErrRecoveryRequired
	}
	if !checkpointsEqual(head, state.Checkpoint) {
		switch {
		case head.AuthoritySequence > state.Checkpoint.AuthoritySequence:
			s.setRecovery(statusFromLedger(state), ReasonAuthorityRollback)
		case head.AuthoritySequence < state.Checkpoint.AuthoritySequence:
			s.setRecovery(statusFromLedger(state), ReasonCheckpointHistoryPartial)
		case head.AuthorityEpoch != state.Checkpoint.AuthorityEpoch ||
			head.AuthorityPrincipal != state.Checkpoint.AuthorityPrincipal ||
			!equalBytes(head.AuthorityPublicKey, state.Checkpoint.AuthorityPublicKey):
			s.setRecovery(statusFromLedger(state), ReasonAuthorityGenerationMismatch)
		default:
			s.setRecovery(statusFromLedger(state), ReasonCheckpointHistoryFork)
		}
		return Ledger{}, ErrRecoveryRequired
	}
	return state, nil
}

func validateVerifyRestoreRequest(request VerifyRestoreRequest) error {
	if request.Version != ContractVersion {
		return ErrUnsupportedVersion
	}
	if !ValidRealmID(request.RealmID) || request.AuthoritySequence == 0 ||
		!digestPattern.MatchString(request.CheckpointDigest) {
		return ErrInvalidArgument
	}
	return nil
}

func validateVerifyRestoreCommand(command Command, realmID string) error {
	if command.Actor == "" || command.Actor != command.Effective ||
		command.Action != ActionVerifyRestore ||
		command.ResourceKind != ResourceKindRealm ||
		command.ResourceID != realmID {
		return ErrPermissionDenied
	}
	return nil
}
