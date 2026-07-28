package authority

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"sync"
	"time"

	identityprincipal "ardents/internal/identity/principal"
)

type Store interface {
	Load(context.Context) (Ledger, bool, error)
	Create(context.Context, Ledger) error
	Save(context.Context, uint64, Ledger) error
}

type CheckpointRepository interface {
	ReadHead(context.Context, string) (SignedCheckpoint, bool, error)
	CreateIfAbsent(context.Context, SignedCheckpoint) (SignedCheckpoint, error)
	CompareAndAppend(context.Context, string, uint64, SignedCheckpoint) (SignedCheckpoint, error)
}

type CrashBoundary string

const (
	CrashAfterLedgerCommit     CrashBoundary = "after_ledger_commit"
	CrashAfterCheckpointCreate CrashBoundary = "after_checkpoint_create"
)

type Config struct {
	Store        Store
	Signer       Signer
	Repository   CheckpointRepository
	Random       io.Reader
	Clock        func() time.Time
	Policy       ProductPolicy
	Audit        AuditSink
	Crash        func(CrashBoundary) error
	RecoveryOnly bool
	// SuccessorSigner is preprovisioned independently and is used only to
	// roll forward an already committed dual-signed authority transition.
	SuccessorSigner Signer
}

type Service struct {
	mu                sync.Mutex
	store             Store
	signer            Signer
	repository        CheckpointRepository
	random            io.Reader
	clock             func() time.Time
	policy            ProductPolicy
	audit             AuditSink
	crash             func(CrashBoundary) error
	status            Status
	recoveryOnly      bool
	migrationPending  bool
	transitionPending bool
	transitionSigner  Signer
}

func New(config Config) *Service {
	service := &Service{
		store: config.Store, signer: config.Signer, repository: config.Repository,
		random: config.Random, clock: config.Clock, policy: config.Policy, crash: config.Crash,
		audit:            config.Audit,
		recoveryOnly:     config.RecoveryOnly,
		transitionSigner: config.SuccessorSigner,
		status: Status{
			Version: ContractVersion, SchemaVersion: SchemaVersion,
			Phase: PhaseUninitialized, Readiness: ReadinessUnavailable, Reason: ReasonUninitialized,
		},
	}
	if service.random == nil {
		service.random = rand.Reader
	}
	if service.clock == nil {
		service.clock = time.Now
	}
	if service.crash == nil {
		service.crash = func(CrashBoundary) error { return nil }
	}
	if service.recoveryOnly {
		service.reconcileRecoveryOnly(context.Background())
	} else {
		service.reconcile(context.Background())
	}
	service.refreshMigrationStatus(context.Background())
	return service
}

func (s *Service) Readiness() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}

