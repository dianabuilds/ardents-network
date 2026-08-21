package releasedecision

import (
	"context"
	"runtime"
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
	refTime := testRefTime
	decisionA := evaluateWithStore(t, repo, newMemoryStoreForTest(), refTime)
	decisionB := evaluateWithStore(t, repo, newMemoryStoreForTest(), refTime)
	if decisionA.Outcome != decisionB.Outcome {
		t.Fatalf("outcome divergence: A=%s B=%s", decisionA.Outcome, decisionB.Outcome)
	}
	if !floorSetEqual(decisionA.Floors, decisionB.Floors) {
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
		buildSafetyNoNewWorkAfter: testRefTime.Add(48 * time.Hour),
		buildSafetyTerminateAfter: testRefTime.Add(96 * time.Hour),
	})
	store := newMemoryStoreForTest()
	decision := evaluateWithStore(t, repo, store, testRefTime)
	if decision.Outcome != outcomeReleaseAccepted {
		t.Fatalf("outcome = %s, want %s (notice: %s)", decision.Outcome, outcomeReleaseAccepted, decision.Notice)
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
	local := defaultLocalEnvironment(testRefTime)
	decision := evaluateWithRepo(t, repo, store, local)
	if decision.Outcome != outcomeReleaseAccepted {
		t.Fatalf("outcome = %s, want %s (notice: %s)", decision.Outcome, outcomeReleaseAccepted, decision.Notice)
	}
	if decision.Floors.TimestampVersion == 0 || decision.Floors.TargetsVersion == 0 {
		t.Fatal("automatic refresh did not publish authenticated safety floors")
	}
}

// TestEvaluateB13OrdinaryProtocolRequiredAfter90Days covers the
// 90-day ordinary gate: before the overlap window elapses, the
// decision is no-update; after the window with capacity and drain
// ready, the decision is release-accepted.
func TestEvaluateB13OrdinaryProtocolRequiredAfter90Days(t *testing.T) {
	t.Parallel()
	refTime := testRefTime
	repo := newSyntheticRepository(t, syntheticOptions{protocolOverlappedSince: refTime.Add(-30 * 24 * time.Hour)})
	// Before 90 days: no-update
	local := defaultLocalEnvironment(refTime)
	store := newMemoryStoreForTest()
	decision := evaluateWithRepo(t, repo, store, local)
	if decision.Outcome != outcomeNoUpdate {
		t.Fatalf("before 90 days outcome = %s, want %s", decision.Outcome, outcomeNoUpdate)
	}
	// After 90 days, capacity and drain ready: release-accepted
	repo2 := newSyntheticRepository(t, syntheticOptions{protocolOverlappedSince: refTime.Add(-100 * 24 * time.Hour)})
	local2 := defaultLocalEnvironment(refTime)
	store2 := newMemoryStoreForTest()
	decision2 := evaluateWithRepo(t, repo2, store2, local2)
	if decision2.Outcome != outcomeReleaseAccepted {
		t.Fatalf("after 90 days outcome = %s, want %s (notice: %s)", decision2.Outcome, outcomeReleaseAccepted, decision2.Notice)
	}
}

// TestEvaluateB13BlocksOnMissingCapacity verifies the capacity gate:
// an ordinary protocol transition requires both capacity and drain
// readiness.
func TestEvaluateB13BlocksOnMissingCapacity(t *testing.T) {
	t.Parallel()
	refTime := testRefTime
	repo := newSyntheticRepository(t, syntheticOptions{capacityNotReady: true})
	local := defaultLocalEnvironment(refTime)
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
	refTime := testRefTime
	emergencyExpiry := refTime.Add(7 * 24 * time.Hour)
	repo := newSyntheticRepository(t, syntheticOptions{
		emergencyReason: "compromised-primitive-or-key",
		emergencyExpiry: emergencyExpiry,
	})
	local := defaultLocalEnvironment(refTime)
	store := newMemoryStoreForTest()
	decision := evaluateWithRepo(t, repo, store, local)
	if decision.Outcome != outcomeReleaseAccepted {
		t.Fatalf("outcome = %s, want %s (notice: %s)", decision.Outcome, outcomeReleaseAccepted, decision.Notice)
	}
	if !decision.ReferenceTime.Equal(refTime) ||
		!decision.BuildSafetyNoNewWorkAfter.Equal(refTime.Add(30*24*time.Hour)) ||
		!decision.BuildSafetyTerminateAfter.Equal(refTime.Add(180*24*time.Hour)) ||
		!decision.ProtocolTransitionDeadline.Equal(emergencyExpiry) {
		t.Fatalf("emergency time facts mismatch: %+v", decision)
	}
}

