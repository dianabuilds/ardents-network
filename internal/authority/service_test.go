package authority

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"errors"
	"testing"
	"time"

	identityprincipal "ardents/internal/identity/principal"

	"github.com/stretchr/testify/require"
)

func TestCreateRealmAuthorityIsDurableIdempotentAndRedacted(t *testing.T) {
	ctx := context.Background()
	fixture := newServiceFixture(t)

	result, err := fixture.service.CreateOrReopen(ctx, fixture.createCommand(), CreateRequest{
		Version: ContractVersion, RequestID: "request-001", RealmClass: RealmClassProduction,
	})
	require.NoError(t, err)
	require.Equal(t, uint64(1), result.AuthorityEpoch)
	require.Equal(t, uint64(1), result.AuthoritySequence)
	require.Equal(t, PhaseReady, result.Phase)
	require.True(t, ValidRealmID(result.RealmID))
	require.NotEmpty(t, result.OperationID)
	require.NotEmpty(t, result.CheckpointDigest)

	replayed, err := fixture.service.CreateOrReopen(ctx, fixture.createCommand(), CreateRequest{
		Version: ContractVersion, RequestID: "request-001", RealmClass: RealmClassProduction,
	})
	require.NoError(t, err)
	require.Equal(t, result, replayed)
	require.Len(t, fixture.store.state.AuditOutbox, 1)
	require.Len(t, fixture.store.state.Operations, 1)

	status, err := fixture.service.Inspect(ctx, fixture.inspectCommand(result.RealmID), InspectRequest{
		Version: ContractVersion, RealmID: result.RealmID,
	})
	require.NoError(t, err)
	require.Equal(t, result.RealmID, status.RealmID)
	require.Equal(t, PhaseReady, status.Phase)
	require.Equal(t, ReadinessReady, status.Readiness)
	require.Zero(t, status.MemberCount)
	require.Zero(t, status.ChannelCount)
	require.Zero(t, status.PendingOperationCount)
	require.Zero(t, status.CurrentGeneration)
	require.Equal(t, time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC), status.OperationDeadline)
	require.NotContains(t, status.String(), fixture.signer.principal)
	require.NotContains(t, status.String(), string(fixture.signer.private))
	require.NotContains(t, status.String(), "public_key")
}

func TestCreateRealmAuthorityRejectsConflictingReuseAndSecondGenesis(t *testing.T) {
	ctx := context.Background()
	fixture := newServiceFixture(t)
	_, err := fixture.service.CreateOrReopen(ctx, fixture.createCommand(), CreateRequest{
		Version: ContractVersion, RequestID: "request-001", RealmClass: RealmClassProduction,
	})
	require.NoError(t, err)

	_, err = fixture.service.CreateOrReopen(ctx, fixture.createCommand(), CreateRequest{
		Version: ContractVersion, RequestID: "request-001", RealmClass: "test",
	})
	require.ErrorIs(t, err, ErrConflict)
	_, err = fixture.service.CreateOrReopen(ctx, fixture.createCommand(), CreateRequest{
		Version: ContractVersion, RequestID: "request-002", RealmClass: RealmClassProduction,
	})
	require.ErrorIs(t, err, ErrConflict)
	require.Len(t, fixture.store.state.AuditOutbox, 1)
	require.Len(t, fixture.store.state.AuditLog, 1)
}

func TestGenesisCrashAfterLedgerCommitResumesTheSameOperation(t *testing.T) {
	ctx := context.Background()
	fixture := newServiceFixture(t)
	fixture.service.crash = func(boundary CrashBoundary) error {
		if boundary == CrashAfterLedgerCommit {
			return errors.New("injected crash")
		}
		return nil
	}
	_, err := fixture.service.CreateOrReopen(ctx, fixture.createCommand(), CreateRequest{
		Version: ContractVersion, RequestID: "request-001", RealmClass: RealmClassProduction,
	})
	require.ErrorIs(t, err, ErrUnavailable)
	operationID := fixture.store.state.Operations[0].ID
	auditID := fixture.store.state.AuditOutbox[0].ID
	require.False(t, fixture.repository.found)

	restarted := fixture.restart(t)
	result, err := restarted.CreateOrReopen(ctx, fixture.createCommand(), CreateRequest{
		Version: ContractVersion, RequestID: "request-001", RealmClass: RealmClassProduction,
	})
	require.NoError(t, err)
	require.Equal(t, operationID, result.OperationID)
	require.Equal(t, auditID, fixture.store.state.AuditOutbox[0].ID)
	require.Len(t, fixture.store.state.Operations, 1)
	require.True(t, fixture.repository.found)
}

