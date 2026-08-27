package replacement

import (
	"context"
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/release"
)

func TestReplaceActivatesOnlyAfterPreparedCandidateSelfTest(t *testing.T) {
	if err := requireLinux(); err != nil {
		t.Skip(err)
	}
	root := t.TempDir()
	program := filepath.Join(root, "ardents")
	stateRoot := filepath.Join(root, "state", "replacement")
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

func TestReplaceRetainsCandidateForAuthorizedRollbackAfterSelfTestFailure(t *testing.T) {
	if err := requireLinux(); err != nil {
		t.Skip(err)
	}
	root := t.TempDir()
	program := filepath.Join(root, "ardents")
	stateRoot := filepath.Join(root, "state", "replacement")
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
	stateRoot := filepath.Join(root, "state", "replacement")
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
	stateRoot := filepath.Join(root, "state", "replacement")
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
	stateRoot := filepath.Join(root, "state", "replacement")
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
		ReleaseIdentity: "h4-1b-test", ReleaseVersion: version}
}

type replacementUnit struct{ stopped, started bool }

func (unit *replacementUnit) Stop(context.Context) error  { unit.stopped = true; return nil }
func (unit *replacementUnit) Start(context.Context) error { unit.started = true; return nil }

type replacementSelfTest struct{ program string }

func (test replacementSelfTest) Check(_ context.Context, stateRoot string) error {
	running, err := VerifyPreparedRunning(stateRoot, test.program)
	if err != nil || running.State != StatePrepared {
		return errors.New("candidate does not match prepared record")
	}
	return nil
}

type failingSelfTest struct{}

func (failingSelfTest) Check(context.Context, string) error {
	return errors.New("candidate self-test failed")
}
