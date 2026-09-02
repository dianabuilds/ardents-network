package replacement

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/dianabuilds/ardents-network/internal/release"
)

// UnitControl is the narrow foreground service-manager boundary. Its caller
// owns the exact Ubuntu user-unit name; replacement never accepts a command,
// shell fragment, or arbitrary unit identifier.
type UnitControl interface {
	Stop(context.Context) error
	Start(context.Context) error
}

// CandidateSelfTest runs the activated candidate without starting Endpoint
// network work. It receives only the replacement state root, not Authority or
// release-floor paths.
type CandidateSelfTest interface {
	Check(context.Context, string) error
}

// Operation composes one Release-authorized candidate with explicit unit and
// candidate-test adapters. ProgramPath must be the process's exact direct
// executable path; callers prove its current binding before invoking Replace.
type Operation struct {
	Request
	ProgramPath string
	Unit        UnitControl
	SelfTest    CandidateSelfTest
}

// Result is the bounded foreground replacement outcome. A non-nil error never
// implies rollback; the journal and Recover determine the safe next action.
type Result struct {
	State       string
	Current     Record
	Predecessor [sha256.Size]byte
}

type operationControl struct{ interruptAfter string }

var errOperationInterrupted = errors.New("endpoint replacement interrupted at test checkpoint")

// Replace is the Ubuntu-only foreground transaction: it verifies an opaque
// Release authorization, retains the current program, stages bytes beside the
// program, stops the exact caller-owned unit, atomically activates the staged
// candidate, runs its no-network self-test, commits it, and explicitly starts
// the unit. It never downloads, opens the Vault, or makes an automatic
// rollback decision.
func Replace(ctx context.Context, operation Operation) (Result, error) {
	return replace(ctx, operation, nil)
}