func (s *Service) CreateOrReopen(ctx context.Context, command Command, request CreateRequest) (CreateResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := validateCreateRequest(request); err != nil {
		return CreateResult{}, err
	}
	if err := validateCreateCommand(command); err != nil {
		return CreateResult{}, err
	}
	if err := s.mutationFence(); err != nil {
		return CreateResult{}, err
	}
	if s.store == nil || s.signer == nil || s.repository == nil || s.policy == nil {
		return CreateResult{}, ErrUnavailable
	}
	if err := s.policy.AdmitRealmGenesis(ctx, command); err != nil {
		return CreateResult{}, ErrPermissionDenied
	}
	state, found, err := s.store.Load(ctx)
	if err != nil {
		if errors.Is(err, ErrCorruptState) || errors.Is(err, ErrUnsupportedVersion) {
			s.setRecovery(Status{}, ReasonPersistedStateInvalid)
			return CreateResult{}, ErrRecoveryRequired
		}
		s.setUnavailable(ReasonStoreUnavailable)
		return CreateResult{}, ErrUnavailable
	}
	payloadHash := requestPayloadHash(request)
	if found {
		if err := validateLedger(state); err != nil {
			s.setRecovery(Status{}, ReasonPersistedStateInvalid)
			return CreateResult{}, ErrRecoveryRequired
		}
		for _, record := range state.Idempotency {
			if record.RequestID != request.RequestID {
				continue
			}
			if record.PayloadHash != payloadHash {
				return CreateResult{}, ErrConflict
			}
			if state.Phase != PhaseReady || len(state.AuditOutbox) > 0 {
				if err := s.reconcileLoaded(ctx, &state); err != nil {
					return CreateResult{}, err
				}
			}
			result := record.Result
			result.Phase = state.Phase
			return result, nil
		}
		return CreateResult{}, ErrConflict
	}
	if err := validateCreatePayload(request); err != nil {
		return CreateResult{}, err
	}
	principal, publicKey, err := s.signerBinding(ctx)
	if err != nil {
		s.setUnavailable(ReasonSignerUnavailable)
		return CreateResult{}, err
	}
	realmID, err := s.randomID("r1_", 16)
	if err != nil {
		return CreateResult{}, ErrUnavailable
	}
	operationID, err := s.randomID("rao1_", 16)
	if err != nil {
		return CreateResult{}, ErrUnavailable
	}
	auditID, err := s.randomID("raa1_", 16)
	if err != nil {
		return CreateResult{}, ErrUnavailable
	}
	now := s.clock().UTC().Truncate(time.Second)
	audit := AuditRecord{
		Version: ContractVersion, ID: auditID, Actor: command.Actor, Effective: command.Effective,
		Action: command.Action, ResourceKind: command.ResourceKind, ResourceID: command.ResourceID,
		OperationID: operationID, Outcome: "accepted", CreatedAt: now,
	}
	audit.Hash = auditHash(audit)
	checkpoint, err := SignCheckpoint(ctx, s.signer, Checkpoint{
		Version: ContractVersion, SchemaVersion: SchemaVersion,
		RealmID: realmID, AuthorityPrincipal: principal, AuthorityPublicKey: publicKey,
		AuthorityEpoch: 1, AuthoritySequence: 1, AuditHead: audit.Hash, CreatedAt: now,
	})
	if err != nil {
		s.setUnavailable(ReasonSignerUnavailable)
		return CreateResult{}, ErrUnavailable
	}
	result := CreateResult{
		Version: ContractVersion, RealmID: realmID, OperationID: operationID,
		AuthorityEpoch: 1, AuthoritySequence: 1,
		CheckpointDigest: checkpoint.Digest, Phase: PhaseCheckpointing,
	}
	state = Ledger{
		Version: ContractVersion, SchemaVersion: SchemaVersion, Revision: 1,
		RealmID: realmID, RealmClass: request.RealmClass,
		AuthorityPrincipal: principal, AuthorityPublicKey: publicKey,
		AuthorityEpoch: 1, AuthoritySequence: 1,
		Phase: PhaseCheckpointing, Readiness: ReadinessDegraded, Reason: ReasonCheckpointMissing,
		AuditHead: audit.Hash, GenesisCheckpointDigest: checkpoint.Digest, Checkpoint: checkpoint,
		Members: []MemberRecord{}, Channels: []ChannelRecord{},
		Operations: []OperationRecord{{
			Version: ContractVersion, ID: operationID, RequestID: request.RequestID,
			Kind: "realm_genesis", Phase: PhaseCheckpointing, CreatedAt: now,
			Deadline: now.Add(MaxOperationLifetime),
		}},
		Idempotency: []IdempotencyRecord{{
			Version: ContractVersion, RequestID: request.RequestID,
			PayloadHash: payloadHash, Result: result,
		}},
		AuditLog: []AuditRecord{audit}, AuditOutbox: []AuditRecord{audit},
	}
	if err := s.store.Create(ctx, state); err != nil {
		if errors.Is(err, ErrConflict) {
			return CreateResult{}, ErrConflict
		}
		s.setUnavailable(ReasonStoreUnavailable)
		return CreateResult{}, ErrUnavailable
	}
	s.status = statusFromLedger(state)
	if err := s.crash(CrashAfterLedgerCommit); err != nil {
		return CreateResult{}, ErrUnavailable
	}
	if err := s.reconcileLoaded(ctx, &state); err != nil {
		return CreateResult{}, err
	}
	result.Phase = state.Phase
	return result, nil
}

