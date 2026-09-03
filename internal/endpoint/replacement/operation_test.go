package replacement

import (
	"context"
	"crypto/sha256"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/release"
)

// stateRoot creates the owner-only replacement state directory, independent
// of the test process umask.
func replacementStateRoot(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "state", "replacement")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestReplaceActivatesOnlyAfterPreparedCandidateSelfTest(t *testing.T) {
	if err := requireLinux(); err != nil {
		t.Skip(err)
	}
	root := t.TempDir()
	program := filepath.Join(root, "ardents")
	stateRoot := replacementStateRoot(t)
	v1 := []byte("current program v1")
	v2 := []byte("candidate program v2")
	if err := os.WriteFile(program, v1, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := Prepare(context.Background(), Request{StateRoot: stateRoot, Artifact: v1, decision: replacementDecision(v1, 1)}); err != nil {
		t.Fatal(err)
	}
	if _, err := CommitPrepared(stateRoot, program); err != nil {
		t.Fatal(err)
	}
	unit := &replacementUnit{}
	result, err := Replace(context.Background(), Operation{Request: Request{StateRoot: stateRoot, Artifact: v2, decision: replacementDecision(v2, 2)},
		ProgramPath: program, Unit: unit, SelfTest: replacementSelfTest{program: program}})
	if err != nil || result.State != "committed-restart-permitted" || !unit.stopped || !unit.started {
		t.Fatalf("Replace() = %+v, %v; unit=%+v", result, err, unit)
	}
	if got, err := os.ReadFile(program); err != nil || string(got) != string(v2) {
		t.Fatalf("activated program = %q, %v", got, err)
	}
	running, err := VerifyRunning(stateRoot, program)
	if err != nil || running.State != StateCurrent || running.Record.ReleaseVersion != 2 {
		t.Fatalf("VerifyRunning() = %+v, %v", running, err)
	}
	recovery, err := Recover(stateRoot, program)
	if err != nil || recovery.State != "committed-restart-permitted" {
		t.Fatalf("Recover() = %+v, %v", recovery, err)
	}
}

func TestReplaceDoesNotRunCandidateBeforeSuccessfulSelfTestWhenActivationDirectorySyncFails(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux-only replacement behavior")
	}
	if os.Geteuid() == 0 {
		t.Fatal("invalid test environment: Endpoint replacement directory-sync regression requires a clean unprivileged Ubuntu account; root bypasses chmod(0300)")
	}
	root := t.TempDir()
	programDirectory := filepath.Join(root, "program")
	if err := os.Mkdir(programDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	program := filepath.Join(programDirectory, "ardents")
	stateRoot := replacementStateRoot(t)
	predecessor := []byte("current program v1")
	candidate := []byte("candidate program v2")
	nextCandidate := []byte("candidate program v3")
	if err := os.WriteFile(program, predecessor, 0o700); err != nil {
		t.Fatal(err)
	}
	predecessorDecision := replacementDecision(predecessor, 1)
	if _, err := Prepare(context.Background(), Request{StateRoot: stateRoot, Artifact: predecessor, decision: predecessorDecision}); err != nil {
		t.Fatal(err)
	}
	if _, err := CommitPrepared(stateRoot, program); err != nil {
		t.Fatal(err)
	}
	candidateDecision := replacementDecision(candidate, 2)
	successfulSelfTests := 0
	var startedProgramDigest [sha256.Size]byte
	invalidEnvironment := errors.New("invalid test environment: Endpoint replacement directory-sync regression requires a clean unprivileged Ubuntu account with no directory-read permission bypass")
	unit := &replacementUnit{
		onStop: func(context.Context) error {
			if err := os.Chmod(programDirectory, 0o300); err != nil {
				return err
			}
			directory, err := os.Open(programDirectory)
			if errors.Is(err, fs.ErrPermission) {
				return nil
			}
			if err != nil {
				return err
			}
			if closeErr := directory.Close(); closeErr != nil {
				return closeErr
			}
			return invalidEnvironment
		},
		onStart: func(context.Context) error {
			started, err := os.ReadFile(program)
			if err != nil {
				return err
			}
			startedProgramDigest = sha256.Sum256(started)
			return nil
		},
	}
	defer func() {
		if err := os.Chmod(programDirectory, 0o700); err != nil {
			t.Errorf("restore program directory mode: %v", err)
		}
	}()
	result, err := Replace(context.Background(), Operation{Request: Request{StateRoot: stateRoot, Artifact: candidate, decision: candidateDecision},
		ProgramPath: program, Unit: unit, SelfTest: replacementSelfTest{program: program, successes: &successfulSelfTests}})
	if restoreErr := os.Chmod(programDirectory, 0o700); restoreErr != nil {
		t.Fatal(restoreErr)
	}
	if errors.Is(err, invalidEnvironment) {
		t.Fatal(err)
	}
	if err == nil || result.State != "activation-failed" || !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("Replace() after activated-directory sync failure = %+v, %v", result, err)
	}
	prepared, err := VerifyPreparedRunning(stateRoot, program)
	if err != nil || prepared.State != StatePrepared {
		t.Fatalf("VerifyPreparedRunning() after activated-directory sync failure = %+v, %v", prepared, err)
	}
	activated, err := os.ReadFile(program)
	if err != nil {
		t.Fatal(err)
	}
	activatedDigest := sha256.Sum256(activated)
	if activatedDigest != prepared.Record.Digest {
		t.Fatalf("activated program digest = %x, prepared digest = %x", activatedDigest, prepared.Record.Digest)
	}
	running, err := VerifyRunning(stateRoot, program)
	if err != nil || running.State != StateMismatch {
		t.Fatalf("VerifyRunning() after activated-directory sync failure = %+v, %v", running, err)
	}
	recovery, err := Recover(stateRoot, program)
	if err != nil || recovery.State != "self-test-required" {
		t.Fatalf("Recover() after activated-directory sync failure = %+v, %v", recovery, err)
	}
	rollbackProgram, err := RollbackProgramPath(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	retained, err := os.ReadFile(rollbackProgram)
	if err != nil {
		t.Fatal(err)
	}
	retainedDigest := sha256.Sum256(retained)
	if running.Record.Digest != retainedDigest || recovery.Predecessor != retainedDigest || prepared.Record.Digest == retainedDigest {
		t.Fatalf("persisted predecessor/candidate digests = running=%x recovery=%x prepared=%x retained=%x", running.Record.Digest, recovery.Predecessor, prepared.Record.Digest, retainedDigest)
	}
	nextDecision := replacementDecision(nextCandidate, 3)
	nextResult, nextErr := Replace(context.Background(), Operation{Request: Request{StateRoot: stateRoot, Artifact: nextCandidate, decision: nextDecision},
		ProgramPath: program, Unit: &replacementUnit{}, SelfTest: replacementSelfTest{program: program}})
	if nextErr == nil || nextResult.State != "current-mismatch" {
		t.Fatalf("Replace() after activated-directory sync failure = %+v, %v", nextResult, nextErr)
	}
	afterNextRecovery, afterNextErr := Recover(stateRoot, program)
	if afterNextErr != nil || afterNextRecovery.State != "self-test-required" || afterNextRecovery.Predecessor != retainedDigest {
		t.Fatalf("Recover() after rejected next replacement = %+v, %v", afterNextRecovery, afterNextErr)
	}
	if unit.started && startedProgramDigest != prepared.Record.Digest {
		t.Fatalf("Unit.Start() observed digest = %x, prepared candidate digest = %x", startedProgramDigest, prepared.Record.Digest)
	}
	if unit.started && successfulSelfTests == 0 {
		t.Fatalf("Replace() called Unit.Start for visible candidate digest %x before a successful candidate self-test", startedProgramDigest)
	}
}

func TestReplaceRetainsCandidateForAuthorizedRollbackAfterSelfTestFailure(t *testing.T) {
	if err := requireLinux(); err != nil {
		t.Skip(err)
	}
	root := t.TempDir()
	program := filepath.Join(root, "ardents")
	stateRoot := replacementStateRoot(t)
	v1, v2 := []byte("current program v1"), []byte("candidate program v2")
	if err := os.WriteFile(program, v1, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := Prepare(context.Background(), Request{StateRoot: stateRoot, Artifact: v1, decision: replacementDecision(v1, 1)}); err != nil {
		t.Fatal(err)
	}
	if _, err := CommitPrepared(stateRoot, program); err != nil {
		t.Fatal(err)
	}
	unit := &replacementUnit{}
	result, err := Replace(context.Background(), Operation{Request: Request{StateRoot: stateRoot, Artifact: v2, decision: replacementDecision(v2, 2)},
		ProgramPath: program, Unit: unit, SelfTest: failingSelfTest{}})
	if err == nil || result.State != "rollback-authorization-required" || !unit.stopped || unit.started {
		t.Fatalf("Replace() = %+v, %v; unit=%+v", result, err, unit)
	}
	recovery, err := Recover(stateRoot, program)
	if err != nil || recovery.State != "rollback-authorization-required" {
		t.Fatalf("Recover() = %+v, %v", recovery, err)
	}
	recoveryProgram, err := RollbackProgramPath(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	if info, statErr := os.Stat(recoveryProgram); statErr != nil || info.Mode().Perm() != 0o700 {
		t.Fatalf("retained predecessor mode = %v, %v", info, statErr)
	}
	target, verifyErr := VerifyRollbackProgram(stateRoot, recoveryProgram)
	if verifyErr != nil || target != program {
		t.Fatalf("VerifyRollbackProgram() = %q, %v", target, verifyErr)
	}
	if _, err := VerifyRollbackProgram(stateRoot, program); err == nil {
		t.Fatal("VerifyRollbackProgram() accepted the activated candidate path")
	}
	if _, err := os.Stat(filepath.Join(stateRoot, rollbackName)); err != nil {
		t.Fatalf("retained predecessor: %v", err)
	}
	if _, err := Prepare(context.Background(), Request{StateRoot: stateRoot, Artifact: v2, decision: replacementDecision(v2, 2)}); err == nil {
		t.Fatal("Prepare() accepted a failed candidate's retained prepared record")
	}
	result, nextErr := Replace(context.Background(), Operation{Request: Request{StateRoot: stateRoot, Artifact: []byte("candidate program v3"),
		decision: replacementDecision([]byte("candidate program v3"), 3)}, ProgramPath: program, Unit: &replacementUnit{},
		SelfTest: replacementSelfTest{program: program}})
	if nextErr == nil || result.State != "current-mismatch" {
		t.Fatalf("Replace() after failed self-test = %+v, %v", result, nextErr)
	}
	recovery, recoveryErr := Recover(stateRoot, program)
	if recoveryErr != nil || recovery.State != "rollback-authorization-required" {
		t.Fatalf("Recover() after rejected next replacement = %+v, %v", recovery, recoveryErr)
	}
}

func TestRollbackRestoresRetainedPredecessorOnlyAfterFreshReleaseAuthorization(t *testing.T) {
	if err := requireLinux(); err != nil {
		t.Skip(err)
	}
	root := t.TempDir()
	program := filepath.Join(root, "ardents")
	stateRoot := replacementStateRoot(t)
	v1, v2 := []byte("current program v1"), []byte("candidate program v2")
	if err := os.WriteFile(program, v1, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := Prepare(context.Background(), Request{StateRoot: stateRoot, Artifact: v1, decision: replacementDecision(v1, 1)}); err != nil {
		t.Fatal(err)
	}
	if _, err := CommitPrepared(stateRoot, program); err != nil {
		t.Fatal(err)
	}
	if _, err := Replace(context.Background(), Operation{Request: Request{StateRoot: stateRoot, Artifact: v2,
		decision: replacementDecision(v2, 2)}, ProgramPath: program, Unit: &replacementUnit{}, SelfTest: failingSelfTest{}}); err == nil {
		t.Fatal("Replace() accepted a failed candidate self-test")
	}
	unit := &replacementUnit{}
	result, err := Rollback(context.Background(), Operation{Request: Request{StateRoot: stateRoot, Artifact: v1,
		decision: replacementDecision(v1, 3)}, ProgramPath: program, Unit: unit, SelfTest: replacementSelfTest{program: program}})
	if err != nil || result.State != "rollback-committed-restart-permitted" || !unit.stopped || !unit.started {
		t.Fatalf("Rollback() = %+v, %v; unit=%+v", result, err, unit)
	}
	if got, readErr := os.ReadFile(program); readErr != nil || string(got) != string(v1) {
		t.Fatalf("rolled-back program = %q, %v", got, readErr)
	}
	running, runningErr := VerifyRunning(stateRoot, program)
	if runningErr != nil || running.State != StateCurrent || running.Record.ReleaseVersion != 3 {
		t.Fatalf("VerifyRunning() after rollback = %+v, %v", running, runningErr)
	}
	recovery, recoveryErr := Recover(stateRoot, program)
	if recoveryErr != nil || recovery.State != "rollback-committed-restart-permitted" {
		t.Fatalf("Recover() after rollback = %+v, %v", recovery, recoveryErr)
	}
}

func TestReplaceRetiresOnlyCompletedPredecessorBeforeNextAuthorizedSuccessor(t *testing.T) {
	if err := requireLinux(); err != nil {
		t.Skip(err)
	}
	root := t.TempDir()
	program := filepath.Join(root, "ardents")
	stateRoot := replacementStateRoot(t)
	v1, v2, v3 := []byte("current program v1"), []byte("candidate program v2"), []byte("candidate program v3")
	if err := os.WriteFile(program, v1, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := Prepare(context.Background(), Request{StateRoot: stateRoot, Artifact: v1, decision: replacementDecision(v1, 1)}); err != nil {
		t.Fatal(err)
	}
	if _, err := CommitPrepared(stateRoot, program); err != nil {
		t.Fatal(err)
	}
	for _, candidate := range []struct {
		artifact []byte
		version  int64
	}{{v2, 2}, {v3, 3}} {
		unit := &replacementUnit{}
		result, err := Replace(context.Background(), Operation{Request: Request{StateRoot: stateRoot, Artifact: candidate.artifact,
			decision: replacementDecision(candidate.artifact, candidate.version)}, ProgramPath: program, Unit: unit,
			SelfTest: replacementSelfTest{program: program}})
		if err != nil || result.State != "committed-restart-permitted" || !unit.stopped || !unit.started {
			t.Fatalf("Replace(v%d) = %+v, %v; unit=%+v", candidate.version, result, err, unit)
		}
	}
	if got, err := os.ReadFile(program); err != nil || string(got) != string(v3) {
		t.Fatalf("current program = %q, %v", got, err)
	}
}

func TestReplaceInterruptionLeavesOnlyExplicitRecoveryPaths(t *testing.T) {
	if err := requireLinux(); err != nil {
		t.Skip(err)
	}
	for _, row := range []struct {
		checkpoint, wantProgram, wantRecovery string
	}{
		{"rollback-retained", "current program v1", "keep-current"},
		{"staged", "current program v1", "keep-current"},
		{"activated", "candidate program v2", "self-test-required"},
		{"committed", "candidate program v2", "committed-restart-permitted"},
	} {
		t.Run(row.checkpoint, func(t *testing.T) {
			program, stateRoot, v2 := replacementFixture(t)
			unit := &replacementUnit{}
			result, err := replaceWithInterruption(context.Background(), Operation{Request: Request{StateRoot: stateRoot, Artifact: v2,
				decision: replacementDecision(v2, 2)}, ProgramPath: program, Unit: unit, SelfTest: replacementSelfTest{program: program}}, operationControl{interruptAfter: row.checkpoint})
			if !errors.Is(err, errOperationInterrupted) {
				t.Fatalf("replaceWithInterruption() = %+v, %v", result, err)
			}
			programBytes, readErr := os.ReadFile(program)
			if readErr != nil || string(programBytes) != row.wantProgram {
				t.Fatalf("program after %s = %q, %v", row.checkpoint, programBytes, readErr)
			}
			recovery, recoveryErr := Recover(stateRoot, program)
			if recoveryErr != nil || recovery.State != row.wantRecovery {
				t.Fatalf("Recover() after %s = %+v, %v", row.checkpoint, recovery, recoveryErr)
			}
		})
	}
}

func replacementFixture(t *testing.T) (string, string, []byte) {
	t.Helper()
	root := t.TempDir()
	program := filepath.Join(root, "ardents")
	stateRoot := replacementStateRoot(t)
	v1, v2 := []byte("current program v1"), []byte("candidate program v2")
	if err := os.WriteFile(program, v1, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := Prepare(context.Background(), Request{StateRoot: stateRoot, Artifact: v1, decision: replacementDecision(v1, 1)}); err != nil {
		t.Fatal(err)
	}
	if _, err := CommitPrepared(stateRoot, program); err != nil {
		t.Fatal(err)
	}
	return program, stateRoot, v2
}

func replacementDecision(artifact []byte, version int64) release.Decision {
	digest := sha256.Sum256(artifact)
	return release.Decision{Outcome: release.OutcomeReleaseAccepted, BuildSafety: release.OutcomeReleaseAccepted,
		Protocol: release.OutcomeReleaseAccepted, Path: "ardents/linux-amd64/ardents", Length: int64(len(artifact)), Digest: digest[:],
		Platform: "linux-amd64", Architecture: "amd64", Environment: "h4-alpha", Network: "ardents-alpha",
		ReleaseIdentity: "endpoint-replacement-test", ReleaseVersion: version}
}

type replacementUnit struct {
	stopped, started bool
	onStop, onStart  func(context.Context) error
}

func (unit *replacementUnit) Stop(ctx context.Context) error {
	unit.stopped = true
	if unit.onStop != nil {
		return unit.onStop(ctx)
	}
	return nil
}

func (unit *replacementUnit) Start(ctx context.Context) error {
	unit.started = true
	if unit.onStart != nil {
		return unit.onStart(ctx)
	}
	return nil
}

type replacementSelfTest struct {
	program   string
	successes *int
}

func (test replacementSelfTest) Check(_ context.Context, stateRoot string) error {
	running, err := VerifyPreparedRunning(stateRoot, test.program)
	if err != nil || running.State != StatePrepared {
		return errors.New("candidate does not match prepared record")
	}
	if test.successes != nil {
		(*test.successes)++
	}
	return nil
}

type failingSelfTest struct{}

func (failingSelfTest) Check(context.Context, string) error {
	return errors.New("candidate self-test failed")
}
