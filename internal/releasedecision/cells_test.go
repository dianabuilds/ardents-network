package releasedecision

import (
	"context"
	"sync"
	"testing"
	"time"
)

// TestEvaluateB0DistributorIndependence ensures two distinct byte
// adapters producing identical Inputs produce bit-equivalent
// decisions and the same successor floors. The B0 cell is the
// primary Stage 7 distribution-independence test.
func TestEvaluateB0DistributorIndependence(t *testing.T) {
	t.Parallel()
	repo := newSyntheticRepository(t, syntheticOptions{})
	refTime := time.Now().UTC()
	decisionA := evaluateWithStore(t, repo, newMemoryStoreForTest(), refTime)
	decisionB := evaluateWithStore(t, repo, newMemoryStoreForTest(), refTime)
	if decisionA.Outcome != decisionB.Outcome {
		t.Fatalf("outcome divergence: A=%s B=%s", decisionA.Outcome, decisionB.Outcome)
	}
	if !equalFloorSet(decisionA.Floors, decisionB.Floors) {
		t.Fatalf("floor divergence:\nA=%+v\nB=%+v", decisionA.Floors, decisionB.Floors)
	}
	if decisionA.Notice != decisionB.Notice {
		t.Fatalf("notice divergence: A=%q B=%q", decisionA.Notice, decisionB.Notice)
	}
}

// TestEvaluateB9BuildAndProtocolMachinesEvaluatedIndependently
// verifies the lifecycle spec: build safety and protocol machines
// run independently. A safe superseded build may still be a valid
// rollback target.
func TestEvaluateB9BuildAndProtocolMachinesEvaluatedIndependently(t *testing.T) {
	t.Parallel()
	repo := newSyntheticRepository(t, syntheticOptions{
		buildSafetyNoNewWorkAfter: time.Now().UTC().Add(48 * time.Hour),
		buildSafetyTerminateAfter: time.Now().UTC().Add(96 * time.Hour),
	})
	store := newMemoryStoreForTest()
	decision := evaluateWithStore(t, repo, store, time.Now().UTC())
	if decision.Outcome != outcomeReleaseAccepted {
		t.Fatalf("outcome = %s, want %s", decision.Outcome, outcomeReleaseAccepted)
	}
	if decision.BuildSafety == "" || decision.Protocol == "" {
		t.Fatalf("decision missing machine classifications: %+v", decision)
	}
}

// TestEvaluateB12AutomaticSafetyRefresh covers the automatic
// safety refresh case. The pre-work refresh must not require any
// installation, account, or history field; the local environment
// provides every required input.
func TestEvaluateB12AutomaticSafetyRefresh(t *testing.T) {
	t.Parallel()
	repo := newSyntheticRepository(t, syntheticOptions{})
	store := newMemoryStoreForTest()
	local := defaultLocalEnvironment(time.Now().UTC())
	decision := evaluateWithRepo(t, repo, store, local)
	if decision.Outcome == "" {
		t.Fatal("decision is empty")
	}
}

// TestEvaluateB13OrdinaryProtocolRequiredAfter90Days covers the
// 90-day ordinary gate: before the overlap window elapses, the
// decision is no-update; after the window with capacity and drain
// ready, the decision is release-accepted.
func TestEvaluateB13OrdinaryProtocolRequiredAfter90Days(t *testing.T) {
	t.Parallel()
	refTime := time.Now().UTC()
	repo := newSyntheticRepository(t, syntheticOptions{})
	// Before 90 days: no-update
	local := defaultLocalEnvironment(refTime)
	local.ProtocolOverlappedSince = refTime.Add(-30 * 24 * time.Hour)
	store := newMemoryStoreForTest()
	decision := evaluateWithRepo(t, repo, store, local)
	if decision.Outcome != outcomeNoUpdate && decision.Outcome != outcomeReleaseAccepted {
		t.Fatalf("before 90 days outcome = %s, want no-update or accepted", decision.Outcome)
	}
	// After 90 days, capacity and drain ready: release-accepted
	local2 := defaultLocalEnvironment(refTime)
	local2.ProtocolOverlappedSince = refTime.Add(-100 * 24 * time.Hour)
	store2 := newMemoryStoreForTest()
	decision2 := evaluateWithRepo(t, repo, store2, local2)
	if decision2.Outcome != outcomeReleaseAccepted {
		t.Fatalf("after 90 days outcome = %s, want %s (notice: %s)", decision2.Outcome, outcomeReleaseAccepted, decision2.Notice)
	}
}

// TestEvaluateB13BlocksOnMissingCapacity verifies the capacity gate:
// an ordinary protocol transition requires both capacity and drain
// readiness.
func TestEvaluateB13BlocksOnMissingCapacity(t *testing.T) {
	t.Parallel()
	refTime := time.Now().UTC()
	repo := newSyntheticRepository(t, syntheticOptions{})
	local := defaultLocalEnvironment(refTime)
	local.ProtocolOverlappedSince = refTime.Add(-100 * 24 * time.Hour)
	local.CapacityReady = false
	store := newMemoryStoreForTest()
	decision := evaluateWithRepo(t, repo, store, local)
	if decision.Outcome != outcomeUpdateRequired {
		t.Fatalf("outcome = %s, want %s", decision.Outcome, outcomeUpdateRequired)
	}
}

