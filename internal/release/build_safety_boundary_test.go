package release

import (
	"testing"
	"time"
)

func TestEvaluateBuildSafetyNoNewWorkDeadlineIsExclusive(t *testing.T) {
	noNewWorkAfter := testRefTime.Add(time.Hour)
	terminateAfter := noNewWorkAfter.Add(time.Hour)
	repository := newSyntheticRepository(t, syntheticOptions{
		expires:                   terminateAfter.Add(time.Hour),
		buildSafetyNoNewWorkAfter: noNewWorkAfter,
		buildSafetyTerminateAfter: terminateAfter,
	})
	store := newMemoryStoreForTest()

	before := evaluateWithStore(t, repository, store, noNewWorkAfter.Add(-time.Second))
	if before.Outcome != OutcomeReleaseAccepted || before.BuildSafety != OutcomeReleaseAccepted {
		t.Fatalf("before no-new-work outcome=%q build_safety=%q", before.Outcome, before.BuildSafety)
	}
	if _, authorized := before.Authorization(); !authorized {
		t.Fatal("before no-new-work decision lacks authorization")
	}
	acceptedFloors := before.Floors

	for _, at := range []time.Time{noNewWorkAfter, noNewWorkAfter.Add(time.Second)} {
		decision := evaluateWithStore(t, repository, store, at)
		if decision.Outcome != OutcomeUpdateRequired || decision.BuildSafety != OutcomeUpdateRequired {
			t.Fatalf("at %s outcome=%q build_safety=%q, want update-required", at.Format(time.RFC3339), decision.Outcome, decision.BuildSafety)
		}
		if _, authorized := decision.Authorization(); authorized {
			t.Fatalf("at %s update-required decision carried authorization", at.Format(time.RFC3339))
		}
		if !floorSetEqual(decision.Floors, acceptedFloors) {
			t.Fatalf("at %s rejected decision changed floors: got=%+v want=%+v", at.Format(time.RFC3339), decision.Floors, acceptedFloors)
		}
		if !decision.BuildSafetyNoNewWorkAfter.Equal(noNewWorkAfter) || !decision.BuildSafetyTerminateAfter.Equal(terminateAfter) {
			t.Fatalf("at %s authenticated build-safety bounds changed: %+v", at.Format(time.RFC3339), decision)
		}
	}
}

func TestEvaluateBuildSafetyTerminalDeadlineIsExclusive(t *testing.T) {
	noNewWorkAfter := testRefTime.Add(time.Hour)
	terminateAfter := noNewWorkAfter.Add(time.Hour)
	repository := newSyntheticRepository(t, syntheticOptions{
		expires:                   terminateAfter.Add(time.Hour),
		buildSafetyNoNewWorkAfter: noNewWorkAfter,
		buildSafetyTerminateAfter: terminateAfter,
	})
	store := newMemoryStoreForTest()

	accepted := evaluateWithStore(t, repository, store, noNewWorkAfter.Add(-time.Second))
	if accepted.Outcome != OutcomeReleaseAccepted {
		t.Fatalf("seed outcome=%q, want release-accepted", accepted.Outcome)
	}
	acceptedFloors := accepted.Floors

	before := evaluateWithStore(t, repository, store, terminateAfter.Add(-time.Second))
	if before.Outcome != OutcomeUpdateRequired || before.BuildSafety != OutcomeUpdateRequired {
		t.Fatalf("before terminal outcome=%q build_safety=%q, want update-required", before.Outcome, before.BuildSafety)
	}
	if _, authorized := before.Authorization(); authorized {
		t.Fatal("before terminal update-required decision carried authorization")
	}
	if !floorSetEqual(before.Floors, acceptedFloors) {
		t.Fatalf("before terminal rejected decision changed floors: got=%+v want=%+v", before.Floors, acceptedFloors)
	}

	for _, at := range []time.Time{terminateAfter, terminateAfter.Add(time.Second)} {
		decision := evaluateWithStore(t, repository, store, at)
		if decision.Outcome != OutcomeReleaseRevoked || decision.BuildSafety != OutcomeReleaseRevoked {
			t.Fatalf("at %s outcome=%q build_safety=%q, want release-revoked", at.Format(time.RFC3339), decision.Outcome, decision.BuildSafety)
		}
		if _, authorized := decision.Authorization(); authorized {
			t.Fatalf("at %s revoked decision carried authorization", at.Format(time.RFC3339))
		}
		if !floorSetEqual(decision.Floors, acceptedFloors) {
			t.Fatalf("at %s revoked decision changed floors: got=%+v want=%+v", at.Format(time.RFC3339), decision.Floors, acceptedFloors)
		}
		if !decision.BuildSafetyNoNewWorkAfter.Equal(noNewWorkAfter) || !decision.BuildSafetyTerminateAfter.Equal(terminateAfter) {
			t.Fatalf("at %s authenticated build-safety bounds changed: %+v", at.Format(time.RFC3339), decision)
		}
	}
}