func TestGenesisCrashAfterCheckpointCreateResumesWithoutOverwrite(t *testing.T) {
	ctx := context.Background()
	fixture := newServiceFixture(t)
	fixture.service.crash = func(boundary CrashBoundary) error {
		if boundary == CrashAfterCheckpointCreate {
			return errors.New("injected crash")
		}
		return nil
	}
	_, err := fixture.service.CreateOrReopen(ctx, fixture.createCommand(), CreateRequest{
		Version: ContractVersion, RequestID: "request-001", RealmClass: RealmClassProduction,
	})
	require.ErrorIs(t, err, ErrUnavailable)
	require.True(t, fixture.repository.found)
	created := fixture.repository.createCalls

	restarted := fixture.restart(t)
	result, err := restarted.CreateOrReopen(ctx, fixture.createCommand(), CreateRequest{
		Version: ContractVersion, RequestID: "request-001", RealmClass: RealmClassProduction,
	})
	require.NoError(t, err)
	require.Equal(t, PhaseReady, result.Phase)
	require.Equal(t, created, fixture.repository.createCalls)
}

func TestAuditOutboxRetainsOnDeliveryFailureAndDrainsAfterRestart(t *testing.T) {
	ctx := context.Background()
	fixture := newServiceFixture(t)
	failing := &auditSinkFixture{err: errors.New("audit unavailable")}
	fixture.service.audit = failing
	_, err := fixture.service.CreateOrReopen(ctx, fixture.createCommand(), CreateRequest{
		Version: ContractVersion, RequestID: "request-001", RealmClass: RealmClassProduction,
	})
	require.NoError(t, err)
	require.Len(t, fixture.store.state.AuditOutbox, 1)
	require.Empty(t, failing.records)
	require.Equal(t, ReadinessDegraded, fixture.service.Readiness().Readiness)
	require.Equal(t, ReasonAuditUnavailable, fixture.service.Readiness().Reason)

	working := &auditSinkFixture{}
	restarted := New(Config{
		Store: fixture.store, Signer: fixture.signer, Repository: fixture.repository,
		Random: bytes.NewReader(fixture.randomSeed), Clock: fixture.clock,
		Policy: allowPolicy{}, Audit: working,
	})
	require.Equal(t, ReadinessReady, restarted.Readiness().Readiness)
	require.Len(t, working.records, 1)
	require.Empty(t, fixture.store.state.AuditOutbox)
	require.Len(t, fixture.store.state.AuditLog, 1)
	require.Equal(t, working.records[0], fixture.store.state.AuditLog[0])
	require.Equal(t, fixture.createCommand().Actor, working.records[0].Actor)
	require.Equal(t, fixture.createCommand().Effective, working.records[0].Effective)
}