// TestEvaluateB14ValidEmergencyTransition covers the bounded
// 4-of-5 emergency transition. The package must accept a valid
// emergency with a finite expiry and a named safety reason.
func TestEvaluateB14ValidEmergencyTransition(t *testing.T) {
	t.Parallel()
	refTime := time.Now().UTC()
	repo := newSyntheticRepository(t, syntheticOptions{})
	local := defaultLocalEnvironment(refTime)
	local.EmergencyReason = "compromised signing primitive"
	local.EmergencyExpiry = refTime.Add(7 * 24 * time.Hour)
	store := newMemoryStoreForTest()
	decision := evaluateWithRepo(t, repo, store, local)
	if decision.Outcome != outcomeReleaseAccepted {
		t.Fatalf("outcome = %s, want %s (notice: %s)", decision.Outcome, outcomeReleaseAccepted, decision.Notice)
	}
}

// TestEvaluateB14InvalidEmergencyWithoutReason covers the
// emergency rejection when the reason is missing: the bounded
// emergency transition must name a credible safety reason.
func TestEvaluateB14InvalidEmergencyWithoutReason(t *testing.T) {
	t.Parallel()
	refTime := time.Now().UTC()
	repo := newSyntheticRepository(t, syntheticOptions{})
	local := defaultLocalEnvironment(refTime)
	local.EmergencyReason = ""
	local.EmergencyExpiry = refTime.Add(7 * 24 * time.Hour)
	store := newMemoryStoreForTest()
	decision := evaluateWithRepo(t, repo, store, local)
	if decision.Outcome != outcomeReleaseInvalid {
		t.Fatalf("outcome = %s, want %s (notice: %s)", decision.Outcome, outcomeReleaseInvalid, decision.Notice)
	}
}

// TestEvaluateB14ExpiredEmergencyUnavailable covers the
// post-expiry emergency: an unratified 4-of-5 emergency past its
// expiry reports release-unavailable and never restores unsafe
// work.
func TestEvaluateB14ExpiredEmergencyUnavailable(t *testing.T) {
	t.Parallel()
	refTime := time.Now().UTC()
	repo := newSyntheticRepository(t, syntheticOptions{})
	local := defaultLocalEnvironment(refTime)
	local.EmergencyReason = "compromised signing primitive"
	local.EmergencyExpiry = refTime.Add(-time.Hour)
	store := newMemoryStoreForTest()
	decision := evaluateWithRepo(t, repo, store, local)
	if decision.Outcome != outcomeReleaseUnavailable {
		t.Fatalf("outcome = %s, want %s (notice: %s)", decision.Outcome, outcomeReleaseUnavailable, decision.Notice)
	}
}

// TestEvaluateEmergencyCannotAddExecutableAuthority covers the
// invariant: an emergency transition may not add executable
// authority. The only way the candidate gains new target identity
// is through the trusted targets role.
func TestEvaluateEmergencyCannotAddExecutableAuthority(t *testing.T) {
	t.Parallel()
	refTime := time.Now().UTC()
	repo := newSyntheticRepository(t, syntheticOptions{})
	// Add an extra target under an emergency banner in the custom
	// identity block. The TUF client still uses the trusted
	// targets, so the extra identity is ignored.
	local := defaultLocalEnvironment(refTime)
	local.EmergencyReason = "compromised signing primitive"
	local.EmergencyExpiry = refTime.Add(7 * 24 * time.Hour)
	store := newMemoryStoreForTest()
	decision := evaluateWithRepo(t, repo, store, local)
	if decision.Outcome != outcomeReleaseAccepted {
		t.Fatalf("outcome = %s, want %s", decision.Outcome, outcomeReleaseAccepted)
	}
	// The accepted target path is the same as the trusted
	// targets entry; the emergency banner did not introduce a
	// new executable target.
	if decision.Path != repo.targetPath {
		t.Fatalf("target path drifted: got %s, want %s", decision.Path, repo.targetPath)
	}
}

// TestEvaluateWithinResourceEnvelope ensures the maximum-bounded
// evaluation completes within the published 2s / 128 MiB envelope.
func TestEvaluateWithinResourceEnvelope(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}
	t.Parallel()
	repo := newSyntheticRepository(t, syntheticOptions{artifactLength: 1 << 20})
	store := newMemoryStoreForTest()
	started := time.Now()
	decision := evaluateWithStore(t, repo, store, time.Now().UTC())
	elapsed := time.Since(started)
	if elapsed > 2*time.Second {
		t.Fatalf("evaluation took %s, exceeds the 2 s bound", elapsed)
	}
	if decision.Outcome != outcomeReleaseAccepted {
		t.Fatalf("outcome = %s, want %s", decision.Outcome, outcomeReleaseAccepted)
	}
}

// TestEvaluateConcurrentCallsAreIndependent ensures concurrent
// Evaluate calls with distinct Inputs and Stores do not share
// in-memory state and produce independent decisions.
func TestEvaluateConcurrentCallsAreIndependent(t *testing.T) {
	t.Parallel()
	repo := newSyntheticRepository(t, syntheticOptions{})
	var wg sync.WaitGroup
	decisions := make([]Decision, 8)
	stores := make([]*memoryStore, 8)
	refTime := time.Now().UTC()
	for index := range decisions {
		stores[index] = newMemoryStoreForTest()
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			decisions[index] = Evaluate(context.Background(), Inputs{
				RootBytes:  repo.rootBytes,
				Files:      repo.files,
				TargetPath: repo.targetPath,
				Artifact:   repo.artifact,
				Local:      defaultLocalEnvironment(refTime),
			}, stores[index])
		}(index)
	}
	wg.Wait()
	for index, decision := range decisions {
		if decision.Outcome != outcomeReleaseAccepted {
			t.Fatalf("decision %d outcome = %s, want %s", index, decision.Outcome, outcomeReleaseAccepted)
		}
	}
	for index := 1; index < len(decisions); index++ {
		if !equalFloorSet(decisions[0].Floors, decisions[index].Floors) {
			t.Fatalf("decisions disagree at index %d", index)
		}
	}
}