// Rollback is the explicit Ubuntu-only recovery transaction after an activated
// candidate failed its self-test. It requires a fresh opaque Release
// authorization for the retained predecessor; local predecessor bytes are not
// an authorization. It neither downloads nor infers a rollback from the
// journal. The caller owns the exact user-unit adapter and candidate-side
// no-network self-test, just as for Replace.
func Rollback(ctx context.Context, operation Operation) (Result, error) {
	if ctx == nil || ctx.Err() != nil {
		return Result{State: "invalid"}, errors.New("endpoint rollback context is unavailable")
	}
	if runtime.GOOS != "linux" {
		return Result{State: "unsupported"}, errors.New("endpoint replacement is available only on Linux")
	}
	if operation.ProgramPath == "" || operation.Unit == nil || operation.SelfTest == nil {
		return Result{State: "invalid"}, errors.New("endpoint rollback operation is incomplete")
	}
	predecessor, err := preparedRecord(operation.Request)
	if err != nil {
		return Result{State: "release-rejected"}, err
	}
	decision, authorized := authorizedDecision(operation.Request)
	if !authorized || decision.Outcome != release.OutcomeReleaseAccepted {
		return Result{State: "release-rejected"}, errors.New("endpoint rollback requires a newly accepted Release authorization")
	}
	store, err := openStore(operation.StateRoot, false)
	if err != nil {
		return Result{State: "invalid"}, err
	}
	defer store.close()
	current, err := store.current()
	if err != nil {
		return Result{State: "current-unbound"}, errors.New("endpoint rollback requires a committed predecessor record")
	}
	journalRecord, err := store.readJournal()
	if err != nil {
		return Result{State: "repair-required", Current: current}, errors.New("endpoint rollback requires a retained failed self-test")
	}
	if journalRecord.phase != "self-test-failed" || journalRecord.predecessor != current.Digest {
		return Result{State: "rollback-unavailable", Current: current}, errors.New("endpoint rollback is not pending after a failed candidate self-test")
	}
	activated, err := readProgram(operation.ProgramPath)
	if err != nil {
		return Result{State: "repair-required", Current: current, Predecessor: journalRecord.predecessor}, err
	}
	activatedDigest := sha256.Sum256(activated)
	if activatedDigest != journalRecord.candidate {
		return Result{State: "repair-required", Current: current, Predecessor: journalRecord.predecessor}, errors.New("endpoint rollback candidate program does not match retained journal")
	}
	retained, err := readProgram(filepath.Join(store.root, rollbackName))
	if err != nil {
		return Result{State: "repair-required", Current: current, Predecessor: journalRecord.predecessor}, err
	}
	retainedDigest := sha256.Sum256(retained)
	if retainedDigest != journalRecord.predecessor || retainedDigest != predecessor.Digest || predecessor.Length != int64(len(retained)) {
		return Result{State: "release-rejected", Current: current, Predecessor: journalRecord.predecessor}, errors.New("endpoint rollback authorization does not match the retained predecessor")
	}
	if err := store.replacePreparedForRollback(predecessor, journalRecord.candidate); err != nil {
		return Result{State: "repair-required", Current: current, Predecessor: journalRecord.predecessor}, err
	}
	staged, err := stageProgram(operation.ProgramPath, retained)
	if err != nil {
		return Result{State: "staging-failed", Current: current, Predecessor: journalRecord.candidate}, err
	}
	defer removeStaged(staged)
	if err := operation.Unit.Stop(ctx); err != nil {
		return Result{State: "stop-refused", Current: current, Predecessor: journalRecord.candidate}, err
	}
	if err := activateStaged(staged, operation.ProgramPath); err != nil {
		return Result{State: "activation-failed", Current: current, Predecessor: journalRecord.candidate}, err
	}
	if !sameProgramPath(operation.ProgramPath, journalRecord.programPath) {
		return Result{State: "repair-required", Current: current, Predecessor: journalRecord.predecessor}, errors.New("endpoint rollback target does not match retained journal")
	}
	rollbackJournal := journal{phase: "rollback-activated", programPath: journalRecord.programPath, predecessor: journalRecord.candidate, candidate: predecessor.Digest}
	if err := store.writeJournal(rollbackJournal); err != nil {
		return Result{State: "self-test-required", Current: predecessor, Predecessor: journalRecord.candidate}, err
	}
	if err := operation.SelfTest.Check(ctx, operation.StateRoot); err != nil {
		return Result{State: "repair-required", Current: predecessor, Predecessor: journalRecord.candidate}, err
	}
	if err := store.commitPrepared(predecessor); err != nil {
		return Result{State: "commit-failed", Current: predecessor, Predecessor: journalRecord.candidate}, err
	}
	if err := writeExecutableAtomic(store.root, rollbackName, activated); err != nil {
		return Result{State: "rollback-committed", Current: predecessor, Predecessor: journalRecord.candidate}, err
	}
	if err := store.writeJournal(journal{phase: "rollback-committed", programPath: journalRecord.programPath, predecessor: journalRecord.candidate, candidate: predecessor.Digest}); err != nil {
		return Result{State: "rollback-committed", Current: predecessor, Predecessor: journalRecord.candidate}, err
	}
	if err := operation.Unit.Start(ctx); err != nil {
		return Result{State: "rollback-committed-start-failed", Current: predecessor, Predecessor: journalRecord.candidate}, err
	}
	return Result{State: "rollback-committed-restart-permitted", Current: predecessor, Predecessor: journalRecord.candidate}, nil
}

// replaceWithInterruption is a package-private crash-boundary seam. It keeps
// all real file, journal, unit, and candidate-test operations but returns just
// after one durable checkpoint, leaving recovery-owned evidence intact.
func replaceWithInterruption(ctx context.Context, operation Operation, control operationControl) (Result, error) {
	return replace(ctx, operation, &control)
}