func TestGenesisFailsClosedAndResumesAcrossRandomPolicyRepositoryAndSignerFailures(t *testing.T) {
	ctx := context.Background()

	t.Run("random unavailable leaves no state", func(t *testing.T) {
		fixture := newServiceFixture(t)
		fixture.service.random = bytes.NewReader([]byte{0x01})
		_, err := fixture.service.CreateOrReopen(ctx, fixture.createCommand(), CreateRequest{
			Version: ContractVersion, RequestID: "request-001", RealmClass: RealmClassProduction,
		})
		require.ErrorIs(t, err, ErrUnavailable)
		require.False(t, fixture.store.found)
	})

	t.Run("product policy denies before mutation", func(t *testing.T) {
		fixture := newServiceFixture(t)
		fixture.service.policy = denyPolicy{}
		_, err := fixture.service.CreateOrReopen(ctx, fixture.createCommand(), CreateRequest{
			Version: ContractVersion, RequestID: "request-001", RealmClass: RealmClassProduction,
		})
		require.ErrorIs(t, err, ErrPermissionDenied)
		require.False(t, fixture.store.found)
	})

	t.Run("product policy is re-evaluated before idempotent reopen", func(t *testing.T) {
		fixture := newServiceFixture(t)
		request := CreateRequest{
			Version: ContractVersion, RequestID: "request-001",
			RealmClass: RealmClassProduction,
		}
		_, err := fixture.service.CreateOrReopen(ctx, fixture.createCommand(), request)
		require.NoError(t, err)
		revision := fixture.store.state.Revision
		fixture.service.policy = denyPolicy{}
		_, err = fixture.service.CreateOrReopen(ctx, fixture.createCommand(), request)
		require.ErrorIs(t, err, ErrPermissionDenied)
		require.Equal(t, revision, fixture.store.state.Revision)
	})

	t.Run("repository outage retains recoverable ledger", func(t *testing.T) {
		fixture := newServiceFixture(t)
		fixture.repository.err = errors.New("repository unavailable")
		_, err := fixture.service.CreateOrReopen(ctx, fixture.createCommand(), CreateRequest{
			Version: ContractVersion, RequestID: "request-001", RealmClass: RealmClassProduction,
		})
		require.ErrorIs(t, err, ErrUnavailable)
		require.True(t, fixture.store.found)
		require.Equal(t, PhaseCheckpointing, fixture.store.state.Phase)
		operationID := fixture.store.state.Operations[0].ID

		fixture.repository.err = nil
		restarted := fixture.restart(t)
		result, err := restarted.CreateOrReopen(ctx, fixture.createCommand(), CreateRequest{
			Version: ContractVersion, RequestID: "request-001", RealmClass: RealmClassProduction,
		})
		require.NoError(t, err)
		require.Equal(t, operationID, result.OperationID)
		require.Equal(t, PhaseReady, result.Phase)
	})

	t.Run("store outage leaves no replacement state", func(t *testing.T) {
		fixture := newServiceFixture(t)
		fixture.store.err = errors.New("store unavailable")
		_, err := fixture.service.CreateOrReopen(ctx, fixture.createCommand(), CreateRequest{
			Version: ContractVersion, RequestID: "request-001", RealmClass: RealmClassProduction,
		})
		require.ErrorIs(t, err, ErrUnavailable)
		require.False(t, fixture.store.found)
		require.Equal(t, ReasonStoreUnavailable, fixture.service.Readiness().Reason)
	})

	t.Run("sign failure leaves no authority ledger", func(t *testing.T) {
		fixture := newServiceFixture(t)
		fixture.service.signer = failingSigner{testSigner: fixture.signer}
		_, err := fixture.service.CreateOrReopen(ctx, fixture.createCommand(), CreateRequest{
			Version: ContractVersion, RequestID: "request-001", RealmClass: RealmClassProduction,
		})
		require.ErrorIs(t, err, ErrUnavailable)
		require.False(t, fixture.store.found)
		require.Equal(t, ReasonSignerUnavailable, fixture.service.Readiness().Reason)
	})

	t.Run("invalid injected clock leaves no authority ledger", func(t *testing.T) {
		fixture := newServiceFixture(t)
		fixture.service.clock = func() time.Time { return time.Time{} }
		_, err := fixture.service.CreateOrReopen(ctx, fixture.createCommand(), CreateRequest{
			Version: ContractVersion, RequestID: "request-001", RealmClass: RealmClassProduction,
		})
		require.ErrorIs(t, err, ErrUnavailable)
		require.False(t, fixture.store.found)
	})

	t.Run("missing signer is visible before genesis", func(t *testing.T) {
		service := New(Config{
			Store: &memoryStore{}, Repository: &memoryRepository{}, Policy: allowPolicy{},
		})
		require.Equal(t, ReadinessUnavailable, service.Readiness().Readiness)
		require.Equal(t, ReasonSignerUnavailable, service.Readiness().Reason)
	})
}

