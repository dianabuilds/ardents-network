package replacement

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"

	"github.com/dianabuilds/ardents-network/internal/release"
)

const maximumProgramBytes = 64 << 20

// State is the bounded result of comparing one executable to the committed
// successor record. Unbound is permitted only for the independently pinned
// first enrollment path; a replacement caller must not treat it as an update
// authorization.
type State string

const (
	StateUnbound  State = "unbound"
	StatePrepared State = "prepared"
	StateCurrent  State = "current"
	StateMismatch State = "mismatch"
)

// Record is the non-secret, durable binding produced after Release has
// authenticated exact program bytes. It identifies no Authority material and
// cannot itself authorize an arbitrary candidate.
type Record struct {
	TargetPath     string
	Length         int64
	Digest         [sha256.Size]byte
	Platform       string
	Architecture   string
	Environment    string
	Network        string
	ReleaseID      string
	ReleaseVersion int64
}

// Request prepares a release-authorized binding for Artifact. StateRoot is an
// Endpoint-owned state directory, never the program directory, release floors,
// cache, runtime directory, or Authority Vault.
type Request struct {
	StateRoot     string
	Artifact      []byte
	Authorization release.Authorization
	decision      release.Decision
}

// Running is the bounded observation of the program currently at ProgramPath.
type Running struct {
	State  State
	Record Record
}

// Prepare writes the exact candidate identity only when a Release-issued
// opaque authorization authenticates Artifact. A foreground replacement owner
// must atomically activate those bytes, run VerifyPreparedRunning from the
// candidate, and then call CommitPrepared before the ordinary unit may accept
// the successor. It is Ubuntu-only: other platforms return an explicit
// unsupported error and make no filesystem change.
func Prepare(ctx context.Context, request Request) (Record, error) {
	if ctx == nil || ctx.Err() != nil {
		return Record{}, errors.New("endpoint replacement context is unavailable")
	}
	if runtime.GOOS != "linux" {
		return Record{}, errors.New("endpoint replacement is available only on Linux")
	}
	record, err := preparedRecord(request)
	if err != nil {
		return Record{}, err
	}
	store, err := openStore(request.StateRoot, true)
	if err != nil {
		return Record{}, err
	}
	defer store.close()
	if err := store.prepare(record); err != nil {
		return Record{}, err
	}
	return record, nil
}

func preparedRecord(request Request) (Record, error) {
	decision, ok := authorizedDecision(request)
	if !ok {
		return Record{}, errors.New("endpoint replacement lacks an accepted Release authorization")
	}
	if !acceptableDecision(decision) {
		return Record{}, fmt.Errorf("endpoint replacement authorization is not eligible for activation: outcome=%s build=%s protocol=%s", decision.Outcome, decision.BuildSafety, decision.Protocol)
	}
	digest := sha256.Sum256(request.Artifact)
	if int64(len(request.Artifact)) != decision.Length || len(decision.Digest) != sha256.Size ||
		!equalDigest(digest, decision.Digest) {
		return Record{}, errors.New("endpoint replacement artifact does not match the accepted Release artifact")
	}
	return recordFromDecision(decision, digest), nil
}

// CommitPrepared promotes a candidate already verified by
// VerifyPreparedRunning. It rechecks ProgramPath's exact bytes before making
// them the normal-start successor. A crash after current publication is safe:
// current remains authoritative and later opens discard the duplicate prepared
// record only when it is byte-identical.
func CommitPrepared(stateRoot, programPath string) (Record, error) {
	if runtime.GOOS != "linux" {
		return Record{}, errors.New("endpoint replacement is available only on Linux")
	}
	store, err := openStore(stateRoot, false)
	if err != nil {
		return Record{}, err
	}
	defer store.close()
	record, err := store.prepared()
	if err != nil {
		return Record{}, err
	}
	program, err := readProgram(programPath)
	if err != nil {
		return Record{}, err
	}
	digest := sha256.Sum256(program)
	if int64(len(program)) != record.Length || digest != record.Digest {
		return Record{}, errors.New("endpoint program does not match the prepared replacement")
	}
	if err := store.commitPrepared(record); err != nil {
		return Record{}, err
	}
	return record, nil
}

// VerifyRunning compares ProgramPath with the one committed successor record.
// It does not create, repair, or delete state. An absent StateRoot yields
// StateUnbound so the caller can run the separate first-enrollment proof.
func VerifyRunning(stateRoot, programPath string) (Running, error) {
	store, err := openReadStore(stateRoot)
	if errors.Is(err, errStoreAbsent) {
		return Running{State: StateUnbound}, nil
	}
	if err != nil {
		return Running{}, err
	}
	defer store.close()
	record, err := store.current()
	if err != nil {
		return Running{}, err
	}
	program, err := readProgram(programPath)
	if err != nil {
		return Running{}, err
	}
	digest := sha256.Sum256(program)
	if int64(len(program)) != record.Length || digest != record.Digest {
		return Running{State: StateMismatch, Record: record}, nil
	}
	return Running{State: StateCurrent, Record: record}, nil
}