func replace(ctx context.Context, operation Operation, control *operationControl) (Result, error) {
	if ctx == nil || ctx.Err() != nil {
		return Result{State: "invalid"}, errors.New("endpoint replacement context is unavailable")
	}
	if runtime.GOOS != "linux" {
		return Result{State: "unsupported"}, errors.New("endpoint replacement is available only on Linux")
	}
	if operation.ProgramPath == "" || operation.Unit == nil || operation.SelfTest == nil {
		return Result{State: "invalid"}, errors.New("endpoint replacement operation is incomplete")
	}
	candidate, err := preparedRecord(operation.Request)
	if err != nil {
		return Result{State: "release-rejected"}, err
	}
	decision, authorized := authorizedDecision(operation.Request)
	if !authorized || decision.Outcome != release.OutcomeReleaseAccepted {
		return Result{State: "release-rejected"}, errors.New("endpoint replacement requires a newly accepted Release authorization")
	}
	store, err := openStore(operation.StateRoot, false)
	if err != nil {
		return Result{State: "invalid"}, err
	}
	defer store.close()
	current, err := store.current()
	if err != nil {
		return Result{State: "current-unbound"}, errors.New("endpoint replacement requires a committed current program")
	}
	predecessor, err := readProgram(operation.ProgramPath)
	if err != nil {
		return Result{State: "current-unavailable"}, err
	}
	predecessorDigest := sha256.Sum256(predecessor)
	if int64(len(predecessor)) != current.Length || predecessorDigest != current.Digest {
		return Result{State: "current-mismatch", Current: current}, errors.New("endpoint replacement current program does not match its committed record")
	}
	if err := store.retireCompletedRollback(current, operation.ProgramPath); err != nil {
		return Result{State: "rollback-retained", Current: current, Predecessor: predecessorDigest}, err
	}
	if err := store.prepare(candidate); err != nil {
		return Result{State: "replacement-pending", Current: current, Predecessor: predecessorDigest}, err
	}
	if err := store.writeJournal(journal{phase: "prepared", programPath: operation.ProgramPath, predecessor: predecessorDigest, candidate: candidate.Digest}); err != nil {
		return Result{State: "replacement-pending", Current: current, Predecessor: predecessorDigest}, err
	}
	if err := writeExecutableAtomic(store.root, rollbackName, predecessor); err != nil {
		return Result{State: "replacement-pending", Current: current, Predecessor: predecessorDigest}, err
	}
	if err := store.writeJournal(journal{phase: "rollback-retained", programPath: operation.ProgramPath, predecessor: predecessorDigest, candidate: candidate.Digest}); err != nil {
		return Result{State: "rollback-retained", Current: current, Predecessor: predecessorDigest}, err
	}
	if interrupted(control, "rollback-retained") {
		return Result{State: "rollback-retained", Current: current, Predecessor: predecessorDigest}, errOperationInterrupted
	}
	staged, err := stageProgram(operation.ProgramPath, operation.Artifact)
	if err != nil {
		return Result{State: "staging-failed", Current: current, Predecessor: predecessorDigest}, err
	}
	defer removeStaged(staged)
	if err := store.writeJournal(journal{phase: "staged", programPath: operation.ProgramPath, predecessor: predecessorDigest, candidate: candidate.Digest}); err != nil {
		return Result{State: "staged", Current: current, Predecessor: predecessorDigest}, err
	}
	if interrupted(control, "staged") {
		return Result{State: "staged", Current: current, Predecessor: predecessorDigest}, errOperationInterrupted
	}
	if err := operation.Unit.Stop(ctx); err != nil {
		return Result{State: "stop-refused", Current: current, Predecessor: predecessorDigest}, err
	}
	if err := activateStaged(staged, operation.ProgramPath); err != nil {
		return Result{State: "activation-failed", Current: current, Predecessor: predecessorDigest}, err
	}
	if err := store.writeJournal(journal{phase: "activated", programPath: operation.ProgramPath, predecessor: predecessorDigest, candidate: candidate.Digest}); err != nil {
		return Result{State: "self-test-required", Current: candidate, Predecessor: predecessorDigest}, err
	}
	if interrupted(control, "activated") {
		return Result{State: "self-test-required", Current: candidate, Predecessor: predecessorDigest}, errOperationInterrupted
	}
	if err := operation.SelfTest.Check(ctx, operation.StateRoot); err != nil {
		journalErr := store.writeJournal(journal{phase: "self-test-failed", programPath: operation.ProgramPath, predecessor: predecessorDigest, candidate: candidate.Digest})
		return Result{State: "rollback-authorization-required", Current: candidate, Predecessor: predecessorDigest}, errors.Join(err, journalErr)
	}
	if err := store.commitPrepared(candidate); err != nil {
		return Result{State: "commit-failed", Current: candidate, Predecessor: predecessorDigest}, err
	}
	if err := store.writeJournal(journal{phase: "committed", programPath: operation.ProgramPath, predecessor: predecessorDigest, candidate: candidate.Digest}); err != nil {
		return Result{State: "committed", Current: candidate, Predecessor: predecessorDigest}, err
	}
	if interrupted(control, "committed") {
		return Result{State: "committed", Current: candidate, Predecessor: predecessorDigest}, errOperationInterrupted
	}
	if err := operation.Unit.Start(ctx); err != nil {
		return Result{State: "committed-start-failed", Current: candidate, Predecessor: predecessorDigest}, err
	}
	return Result{State: "committed-restart-permitted", Current: candidate, Predecessor: predecessorDigest}, nil
}