func TestPersistedBoundsAndAuditIntegrityFailClosed(t *testing.T) {
	fixture := newServiceFixture(t)
	_, err := fixture.service.CreateOrReopen(context.Background(), fixture.createCommand(), CreateRequest{
		Version: ContractVersion, RequestID: "request-001", RealmClass: RealmClassProduction,
	})
	require.NoError(t, err)

	oversized := cloneLedger(fixture.store.state)
	oversized.Channels = make([]ChannelRecord, MaxActiveChannels+1)
	require.ErrorIs(t, validateLedger(oversized), ErrCorruptState)

	tampered := cloneLedger(fixture.store.state)
	tampered.AuditOutbox[0].Actor = "different"
	require.ErrorIs(t, validateLedger(tampered), ErrCorruptState)

	tampered = cloneLedger(fixture.store.state)
	tampered.AuditLog[0].Actor = "different"
	require.ErrorIs(t, validateLedger(tampered), ErrCorruptState)

	oversizedAudit := cloneLedger(fixture.store.state)
	oversizedAudit.AuditLog = make([]AuditRecord, MaxAuditRecords+1)
	require.ErrorIs(t, validateLedger(oversizedAudit), ErrCorruptState)
}

func TestRestartFailsClosedOnSignerOrCheckpointMismatch(t *testing.T) {
	ctx := context.Background()
	fixture := newServiceFixture(t)
	result, err := fixture.service.CreateOrReopen(ctx, fixture.createCommand(), CreateRequest{
		Version: ContractVersion, RequestID: "request-001", RealmClass: RealmClassProduction,
	})
	require.NoError(t, err)

	wrongSigner := newTestSigner(t, 0x42)
	wrong := New(Config{
		Store: fixture.store, Signer: wrongSigner, Repository: fixture.repository,
		Random: bytes.NewReader(bytes.Repeat([]byte{0x31}, 256)), Clock: fixture.clock,
		Policy: allowPolicy{},
	})
	status := wrong.Readiness()
	require.Equal(t, ReadinessRecoveryRequired, status.Readiness)
	require.Equal(t, ReasonSignerMismatch, status.Reason)
	_, err = wrong.CreateOrReopen(ctx, fixture.createCommand(), CreateRequest{
		Version: ContractVersion, RequestID: "request-001", RealmClass: RealmClassProduction,
	})
	require.ErrorIs(t, err, ErrRecoveryRequired)

	fixture.repository.head.AuthoritySequence++
	forked := fixture.restart(t)
	status, err = forked.Inspect(ctx, fixture.inspectCommand(result.RealmID), InspectRequest{
		Version: ContractVersion, RealmID: result.RealmID,
	})
	require.NoError(t, err)
	require.Equal(t, ReadinessRecoveryRequired, status.Readiness)
	require.Equal(t, ReasonCheckpointMismatch, status.Reason)
	_, err = forked.CreateOrReopen(ctx, fixture.createCommand(), CreateRequest{
		Version: ContractVersion, RequestID: "request-001", RealmClass: RealmClassProduction,
	})
	require.ErrorIs(t, err, ErrRecoveryRequired)
}

func TestRestartRequiresRecoveryForCorruptLowerForkedOrMalformedTruth(t *testing.T) {
	ctx := context.Background()
	for name, corrupt := range map[string]func(*serviceFixture){
		"corrupt authority store": func(f *serviceFixture) {
			f.store.err = ErrCorruptState
		},
		"corrupt repository": func(f *serviceFixture) {
			f.repository.err = ErrCorruptState
		},
		"lower head": func(f *serviceFixture) {
			f.repository.head.AuthoritySequence = 0
		},
		"forked digest": func(f *serviceFixture) {
			f.repository.head.Digest = "ac1_" + string(bytes.Repeat([]byte{'0'}, 64))
		},
		"malformed signature": func(f *serviceFixture) {
			f.repository.head.Signature[0] ^= 0xff
		},
	} {
		t.Run(name, func(t *testing.T) {
			fixture := newServiceFixture(t)
			_, err := fixture.service.CreateOrReopen(ctx, fixture.createCommand(), CreateRequest{
				Version: ContractVersion, RequestID: "request-001", RealmClass: RealmClassProduction,
			})
			require.NoError(t, err)
			corrupt(fixture)
			restarted := fixture.restart(t)
			require.Equal(t, ReadinessRecoveryRequired, restarted.Readiness().Readiness)
			_, err = restarted.CreateOrReopen(ctx, fixture.createCommand(), CreateRequest{
				Version: ContractVersion, RequestID: "request-001", RealmClass: RealmClassProduction,
			})
			require.ErrorIs(t, err, ErrRecoveryRequired)
		})
	}
}

