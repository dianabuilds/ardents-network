//go:build linux

package replacement

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/release"
)

func TestReplaceRetriesExactPreactivationCandidateWithFreshNoUpdateAuthorization(t *testing.T) {
	fixture := replacementProtectedFixture(t, true)
	original, err := os.ReadFile(fixture.program)
	if err != nil {
		t.Fatal(err)
	}
	unit := &replacementUnit{onStop: func(context.Context) error { return errors.New("temporary stop refusal") }}
	first, err := Replace(context.Background(), protectedAuthorizedOperation(fixture, fixture.candidate, unit, replacementSelfTest{program: fixture.program}))
	if err == nil || first.State != "stop-refused" || !unit.stopped || unit.started {
		t.Fatalf("first Replace() = %+v, %v; unit=%+v", first, err, unit)
	}
	if program, readErr := os.ReadFile(fixture.program); readErr != nil || string(program) != string(original) {
		t.Fatalf("program after stop refusal = %q, %v", program, readErr)
	}
	if recovery, recoveryErr := Recover(fixture.stateRoot, fixture.program); recoveryErr != nil || recovery.State != "keep-current" {
		t.Fatalf("Recover() after stop refusal = %+v, %v", recovery, recoveryErr)
	}
	authorization := freshNoUpdateAuthorization(t, fixture)
	unit.onStop = nil
	second, err := Replace(context.Background(), Operation{Request: Request{StateRoot: fixture.stateRoot, Artifact: fixture.candidate,
		Authorization: authorization}, ProgramPath: fixture.program, Unit: unit, SelfTest: replacementSelfTest{program: fixture.program}})
	if err != nil || second.State != "committed-restart-permitted" || !unit.started {
		t.Fatalf("retry Replace() = %+v, %v; unit=%+v", second, err, unit)
	}
	if program, readErr := os.ReadFile(fixture.program); readErr != nil || string(program) != string(fixture.candidate) {
		t.Fatalf("program after retry = %q, %v", program, readErr)
	}
	if running, runningErr := VerifyRunning(fixture.stateRoot, fixture.program); runningErr != nil || running.State != StateCurrent || running.Record != second.Current {
		t.Fatalf("VerifyRunning() after retry = %+v, %v", running, runningErr)
	}
	assertProtectedTreeUnchanged(t, fixture.vaultRoot, fixture.vaultBefore)
	assertProtectedTreeUnchanged(t, fixture.releaseRoot, fixture.releaseBefore)
	assertReleaseFloorRootRemainsValid(t, fixture)
	stableAuthorization := freshNoUpdateAuthorization(t, fixture)
	stableUnit := &replacementUnit{onStop: func(context.Context) error { return errors.New("completed retry must not stop") }}
	stable, stableErr := Replace(context.Background(), Operation{Request: Request{StateRoot: fixture.stateRoot, Artifact: fixture.candidate,
		Authorization: stableAuthorization}, ProgramPath: fixture.program, Unit: stableUnit, SelfTest: replacementSelfTest{program: fixture.program}})
	if stableErr != nil || stable.State != "committed-restart-permitted" || stableUnit.stopped || stableUnit.started {
		t.Fatalf("completed Replace() = %+v, %v; unit=%+v", stable, stableErr, stableUnit)
	}
}

func TestReplaceRejectsNoUpdateWithoutItsExactPreactivationTransaction(t *testing.T) {
	fixture := replacementProtectedFixture(t, true)
	authorization := freshNoUpdateAuthorization(t, fixture)
	for _, attempt := range []struct {
		name, programPath string
		artifact          []byte
	}{
		{name: "unbound-no-update", programPath: fixture.program, artifact: fixture.candidate},
		{name: "substituted-candidate", programPath: fixture.program, artifact: []byte("foreign candidate")},
	} {
		t.Run(attempt.name, func(t *testing.T) {
			unit := &replacementUnit{}
			result, attemptErr := Replace(context.Background(), Operation{Request: Request{StateRoot: fixture.stateRoot, Artifact: attempt.artifact,
				Authorization: authorization}, ProgramPath: attempt.programPath, Unit: unit, SelfTest: replacementSelfTest{program: fixture.program}})
			if attemptErr == nil || result.State != "release-rejected" || unit.stopped || unit.started {
				t.Fatalf("Replace() = %+v, %v; unit=%+v", result, attemptErr, unit)
			}
			if running, runningErr := VerifyRunning(fixture.stateRoot, fixture.program); runningErr != nil || running.State != StateCurrent {
				t.Fatalf("VerifyRunning() after rejected retry = %+v, %v", running, runningErr)
			}
		})
	}
}

func TestReplaceRetriesNoUpdateAfterStagingDirectorySyncFailure(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Fatal("invalid test environment: staging retry requires a clean unprivileged Linux account; root bypasses chmod(0300)")
	}
	fixture := replacementProtectedFixture(t, true)
	directory := filepath.Dir(fixture.program)
	if err := os.Chmod(directory, 0o300); err != nil {
		t.Fatal(err)
	}
	unit := &replacementUnit{}
	first, firstErr := Replace(context.Background(), protectedAuthorizedOperation(fixture, fixture.candidate, unit, replacementSelfTest{program: fixture.program}))
	if err := os.Chmod(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	if firstErr == nil || first.State != "staging-failed" || !errors.Is(firstErr, fs.ErrPermission) || unit.stopped || unit.started {
		t.Fatalf("Replace() after staging directory-sync failure = %+v, %v; unit=%+v", first, firstErr, unit)
	}
	if recovery, recoveryErr := Recover(fixture.stateRoot, fixture.program); recoveryErr != nil || recovery.State != "keep-current" {
		t.Fatalf("Recover() after staging failure = %+v, %v", recovery, recoveryErr)
	}
	secondUnit := &replacementUnit{}
	second, secondErr := Replace(context.Background(), Operation{Request: Request{StateRoot: fixture.stateRoot, Artifact: fixture.candidate,
		Authorization: freshNoUpdateAuthorization(t, fixture)}, ProgramPath: fixture.program, Unit: secondUnit, SelfTest: replacementSelfTest{program: fixture.program}})
	if secondErr != nil || second.State != "committed-restart-permitted" || !secondUnit.stopped || !secondUnit.started {
		t.Fatalf("Replace() retry after staging failure = %+v, %v; unit=%+v", second, secondErr, secondUnit)
	}
}