func interrupted(control *operationControl, checkpoint string) bool {
	return control != nil && control.interruptAfter == checkpoint
}

// Recover only classifies retained interruption evidence. It never rolls back,
// removes bytes, starts a unit, or converts local predecessor bytes into a
// Release authorization.
func Recover(stateRoot, programPath string) (Result, error) {
	store, err := openReadStore(stateRoot)
	if err != nil {
		return Result{State: "invalid"}, err
	}
	defer store.close()
	journalRecord, err := store.readJournal()
	if errors.Is(err, os.ErrNotExist) {
		running, runningErr := VerifyRunning(stateRoot, programPath)
		if runningErr != nil {
			return Result{State: "invalid"}, runningErr
		}
		return Result{State: string(running.State), Current: running.Record}, nil
	}
	if err != nil {
		return Result{State: "repair-required"}, err
	}
	if !sameProgramPath(programPath, journalRecord.programPath) {
		return Result{State: "repair-required", Predecessor: journalRecord.predecessor}, errors.New("endpoint recovery program path does not match retained journal")
	}
	program, err := readProgram(programPath)
	if err != nil {
		return Result{State: "repair-required", Predecessor: journalRecord.predecessor}, err
	}
	digest := sha256.Sum256(program)
	if digest == journalRecord.candidate {
		if journalRecord.phase == "self-test-failed" {
			return Result{State: "rollback-authorization-required", Predecessor: journalRecord.predecessor}, nil
		}
		if journalRecord.phase == "rollback-activated" {
			return Result{State: "rollback-self-test-required", Predecessor: journalRecord.predecessor}, nil
		}
		if current, currentErr := store.current(); currentErr == nil && current.Digest == digest {
			if journalRecord.phase == "rollback-committed" {
				return Result{State: "rollback-committed-restart-permitted", Current: current, Predecessor: journalRecord.predecessor}, nil
			}
			return Result{State: "committed-restart-permitted", Current: current, Predecessor: journalRecord.predecessor}, nil
		}
		return Result{State: "self-test-required", Predecessor: journalRecord.predecessor}, nil
	}
	if digest == journalRecord.predecessor && (journalRecord.phase == "prepared" || journalRecord.phase == "rollback-retained" || journalRecord.phase == "staged") {
		return Result{State: "keep-current", Predecessor: journalRecord.predecessor}, nil
	}
	return Result{State: "repair-required", Predecessor: journalRecord.predecessor}, nil
}

type journal struct {
	phase                  string
	programPath            string
	predecessor, candidate [sha256.Size]byte
}

func (store *store) writeJournal(value journal) error {
	if !filepath.IsAbs(value.programPath) || strings.ContainsAny(value.programPath, "\r\n") {
		return errors.New("endpoint replacement journal program path is invalid")
	}
	return writeAtomic(store.root, journalName, []byte("schema=ardents-endpoint-replacement-journal-v1\nphase="+value.phase+"\nprogram_path="+value.programPath+"\npredecessor="+
		hex.EncodeToString(value.predecessor[:])+"\ncandidate="+hex.EncodeToString(value.candidate[:])+"\n"))
}