func TestCommandsAndRequestsAreExactBoundedAndDirect(t *testing.T) {
	fixture := newServiceFixture(t)
	ctx := context.Background()
	tests := []struct {
		name    string
		command Command
		request CreateRequest
		want    error
	}{
		{name: "unsupported version", command: fixture.createCommand(), request: CreateRequest{Version: 2, RequestID: "r", RealmClass: RealmClassProduction}, want: ErrUnsupportedVersion},
		{name: "empty idempotency identity", command: fixture.createCommand(), request: CreateRequest{Version: 1, RealmClass: RealmClassProduction}, want: ErrInvalidArgument},
		{name: "oversized idempotency identity", command: fixture.createCommand(), request: CreateRequest{Version: 1, RequestID: string(bytes.Repeat([]byte{'x'}, MaxRequestIDBytes+1)), RealmClass: RealmClassProduction}, want: ErrInvalidArgument},
		{name: "delegated", command: Command{Actor: "a", Effective: "b", Action: ActionCreate, ResourceKind: ResourceKindAuthorityInstance, ResourceID: PrimaryAuthorityInstance}, request: CreateRequest{Version: 1, RequestID: "r", RealmClass: RealmClassProduction}, want: ErrPermissionDenied},
		{name: "sibling action", command: Command{Actor: "a", Effective: "a", Action: ActionInspect, ResourceKind: ResourceKindAuthorityInstance, ResourceID: PrimaryAuthorityInstance}, request: CreateRequest{Version: 1, RequestID: "r", RealmClass: RealmClassProduction}, want: ErrPermissionDenied},
		{name: "parent resource", command: Command{Actor: "a", Effective: "a", Action: ActionCreate, ResourceKind: ResourceKindRealm, ResourceID: ""}, request: CreateRequest{Version: 1, RequestID: "r", RealmClass: RealmClassProduction}, want: ErrPermissionDenied},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := fixture.service.CreateOrReopen(ctx, test.command, test.request)
			require.ErrorIs(t, err, test.want)
			require.False(t, fixture.store.found)
		})
	}
}

type serviceFixture struct {
	store      *memoryStore
	repository *memoryRepository
	signer     *testSigner
	service    *Service
	clock      func() time.Time
	randomSeed []byte
}

func newServiceFixture(t *testing.T) *serviceFixture {
	t.Helper()
	signer := newTestSigner(t, 0x21)
	clock := func() time.Time { return time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC) }
	seed := bytes.Repeat([]byte{0x31}, 1024)
	f := &serviceFixture{
		store: &memoryStore{}, repository: &memoryRepository{}, signer: signer,
		clock: clock, randomSeed: seed,
	}
	f.service = New(Config{
		Store: f.store, Signer: signer, Repository: f.repository,
		Random: bytes.NewReader(seed), Clock: clock, Policy: allowPolicy{},
	})
	return f
}

func (f *serviceFixture) restart(t *testing.T) *Service {
	t.Helper()
	return New(Config{
		Store: f.store, Signer: f.signer, Repository: f.repository,
		Random: bytes.NewReader(f.randomSeed), Clock: f.clock, Policy: allowPolicy{},
	})
}

func (f *serviceFixture) createCommand() Command {
	return Command{
		Actor: "operator", Effective: "operator", Action: ActionCreate,
		ResourceKind: ResourceKindAuthorityInstance, ResourceID: PrimaryAuthorityInstance,
	}
}

func (f *serviceFixture) inspectCommand(realmID string) Command {
	return Command{
		Actor: "operator", Effective: "operator", Action: ActionInspect,
		ResourceKind: ResourceKindRealm, ResourceID: realmID,
	}
}