func TestReplaceRejectsNoUpdateWhenPreparedRecordDiffersBeyondCandidateBytes(t *testing.T) {
	fixture := replacementProtectedFixture(t, true)
	firstUnit := &replacementUnit{onStop: func(context.Context) error { return errors.New("temporary stop refusal") }}
	if _, err := Replace(context.Background(), protectedAuthorizedOperation(fixture, fixture.candidate, firstUnit, replacementSelfTest{program: fixture.program})); err == nil {
		t.Fatal("first Replace() unexpectedly succeeded")
	}
	state, err := openStore(fixture.stateRoot, false)
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := state.prepared()
	if err != nil {
		_ = state.close()
		t.Fatal(err)
	}
	prepared.ReleaseID += "-different"
	encoded, err := encodeRecord(prepared)
	if err == nil {
		err = writeAtomic(state.root, preparedName, encoded)
	}
	if closeErr := state.close(); err == nil {
		err = closeErr
	}
	if err != nil {
		t.Fatal(err)
	}
	before := preactivationEvidence(t, fixture.stateRoot)
	unit := &replacementUnit{}
	result, replaceErr := Replace(context.Background(), Operation{Request: Request{StateRoot: fixture.stateRoot, Artifact: fixture.candidate,
		Authorization: freshNoUpdateAuthorization(t, fixture)}, ProgramPath: fixture.program, Unit: unit, SelfTest: replacementSelfTest{program: fixture.program}})
	if replaceErr == nil || result.State != "repair-required" || unit.stopped || unit.started {
		t.Fatalf("Replace() with a different prepared record = %+v, %v; unit=%+v", result, replaceErr, unit)
	}
	if after := preactivationEvidence(t, fixture.stateRoot); after != before {
		t.Fatal("rejected retry changed retained preactivation evidence")
	}
}

func TestReplaceSerializesNoUpdateRetryWithoutMutatingRetainedEvidence(t *testing.T) {
	fixture := replacementProtectedFixture(t, true)
	stopEntered := make(chan struct{})
	allowStop := make(chan struct{})
	firstUnit := &replacementUnit{onStop: func(context.Context) error {
		close(stopEntered)
		<-allowStop
		return errors.New("temporary stop refusal")
	}}
	firstDone := make(chan error, 1)
	go func() {
		_, err := Replace(context.Background(), protectedAuthorizedOperation(fixture, fixture.candidate, firstUnit, replacementSelfTest{program: fixture.program}))
		firstDone <- err
	}()
	<-stopEntered
	before := preactivationEvidence(t, fixture.stateRoot)
	secondUnit := &replacementUnit{}
	second, secondErr := Replace(context.Background(), Operation{Request: Request{StateRoot: fixture.stateRoot, Artifact: fixture.candidate,
		Authorization: freshNoUpdateAuthorization(t, fixture)}, ProgramPath: fixture.program, Unit: secondUnit, SelfTest: replacementSelfTest{program: fixture.program}})
	if secondErr == nil || second.State != "invalid" || secondUnit.stopped || secondUnit.started {
		t.Fatalf("concurrent Replace() = %+v, %v; unit=%+v", second, secondErr, secondUnit)
	}
	if after := preactivationEvidence(t, fixture.stateRoot); after != before {
		t.Fatal("concurrent rejected retry changed retained preactivation evidence")
	}
	close(allowStop)
	if err := <-firstDone; err == nil {
		t.Fatal("first Replace() unexpectedly succeeded")
	}
}

type retainedEvidence struct {
	current, prepared, journal, predecessor string
}

func preactivationEvidence(t *testing.T, stateRoot string) retainedEvidence {
	t.Helper()
	read := func(name string) string {
		contents, err := os.ReadFile(filepath.Join(stateRoot, name))
		if err != nil {
			t.Fatal(err)
		}
		return string(contents)
	}
	return retainedEvidence{current: read(currentName), prepared: read(preparedName), journal: read(journalName), predecessor: read(rollbackName)}
}

func freshNoUpdateAuthorization(t *testing.T, fixture protectedReplacementFixture) release.Authorization {
	t.Helper()
	verifier, err := release.Open(fixture.releaseRoot)
	if err != nil {
		t.Fatal(err)
	}
	decision := verifier.Evaluate(context.Background(), fixture.releaseInputs)
	if err := verifier.Close(); err != nil {
		t.Fatal(err)
	}
	if decision.Outcome != release.OutcomeNoUpdate {
		t.Fatalf("retry Release decision = %s, want %s", decision.Outcome, release.OutcomeNoUpdate)
	}
	authorization, ok := decision.Authorization()
	if !ok {
		t.Fatal("NoUpdate did not carry an opaque authorization")
	}
	return authorization
}