// VerifyPreparedRunning is the exact no-network candidate-side check used
// before CommitPrepared. It never accepts a committed predecessor or an
// arbitrary program that merely shares the same state root.
func VerifyPreparedRunning(stateRoot, programPath string) (Running, error) {
	store, err := openReadStore(stateRoot)
	if err != nil {
		return Running{}, err
	}
	defer store.close()
	record, err := store.prepared()
	if err != nil {
		return Running{}, err
	}
	program, err := readProgram(programPath)
	if err != nil {
		return Running{}, err
	}
	digest := sha256.Sum256(program)
	if int64(len(program)) != record.Length || digest != record.Digest {
		return Running{State: StateMismatch, Record: record}, nil
	}
	return Running{State: StatePrepared, Record: record}, nil
}

// RollbackProgramPath resolves the owner-private retained predecessor path.
// It does not establish that recovery is currently permitted; callers must
// use VerifyRollbackProgram before invoking it as a recovery executable.
func RollbackProgramPath(stateRoot string) (string, error) {
	if stateRoot == "" {
		return "", errors.New("endpoint replacement state root is required")
	}
	root, err := filepath.Abs(stateRoot)
	if err != nil {
		return "", fmt.Errorf("resolve Endpoint replacement state root: %w", err)
	}
	return filepath.Join(root, rollbackName), nil
}

// VerifyRollbackProgram proves that ProgramPath is the exact retained
// predecessor for a durable failed candidate self-test. It is the recovery
// bootstrap check: a broken activated candidate cannot invoke a command from
// the normal program path, so only this owner-private retained executable may
// run the explicit rollback transaction. It never grants a rollback; Release
// must still issue a fresh authorization for the predecessor bytes. Its return
// value is the journal-bound normal program path to restore; callers must not
// accept a user-supplied rollback target.
func VerifyRollbackProgram(stateRoot, programPath string) (string, error) {
	if runtime.GOOS != "linux" {
		return "", errors.New("endpoint replacement is available only on Linux")
	}
	retainedPath, err := RollbackProgramPath(stateRoot)
	if err != nil {
		return "", err
	}
	providedPath, err := filepath.Abs(programPath)
	if err != nil {
		return "", fmt.Errorf("resolve Endpoint rollback program: %w", err)
	}
	if providedPath != retainedPath {
		return "", errors.New("endpoint rollback must run the retained recovery program")
	}
	store, err := openReadStore(stateRoot)
	if err != nil {
		return "", err
	}
	defer store.close()
	journalRecord, err := store.readJournal()
	if err != nil {
		return "", errors.New("endpoint rollback requires a retained failed self-test")
	}
	if journalRecord.phase != "self-test-failed" {
		return "", errors.New("endpoint rollback is not pending after a failed candidate self-test")
	}
	program, err := readProgram(retainedPath)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(program)
	if digest != journalRecord.predecessor {
		return "", errors.New("endpoint rollback recovery program does not match retained predecessor")
	}
	return journalRecord.programPath, nil
}

func authorizedDecision(request Request) (release.Decision, bool) {
	if decision, ok := request.Authorization.AcceptedDecision(); ok {
		return decision, true
	}
	// The unexported field is a package-private behavior-test seam. Production
	// callers cannot construct it and therefore cannot bypass Release.
	if request.decision.Outcome == release.OutcomeReleaseAccepted {
		return request.decision, true
	}
	return release.Decision{}, false
}

func acceptableDecision(decision release.Decision) bool {
	identity := decision.Length > 0 && decision.Length <= maximumProgramBytes &&
		len(decision.Digest) == sha256.Size && decision.Path != "" &&
		decision.Platform != "" && decision.Architecture != "" &&
		decision.Environment != "" && decision.Network != "" &&
		decision.ReleaseIdentity != "" && decision.ReleaseVersion > 0
	if decision.Outcome == release.OutcomeNoUpdate {
		// NoUpdate is a previously authenticated target that did not advance a
		// floor. It can establish the first local program binding, but Replace
		// separately rejects it as a candidate activation authorization.
		return identity
	}
	return decision.Outcome == release.OutcomeReleaseAccepted &&
		decision.BuildSafety == release.OutcomeReleaseAccepted &&
		decision.Protocol == release.OutcomeReleaseAccepted && identity
}

func recordFromDecision(decision release.Decision, digest [sha256.Size]byte) Record {
	return Record{TargetPath: decision.Path, Length: decision.Length, Digest: digest,
		Platform: decision.Platform, Architecture: decision.Architecture,
		Environment: decision.Environment, Network: decision.Network,
		ReleaseID: decision.ReleaseIdentity, ReleaseVersion: decision.ReleaseVersion}
}

func equalDigest(actual [sha256.Size]byte, expected []byte) bool {
	var value [sha256.Size]byte
	copy(value[:], expected)
	return actual == value
}