type allowPolicy struct{}

func (allowPolicy) AdmitRealmGenesis(context.Context, Command) error      { return nil }
func (allowPolicy) AdmitInitialGeneration(context.Context, Command) error { return nil }
func (allowPolicy) AdmitChannelRotation(context.Context, Command) error   { return nil }

type denyPolicy struct{}

func (denyPolicy) AdmitRealmGenesis(context.Context, Command) error {
	return errors.New("denied")
}
func (denyPolicy) AdmitInitialGeneration(context.Context, Command) error {
	return errors.New("denied")
}
func (denyPolicy) AdmitChannelRotation(context.Context, Command) error {
	return errors.New("denied")
}

type auditSinkFixture struct {
	records []AuditRecord
	err     error
}

func (s *auditSinkFixture) RecordAuthorityAudit(_ context.Context, record AuditRecord) error {
	if s.err != nil {
		return s.err
	}
	s.records = append(s.records, record)
	return nil
}

type testSigner struct {
	private   ed25519.PrivateKey
	principal string
}

func newTestSigner(t *testing.T, seed byte) *testSigner {
	t.Helper()
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{seed}, ed25519.SeedSize))
	principal, err := identityprincipal.FromEd25519PublicKey(private.Public().(ed25519.PublicKey))
	require.NoError(t, err)
	return &testSigner{private: private, principal: principal.String()}
}

func (s *testSigner) Principal(context.Context) (string, error) { return s.principal, nil }
func (s *testSigner) PublicKey(context.Context) (ed25519.PublicKey, error) {
	return append(ed25519.PublicKey(nil), s.private.Public().(ed25519.PublicKey)...), nil
}
func (s *testSigner) Sign(_ context.Context, message []byte) ([]byte, error) {
	return ed25519.Sign(s.private, message), nil
}

type failingSigner struct{ *testSigner }

func (f failingSigner) Sign(context.Context, []byte) ([]byte, error) {
	return nil, errors.New("signer unavailable")
}

type memoryStore struct {
	state Ledger
	found bool
	err   error
}

func (s *memoryStore) Load(context.Context) (Ledger, bool, error) {
	if s.err != nil {
		return Ledger{}, false, s.err
	}
	return cloneLedger(s.state), s.found, nil
}

func (s *memoryStore) Create(_ context.Context, state Ledger) error {
	if s.err != nil {
		return s.err
	}
	if s.found {
		return ErrConflict
	}
	s.state, s.found = cloneLedger(state), true
	return nil
}

func (s *memoryStore) Save(_ context.Context, expectedRevision uint64, state Ledger) error {
	if s.err != nil {
		return s.err
	}
	if !s.found || s.state.Revision != expectedRevision {
		return ErrConflict
	}
	s.state, s.found = cloneLedger(state), true
	return nil
}

type memoryRepository struct {
	head        SignedCheckpoint
	found       bool
	err         error
	createCalls int
}

func (r *memoryRepository) ReadHead(context.Context, string) (SignedCheckpoint, bool, error) {
	if r.err != nil {
		return SignedCheckpoint{}, false, r.err
	}
	return r.head, r.found, nil
}

func (r *memoryRepository) CreateIfAbsent(_ context.Context, next SignedCheckpoint) (SignedCheckpoint, error) {
	if r.err != nil {
		return SignedCheckpoint{}, r.err
	}
	r.createCalls++
	if r.found {
		return SignedCheckpoint{}, ErrConflict
	}
	r.head, r.found = next, true
	return next, nil
}

func (r *memoryRepository) CompareAndAppend(_ context.Context, realmID string, expected uint64, next SignedCheckpoint) (SignedCheckpoint, error) {
	if r.err != nil {
		return SignedCheckpoint{}, r.err
	}
	if !r.found || r.head.RealmID != realmID ||
		r.head.AuthoritySequence != expected ||
		next.AuthoritySequence != expected+1 ||
		next.PreviousDigest != r.head.Digest ||
		ValidateCheckpoint(next) != nil {
		return SignedCheckpoint{}, ErrConflict
	}
	r.head = next
	return next, nil
}