func (s *Service) Inspect(_ context.Context, command Command, request InspectRequest) (Status, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := validateInspectRequest(request); err != nil {
		return Status{}, err
	}
	if command.Actor == "" || command.Actor != command.Effective ||
		command.Action != ActionInspect || command.ResourceKind != ResourceKindRealm ||
		command.ResourceID != request.RealmID {
		return Status{}, ErrPermissionDenied
	}
	if s.status.RealmID != "" && s.status.RealmID != request.RealmID {
		return Status{}, ErrPermissionDenied
	}
	return s.status, nil
}

func (s *Service) reconcile(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.store == nil {
		s.setUnavailable(ReasonStoreUnavailable)
		return
	}
	if s.signer == nil {
		s.setUnavailable(ReasonSignerUnavailable)
		return
	}
	if s.repository == nil {
		s.setUnavailable(ReasonRepositoryUnavailable)
		return
	}
	state, found, err := s.store.Load(ctx)
	if err != nil {
		if errors.Is(err, ErrCorruptState) || errors.Is(err, ErrUnsupportedVersion) {
			s.setRecovery(Status{}, ReasonPersistedStateInvalid)
			return
		}
		s.setUnavailable(ReasonStoreUnavailable)
		return
	}
	principal, publicKey, err := s.signerBinding(ctx)
	if err != nil {
		s.setUnavailable(ReasonSignerUnavailable)
		return
	}
	if !found {
		s.status = Status{
			Version: ContractVersion, SchemaVersion: SchemaVersion,
			Phase: PhaseUninitialized, Readiness: ReadinessReady, Reason: ReasonUninitialized,
		}
		return
	}
	if err := validateLedger(state); err != nil {
		s.setRecovery(Status{}, ReasonPersistedStateInvalid)
		return
	}
	s.status = statusFromLedger(state)
	if principal != state.AuthorityPrincipal || !ed25519.PublicKey(publicKey).Equal(ed25519.PublicKey(state.AuthorityPublicKey)) {
		successorPrincipal, successorPublic, successorErr := signerIdentity(
			ctx, s.transitionSigner,
		)
		if successorErr != nil || successorPrincipal != state.AuthorityPrincipal ||
			!successorPublic.Equal(ed25519.PublicKey(state.AuthorityPublicKey)) ||
			state.Transition == nil {
			s.setRecovery(statusFromLedger(state), ReasonSignerMismatch)
			return
		}
		s.signer = s.transitionSigner
	}
	_ = s.reconcileLoaded(ctx, &state)
}

func (s *Service) reconcileLoaded(ctx context.Context, state *Ledger) error {
	if state.Phase == PhaseRecoveryRequired {
		s.setRecovery(statusFromLedger(*state), state.Reason)
		return ErrRecoveryRequired
	}
	head, found, err := s.repository.ReadHead(ctx, state.RealmID)
	if err != nil {
		if errors.Is(err, ErrCorruptState) || errors.Is(err, ErrUnsupportedVersion) {
			s.markRecovery(ctx, state, ReasonCheckpointMismatch)
			return ErrRecoveryRequired
		}
		s.status = statusFromLedger(*state)
		s.status.Readiness, s.status.Reason = ReadinessUnavailable, ReasonRepositoryUnavailable
		return ErrUnavailable
	}
	if state.Phase == PhaseCheckpointing {
		switch {
		case !found && state.Checkpoint.AuthoritySequence == 1:
			head, err = s.repository.CreateIfAbsent(ctx, state.Checkpoint)
		case found && checkpointsEqual(head, state.Checkpoint):
			err = nil
		case found && state.Checkpoint.AuthoritySequence > 1 &&
			head.AuthoritySequence+1 == state.Checkpoint.AuthoritySequence &&
			head.Digest == state.Checkpoint.PreviousDigest:
			head, err = s.repository.CompareAndAppend(
				ctx, state.RealmID, head.AuthoritySequence, state.Checkpoint,
			)
		default:
			s.markRecovery(ctx, state, ReasonCheckpointMismatch)
			return ErrRecoveryRequired
		}
		if err != nil {
			if errors.Is(err, ErrConflict) || errors.Is(err, ErrCorruptState) {
				s.markRecovery(ctx, state, ReasonCheckpointMismatch)
				return ErrRecoveryRequired
			}
			s.status = statusFromLedger(*state)
			s.status.Readiness, s.status.Reason = ReadinessUnavailable, ReasonRepositoryUnavailable
			return ErrUnavailable
		}
		found = true
	}
	if !found {
		s.markRecovery(ctx, state, ReasonCheckpointMissing)
		return ErrRecoveryRequired
	}
	if !checkpointsEqual(head, state.Checkpoint) {
		s.markRecovery(ctx, state, ReasonCheckpointMismatch)
		return ErrRecoveryRequired
	}
	if state.Phase == PhaseCheckpointing {
		if err := s.crash(CrashAfterCheckpointCreate); err != nil {
			return ErrUnavailable
		}
		expected := state.Revision
		state.Revision++
		state.Phase, state.Readiness, state.Reason = PhaseReady, ReadinessReady, ReasonNone
		if state.AuthoritySequence == 1 {
			state.Operations[0].Phase = PhaseReady
			state.Idempotency[0].Result.Phase = PhaseReady
		}
		if state.Transition != nil {
			setOperationPhase(state, state.Transition.OperationID, PhaseReady)
		}
		if err := s.store.Save(ctx, expected, *state); err != nil {
			s.setUnavailable(ReasonStoreUnavailable)
			return ErrUnavailable
		}
	}
	s.status = statusFromLedger(*state)
	s.flushAudit(ctx, state)
	s.applyMigrationStatus(*state)
	s.applyTransitionStatus(*state)
	return nil
}

