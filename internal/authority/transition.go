package authority

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"time"

	identityprincipal "ardents/internal/identity/principal"
)

const authorityTransitionDomain = "ardents:realm-authority-transition:v1\x00"

type PlanAuthorityTransitionRequest struct {
	Version           uint32 `json:"version"`
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
	if err := s.mutationFence(); err != nil {
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
	if err := validateLedger(state); err != nil || state.Phase != PhaseReady {
		s.setRecovery(statusFromLedger(state), ReasonPersistedStateInvalid)
		return AuthorityTransition{}, ErrRecoveryRequired
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
	return transition, nil
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
	if !ValidRealmID(request.RealmID) || request.AuthoritySequence == 0 ||
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