func (store *store) readJournal() (journal, error) {
	raw, err := readDirectFile(filepath.Join(store.root, journalName), maximumText)
	if err != nil {
		return journal{}, err
	}
	keys := []string{"schema", "phase", "program_path", "predecessor", "candidate"}
	lines := strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
	if len(raw) == 0 || raw[len(raw)-1] != '\n' || len(lines) != len(keys) {
		return journal{}, errors.New("endpoint replacement journal is not canonical")
	}
	values := make([]string, len(keys))
	for index, line := range lines {
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 || parts[0] != keys[index] || parts[1] == "" {
			return journal{}, errors.New("endpoint replacement journal is not canonical")
		}
		values[index] = parts[1]
	}
	if values[0] != "ardents-endpoint-replacement-journal-v1" ||
		(values[1] != "prepared" && values[1] != "rollback-retained" && values[1] != "staged" && values[1] != "activated" && values[1] != "self-test-failed" && values[1] != "committed" && values[1] != "rollback-activated" && values[1] != "rollback-committed") {
		return journal{}, errors.New("endpoint replacement journal is invalid")
	}
	if !filepath.IsAbs(values[2]) {
		return journal{}, errors.New("endpoint replacement journal program path is invalid")
	}
	predecessor, err := decodeDigest(values[3])
	if err != nil {
		return journal{}, err
	}
	candidate, err := decodeDigest(values[4])
	if err != nil {
		return journal{}, err
	}
	return journal{phase: values[1], programPath: values[2], predecessor: predecessor, candidate: candidate}, nil
}

func sameProgramPath(left, right string) bool {
	leftPath, leftErr := filepath.Abs(left)
	rightPath, rightErr := filepath.Abs(right)
	return leftErr == nil && rightErr == nil && leftPath == rightPath
}

func decodeDigest(value string) ([sha256.Size]byte, error) {
	raw, err := hex.DecodeString(value)
	if err != nil || len(raw) != sha256.Size || strings.ToLower(value) != value {
		return [sha256.Size]byte{}, errors.New("endpoint replacement digest is invalid")
	}
	var digest [sha256.Size]byte
	copy(digest[:], raw)
	return digest, nil
}

func stageProgram(programPath string, artifact []byte) (string, error) {
	info, err := os.Lstat(programPath)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return "", errors.New("endpoint replacement program is not a direct regular file")
	}
	directory := filepath.Dir(programPath)
	directoryInfo, err := os.Lstat(directory)
	if err != nil || !directoryInfo.IsDir() || directoryInfo.Mode()&os.ModeSymlink != 0 {
		return "", errors.New("endpoint replacement program directory is invalid")
	}
	var token [8]byte
	if _, err := rand.Read(token[:]); err != nil {
		return "", err
	}
	staged := filepath.Join(directory, "."+filepath.Base(programPath)+".ardents-"+hex.EncodeToString(token[:])+".candidate")
	file, err := os.OpenFile(staged, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o700)
	if err != nil {
		return "", err
	}
	if _, err = file.Write(artifact); err == nil {
		err = file.Sync()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(staged)
		return "", err
	}
	return staged, syncDirectory(directory)
}

func activateStaged(staged, programPath string) error {
	if err := os.Rename(staged, programPath); err != nil {
		return fmt.Errorf("atomically activate Endpoint replacement: %w", err)
	}
	return syncDirectory(filepath.Dir(programPath))
}

func removeStaged(path string) {
	if path == "" {
		return
	}
	info, err := os.Lstat(path)
	if err == nil && info.Mode().IsRegular() && info.Mode()&os.ModeSymlink == 0 {
		_ = os.Remove(path)
	}
}

func syncDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	err = directory.Sync()
	closeErr := directory.Close()
	return errors.Join(err, closeErr)
}