func TestEvaluateB14RejectsOrdinaryThresholdEmergency(t *testing.T) {
	t.Parallel()
	refTime := testRefTime
	repo := newSyntheticRepository(t, syntheticOptions{
		emergencyReason:       "compromised-primitive-or-key",
		emergencyExpiry:       refTime.Add(7 * 24 * time.Hour),
		targetsSignatureCount: ordinaryThreshold,
	})
	decision := evaluateWithRepo(t, repo, newMemoryStoreForTest(), defaultLocalEnvironment(refTime))
	if decision.Outcome != outcomeReleaseInvalid {
		t.Fatalf("outcome = %s, want %s", decision.Outcome, outcomeReleaseInvalid)
	}
}

func TestEvaluateB14RejectsUnnamedSafetyCategory(t *testing.T) {
	t.Parallel()
	refTime := testRefTime
	repo := newSyntheticRepository(t, syntheticOptions{
		emergencyReason: "operator-convenience",
		emergencyExpiry: refTime.Add(24 * time.Hour),
	})
	decision := evaluateWithRepo(t, repo, newMemoryStoreForTest(), defaultLocalEnvironment(refTime))
	if decision.Outcome != outcomeReleaseInvalid {
		t.Fatalf("outcome = %s, want %s", decision.Outcome, outcomeReleaseInvalid)
	}
}

// TestEvaluateB14InvalidEmergencyWithoutReason covers the
// emergency rejection when the reason is missing: the bounded
// emergency transition must name a credible safety reason.
func TestEvaluateB14InvalidEmergencyWithoutReason(t *testing.T) {
	t.Parallel()
	refTime := testRefTime
	repo := newSyntheticRepository(t, syntheticOptions{emergencyExpiry: refTime.Add(7 * 24 * time.Hour)})
	local := defaultLocalEnvironment(refTime)
	store := newMemoryStoreForTest()
	decision := evaluateWithRepo(t, repo, store, local)
	if decision.Outcome != outcomeReleaseInvalid {
		t.Fatalf("outcome = %s, want %s (notice: %s)", decision.Outcome, outcomeReleaseInvalid, decision.Notice)
	}
	if !decision.ReferenceTime.IsZero() ||
		!decision.BuildSafetyNoNewWorkAfter.IsZero() ||
		!decision.BuildSafetyTerminateAfter.IsZero() ||
		!decision.ProtocolTransitionDeadline.IsZero() {
		t.Fatalf("identity-invalid decision carried time facts: %+v", decision)
	}
}

// TestEvaluateB14ExpiredEmergencyUnavailable covers the
// post-expiry emergency: an unratified 4-of-5 emergency past its
// expiry reports release-unavailable and never restores unsafe
// work.
func TestEvaluateB14ExpiredEmergencyUnavailable(t *testing.T) {
	t.Parallel()
	refTime := testRefTime
	repo := newSyntheticRepository(t, syntheticOptions{
		emergencyReason: "compromised-primitive-or-key",
		emergencyExpiry: refTime.Add(-time.Hour),
	})
	local := defaultLocalEnvironment(refTime)
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
	refTime := testRefTime
	repo := newSyntheticRepository(t, syntheticOptions{
		emergencyReason: "compromised-primitive-or-key",
		emergencyExpiry: refTime.Add(7 * 24 * time.Hour),
	})
	// Emergency policy is carried inside the already-authorized target
	// identity; it has no separate target-path or root-authority field.
	local := defaultLocalEnvironment(refTime)
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
	repo := withTargetCount(t, newSyntheticRepository(t, syntheticOptions{artifactLength: 1 << 20}), maximumTargets)
	store := newMemoryStoreForTest()
	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)
	started := time.Now()
	decision := evaluateWithStore(t, repo, store, testRefTime)
	elapsed := time.Since(started)
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	if elapsed > 2*time.Second {
		t.Fatalf("evaluation took %s, exceeds the 2 s bound", elapsed)
	}
	if decision.Outcome != outcomeReleaseAccepted {
		t.Fatalf("outcome = %s, want %s (notice: %s)", decision.Outcome, outcomeReleaseAccepted, decision.Notice)
	}
	if allocated := after.TotalAlloc - before.TotalAlloc; allocated > 128<<20 {
		t.Fatalf("evaluation allocated %d bytes, exceeds 128 MiB", allocated)
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
	refTime := testRefTime
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
		if !floorSetEqual(decisions[0].Floors, decisions[index].Floors) {
			t.Fatalf("decisions disagree at index %d", index)
		}
	}
}