type AuditSink interface {
	RecordAuthorityAudit(context.Context, AuditRecord) error
}

func (s *Service) flushAudit(ctx context.Context, state *Ledger) {
	if s.audit == nil {
		return
	}
	for len(state.AuditOutbox) > 0 {
		record := state.AuditOutbox[0]
		if auditHash(record) != record.Hash {
			s.markRecovery(ctx, state, ReasonPersistedStateInvalid)
			return
		}
		if err := s.audit.RecordAuthorityAudit(ctx, record); err != nil {
			s.status = statusFromLedger(*state)
			s.status.Readiness, s.status.Reason = ReadinessDegraded, ReasonAuditUnavailable
			return
		}
		expected := state.Revision
		state.Revision++
		state.AuditOutbox = append([]AuditRecord(nil), state.AuditOutbox[1:]...)
		if err := s.store.Save(ctx, expected, *state); err != nil {
			s.setUnavailable(ReasonStoreUnavailable)
			return
		}
		s.status = statusFromLedger(*state)
	}
}

func (s *Service) markRecovery(ctx context.Context, state *Ledger, reason string) {
	expected := state.Revision
	state.Revision++
	state.Phase, state.Readiness, state.Reason = PhaseRecoveryRequired, ReadinessRecoveryRequired, reason
	if len(state.Operations) > 0 {
		state.Operations[0].Phase = PhaseRecoveryRequired
	}
	_ = s.store.Save(ctx, expected, *state)
	s.setRecovery(statusFromLedger(*state), reason)
}

func (s *Service) signerBinding(ctx context.Context) (string, ed25519.PublicKey, error) {
	principal, err := s.signer.Principal(ctx)
	if err != nil {
		return "", nil, ErrUnavailable
	}
	publicKey, err := s.signer.PublicKey(ctx)
	if err != nil || len(publicKey) != ed25519.PublicKeySize {
		return "", nil, ErrUnavailable
	}
	derived, err := identityprincipal.FromEd25519PublicKey(publicKey)
	if err != nil || derived.String() != principal {
		return "", nil, ErrRecoveryRequired
	}
	return principal, append(ed25519.PublicKey(nil), publicKey...), nil
}

func (s *Service) randomID(prefix string, size int) (string, error) {
	raw := make([]byte, size)
	if _, err := io.ReadFull(s.random, raw); err != nil {
		return "", err
	}
	allZero := true
	for _, value := range raw {
		allZero = allZero && value == 0
	}
	if allZero {
		return "", ErrUnavailable
	}
	return prefix + hex.EncodeToString(raw), nil
}

func validateCreateCommand(command Command) error {
	if command.Actor == "" || command.Actor != command.Effective ||
		command.Action != ActionCreate ||
		command.ResourceKind != ResourceKindAuthorityInstance ||
		command.ResourceID != PrimaryAuthorityInstance {
		return ErrPermissionDenied
	}
	return nil
}

func auditHash(record AuditRecord) string {
	copy := record
	copy.Hash = ""
	raw, _ := json.Marshal(copy)
	sum := sha256.Sum256(append([]byte("ardents:realm-authority-audit:v1\x00"), raw...))
	return "aa1_" + hex.EncodeToString(sum[:])
}

func statusFromLedger(state Ledger) Status {
	pending := uint32(0)
	for _, operation := range state.Operations {
		if operation.Phase != PhaseReady {
			pending++
		}
	}
	currentGeneration := uint32(0)
	for _, channel := range state.Channels {
		if channel.CurrentGeneration > currentGeneration {
			currentGeneration = channel.CurrentGeneration
		}
	}
	return Status{
		Version: ContractVersion, SchemaVersion: SchemaVersion,
		RealmID: state.RealmID, RealmClass: state.RealmClass,
		AuthorityEpoch: state.AuthorityEpoch, AuthoritySequence: state.AuthoritySequence,
		CheckpointDigest: state.Checkpoint.Digest, Phase: state.Phase,
		Readiness: state.Readiness, Reason: state.Reason,
		MemberCount: uint32(len(state.Members)), ChannelCount: uint32(len(state.Channels)),
		PendingOperationCount: pending, AuditOutboxDepth: uint32(len(state.AuditOutbox)),
		CurrentGeneration: currentGeneration, OperationDeadline: state.Operations[0].Deadline,
	}
}

func (s *Service) setUnavailable(reason string) {
	s.status.Readiness, s.status.Reason = ReadinessUnavailable, reason
}

func (s *Service) setRecovery(base Status, reason string) {
	if base.Version == 0 {
		base.Version, base.SchemaVersion = ContractVersion, SchemaVersion
	}
	base.Phase, base.Readiness, base.Reason = PhaseRecoveryRequired, ReadinessRecoveryRequired, reason
	s.status = base
}

func (s *Service) mutationFence() error {
	if s.recoveryOnly || s.migrationPending || s.transitionPending ||
		s.status.Readiness == ReadinessRecoveryRequired {
		return ErrRecoveryRequired
	}
	return nil
}

func (s *Service) continuationMutationFence() error {
	if s.recoveryOnly || s.status.Readiness == ReadinessRecoveryRequired {
		return ErrRecoveryRequired
	}
	return nil
}

func (s *Service) applyRecoveryOnlyStatus() {
	if !s.recoveryOnly || s.status.RealmID == "" ||
		s.status.Readiness != ReadinessReady {
		return
	}
	s.status.Phase = PhaseRecoveryOnly
	s.status.Readiness = ReadinessDegraded
	s.status.Reason = ReasonRestoreVerificationRequired
}

func (s *Service) refreshMigrationStatus(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.store == nil || s.status.Readiness == ReadinessRecoveryRequired {
		return
	}
	state, found, err := s.store.Load(ctx)
	if err != nil || !found || validateLedger(state) != nil {
		return
	}
	s.applyMigrationStatus(state)
	s.applyTransitionStatus(state)
}

func (s *Service) applyMigrationStatus(state Ledger) {
	s.migrationPending = migrationPending(state.Migration)
	if !s.migrationPending || s.status.Readiness != ReadinessReady {
		return
	}
	s.status.Phase = PhaseMigrationRotationRequired
	s.status.Readiness = ReadinessDegraded
	s.status.Reason = ReasonMigrationRotationRequired
}

func (s *Service) applyTransitionStatus(state Ledger) {
	s.transitionPending = authorityTransitionPending(state.Transition)
	if !s.transitionPending || s.status.Readiness != ReadinessReady {
		return
	}
	s.status.Phase = PhaseAuthorityTransitionRotationRequired
	s.status.Readiness = ReadinessDegraded
	s.status.Reason = ReasonAuthorityTransitionRotationRequired
}
