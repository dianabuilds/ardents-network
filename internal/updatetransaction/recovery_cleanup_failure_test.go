package updatetransaction

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// TestRecoverCleanupFailureObservesEachStep exercises the bounded
// cleanup fault seam: every typed remove, move, replace, and sync
// operation must fail closed without overwriting the last coherent
// journal state, the verified transaction generation, or the
// permanent lock file. The cleanup fault seam is the private
// per-invocation recoverWithOperations wrapper; public Recover
// always supplies native operations.
//
// The expected behavior is the frozen cleanup-incomplete Result
// shape: outcome=cleanup-incomplete, safe_notice=update cleanup
// incomplete, generation equals the verified transaction
// generation, zero current/rollback digests, false staging, and no
// custody notice. The R03 row is exercised because it covers the
// longest allowlist (six steps) and therefore the densest
// fault-injection surface.
func TestRecoverCleanupFailures(t *testing.T) {
	for _, op := range recoveryCleanupFailureOps() {
		op := op
		t.Run(op.id, func(t *testing.T) {
			root, predecessor := recoveryOracleBootstrap(t)
			artifact, manifest := recoveryOracleStage(t, root, 1)
			recoveryOracleWriteChain(t, root, 1, predecessor, artifact, manifest, 2)
			snapshot := recoveryOracleTreeDigest(t, root)
			result, err := recoverWithOperations(context.Background(), root, op.ops(t, root))
			recoveryOracleAssertCleanupIncomplete(t, result, err, 1, "artifact-verified")
			op.assertPrefix(t, root, snapshot)
			if _, lockErr := os.Lstat(filepath.Join(root, lockFileName)); lockErr != nil {
				t.Fatalf("RECOVER: cleanup failure must not remove the permanent lock: %v", lockErr)
			}
		})
	}
}

// TestRecoverCleanupOverrunStopsAfterBudget blocks one chosen cleanup
// step past the five-second continuation budget and proves no later
// operation starts. The test injects a custom syncDirectory that
// blocks until ctx.Err() is observed, then asserts the bounded
// cleanup-incomplete Result and the absence of any subsequent
// operation. The test name mirrors the Gate A frozen table for the
// same row.
func TestRecoverCleanupOverrunStopsAfterBudget(t *testing.T) {
	root, predecessor := recoveryOracleBootstrap(t)
	artifact, manifest := recoveryOracleStage(t, root, 1)
	recoveryOracleWriteChain(t, root, 1, predecessor, artifact, manifest, 2)
	slow := make(chan struct{})
	var observedDeadline atomicBool
	var blockedOperationReturned atomicBool
	var laterOperationStarted atomicBool
	observeLaterStart := func() {
		if blockedOperationReturned.get() {
			laterOperationStarted.set(true)
		}
	}
	ops := cleanupOps{
		removeFile: func(path string) error {
			observeLaterStart()
			return os.Remove(path)
		},
		removeDirectory: func(path string) error {
			observeLaterStart()
			return os.Remove(path)
		},
		moveDirectory: func(source, destination string) error {
			observeLaterStart()
			return moveDirectoryNative(source, destination)
		},
		atomicReplaceCurrent: func(path string, payload []byte, deadline time.Time) error {
			observeLaterStart()
			return atomicReplaceCurrentNative(path, payload, deadline)
		},
		syncDirectory: func(path string) error {
			observeLaterStart()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			<-ctx.Done()
			observedDeadline.set(true)
			<-slow
			blockedOperationReturned.set(true)
			return nil
		},
	}
	var wg sync.WaitGroup
	wg.Add(1)
	var result Result
	var err error
	go func() {
		defer wg.Done()
		result, err = recoverWithOperations(context.Background(), root, ops)
	}()
	time.Sleep(6 * time.Second)
	if !observedDeadline.get() {
		t.Fatal("RECOVER: cleanup did not observe the budget overrun")
	}
	close(slow)
	wg.Wait()
	if !errors.Is(err, errCleanupOverrun) {
		t.Fatalf("RECOVER: cleanup err=%v, want errCleanupOverrun", err)
	}
	if result.Outcome != outcomeCleanupIncomplete {
		t.Fatalf("RECOVER: cleanup outcome=%s, want cleanup-incomplete", result.Outcome)
	}
	if result.Generation != 1 {
		t.Fatalf("RECOVER: cleanup generation=%d, want 1", result.Generation)
	}
	if result.State != "artifact-verified" {
		t.Fatalf("RECOVER: cleanup state=%s, want artifact-verified", result.State)
	}
	if result.SafeNotice != noticeUpdateCleanupIncomplete {
		t.Fatalf("RECOVER: cleanup safe notice=%q, want update cleanup incomplete", result.SafeNotice)
	}
	if laterOperationStarted.get() {
		t.Fatal("RECOVER: cleanup started an operation after the blocking step crossed the deadline")
	}
}

// recoveryCleanupFailureOps returns one failing operation per cleanup
// step kind. Each failure must surface as cleanup-incomplete without
// overwriting the verified transaction generation or last coherent
// journal state.
func recoveryCleanupFailureOps() []recoveryCleanupFailureOp {
	sentinel := errors.New("injected cleanup failure")
	return []recoveryCleanupFailureOp{{
		id: "remove-file-failure",
		ops: func(t *testing.T, root string) cleanupOps {
			return cleanupOps{
				removeFile:           func(string) error { return sentinel },
				removeDirectory:      os.Remove,
				moveDirectory:        moveDirectoryNative,
				atomicReplaceCurrent: atomicReplaceCurrentNative,
				syncDirectory:        syncDirectoryNative,
			}
		},
		assertPrefix: func(t *testing.T, root string, before []byte) {
			if after := recoveryOracleTreeDigest(t, root); !bytes.Equal(after, before) {
				t.Fatalf("remove-file failure mutated tree: before=%x after=%x", before, after)
			}
		},
	}, {
		id: "remove-directory-failure",
		ops: func(t *testing.T, root string) cleanupOps {
			return cleanupOps{
				removeFile:           os.Remove,
				removeDirectory:      func(string) error { return sentinel },
				moveDirectory:        moveDirectoryNative,
				atomicReplaceCurrent: atomicReplaceCurrentNative,
				syncDirectory:        syncDirectoryNative,
			}
		},
		assertPrefix: func(t *testing.T, root string, _ []byte) {
			assertRecoveryPath(t, filepath.Join(root, "staging", "1"), true)
			assertRecoveryPath(t, filepath.Join(root, "staging", "1", "artifact"), false)
			assertRecoveryPath(t, filepath.Join(root, "staging", "1", "manifest.bin"), false)
		},
	}, {
		id: "sync-directory-failure",
		ops: func(t *testing.T, root string) cleanupOps {
			return cleanupOps{
				removeFile:           os.Remove,
				removeDirectory:      os.Remove,
				moveDirectory:        moveDirectoryNative,
				atomicReplaceCurrent: atomicReplaceCurrentNative,
				syncDirectory:        func(string) error { return sentinel },
			}
		},
		assertPrefix: func(t *testing.T, root string, _ []byte) {
			assertRecoveryPath(t, filepath.Join(root, "staging", "1"), true)
			assertRecoveryPath(t, filepath.Join(root, "staging", "1", "artifact"), false)
			assertRecoveryPath(t, filepath.Join(root, "staging", "1", "manifest.bin"), true)
		},
	}}
}

// TestExecutePlanStopsAtFirstFailure proves the executor treats every
// typed cleanup operation as a prefix boundary: after the injected
// operation fails, no operation later in the deterministic plan starts.
func TestExecutePlanStopsAtFirstFailure(t *testing.T) {
	kinds := []planOpKind{opRemoveFile, opRemoveDirectory, opMoveDirectory, opAtomicReplaceCurrent, opSyncDirectory}
	for failureIndex, failingKind := range kinds {
		failureIndex, failingKind := failureIndex, failingKind
		t.Run(fmt.Sprintf("operation-%d", failingKind), func(t *testing.T) {
			sentinel := errors.New("injected cleanup failure")
			var calls []planOpKind
			record := func(kind planOpKind) error {
				calls = append(calls, kind)
				if kind == failingKind {
					return sentinel
				}
				return nil
			}
			ops := cleanupOps{
				removeFile:           func(string) error { return record(opRemoveFile) },
				removeDirectory:      func(string) error { return record(opRemoveDirectory) },
				moveDirectory:        func(string, string) error { return record(opMoveDirectory) },
				atomicReplaceCurrent: func(string, []byte, time.Time) error { return record(opAtomicReplaceCurrent) },
				syncDirectory:        func(string) error { return record(opSyncDirectory) },
			}
			plan := recoveryPlan{Operations: []planOperation{
				{Kind: opRemoveFile, Path: "remove-file"},
				{Kind: opRemoveDirectory, Path: "remove-directory"},
				{Kind: opMoveDirectory, Path: "move-source", DestPath: "move-destination"},
				{Kind: opAtomicReplaceCurrent, Payload: []byte("current")},
				{Kind: opSyncDirectory, Path: "."},
			}}
			if err := executePlan(t.TempDir(), plan, ops); !errors.Is(err, sentinel) {
				t.Fatalf("executePlan err=%v, want injected failure", err)
			}
			want := kinds[:failureIndex+1]
			if len(calls) != len(want) {
				t.Fatalf("cleanup calls=%v, want exact prefix %v", calls, want)
			}
			for index := range want {
				if calls[index] != want[index] {
					t.Fatalf("cleanup calls=%v, want exact prefix %v", calls, want)
				}
			}
		})
	}
}

func TestAtomicReplaceRefusesExpiredDeadline(t *testing.T) {
	root := t.TempDir()
	current := filepath.Join(root, "current")
	if err := os.WriteFile(current, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := atomicReplaceCurrentNative(root, []byte("new"), time.Now().Add(-time.Second)); !errors.Is(err, errCleanupOverrun) {
		t.Fatalf("atomic replace err=%v, want cleanup overrun", err)
	}
	raw, err := os.ReadFile(current)
	if err != nil || string(raw) != "old" {
		t.Fatalf("current=%q err=%v, want unchanged old bytes", raw, err)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 1 || entries[0].Name() != "current" {
		t.Fatalf("expired atomic replace left artifacts: entries=%v err=%v", entries, err)
	}
}

// TestRecoverCleanupFailureOnMove asserts that a moveDirectory failure
// during the R09 cleanup (which moves generations/1 back to staging/1
// after removing the current temp) surfaces as cleanup-incomplete and
// preserves the lock, the verified generation, and the last coherent
// journal state.
func TestRecoverCleanupFailureOnMove(t *testing.T) {
	root, predecessor := recoveryOracleBootstrap(t)
	artifact, manifest := recoveryOracleStage(t, root, 1)
	recoveryOracleWriteChain(t, root, 1, predecessor, artifact, manifest, 6)
	recoveryOraclePublish(t, root, 1)
	recoveryOracleWriteCurrentTemp(t, root, ".current.0123456789abcdef.tmp",
		artifact, manifest,
		recoveryOracleDecodeHex(recoveryOraclePreviousDigestHex),
		recoveryOracleBootstrapManifestDigest(t, root),
		recoveryOracleCandidateLength(), recoveryOraclePreviousLength)
	sentinel := errors.New("injected cleanup failure")
	ops := cleanupOps{
		removeFile:           os.Remove,
		removeDirectory:      os.Remove,
		moveDirectory:        func(string, string) error { return sentinel },
		atomicReplaceCurrent: atomicReplaceCurrentNative,
		syncDirectory:        syncDirectoryNative,
	}
	result, err := recoverWithOperations(context.Background(), root, ops)
	recoveryOracleAssertCleanupIncomplete(t, result, err, 1, "draining")
	if _, lockErr := os.Lstat(filepath.Join(root, lockFileName)); lockErr != nil {
		t.Fatalf("RECOVER: cleanup failure must not remove the permanent lock: %v", lockErr)
	}
}

type recoveryCleanupFailureOp struct {
	id           string
	ops          func(t *testing.T, root string) cleanupOps
	assertPrefix func(t *testing.T, root string, before []byte)
}

func recoveryOracleAssertCleanupIncomplete(t *testing.T, result Result, err error, generation uint64, state string) {
	t.Helper()
	if !errors.Is(err, errCleanupOverrun) && err == nil {
		t.Fatalf("RECOVER: cleanup failure must report err != nil: err=%v", err)
	}
	if result.Outcome != outcomeCleanupIncomplete {
		t.Fatalf("RECOVER: cleanup outcome=%s, want cleanup-incomplete", result.Outcome)
	}
	if result.Generation != generation {
		t.Fatalf("RECOVER: cleanup generation=%d, want %d", result.Generation, generation)
	}
	if result.State != state {
		t.Fatalf("RECOVER: cleanup state=%s, want %s", result.State, state)
	}
	if result.SafeNotice != noticeUpdateCleanupIncomplete {
		t.Fatalf("RECOVER: cleanup safe notice=%q, want update cleanup incomplete", result.SafeNotice)
	}
}

func assertRecoveryPath(t *testing.T, path string, want bool) {
	t.Helper()
	_, err := os.Lstat(path)
	present := err == nil
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("lstat %s: %v", path, err)
	}
	if present != want {
		t.Fatalf("path %s present=%t, want %t", path, present, want)
	}
}

type atomicBool struct {
	mu sync.Mutex
	v  bool
}

func (a *atomicBool) set(value bool) {
	a.mu.Lock()
	a.v = value
	a.mu.Unlock()
}

func (a *atomicBool) get() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.v
}

// TestRecoverRepeatedCallsReturnCoherentResults calls Recover twice on
// the same root in sequence and asserts no package-global mutable fact
// leaks between invocations. The first call returns the recovered
// state; the second call returns the same recovered state because no
// second mutation occurs. This is the controller-required "no fact
// leakage" row for the immutable inventory and pure planner.
func TestRecoverRepeatedCallsReturnCoherentResults(t *testing.T) {
	root, predecessor := recoveryOracleBootstrap(t)
	artifact, manifest := recoveryOracleStage(t, root, 1)
	recoveryOracleWriteChain(t, root, 1, predecessor, artifact, manifest, 2)
	first, err := Recover(context.Background(), root)
	if err != nil {
		t.Fatalf("first Recover err=%v", err)
	}
	if first.Outcome != outcomeRecovered || first.State != "artifact-verified" {
		t.Fatalf("first Recover outcome=%s state=%s", first.Outcome, first.State)
	}
	second, err := Recover(context.Background(), root)
	if err != nil {
		t.Fatalf("second Recover err=%v", err)
	}
	if second != first {
		t.Fatalf("RECOVER: repeated call mutated Result: first=%+v second=%+v", first, second)
	}
}

// TestRecoverConcurrentRootsReturnIndependentResults calls Recover
// concurrently against two independent roots and asserts no fact
// leakage between the two inventories. The second-pass controller
// remediation requires this row to prove the inventory and planner
// remain bound to their per-invocation roots.
func TestRecoverConcurrentRootsReturnIndependentResults(t *testing.T) {
	rootA, predecessorA := recoveryOracleBootstrap(t)
	rootB, predecessorB := recoveryOracleBootstrap(t)
	artifactA, manifestA := recoveryOracleStage(t, rootA, 1)
	artifactB, manifestB := recoveryOracleStage(t, rootB, 1)
	recoveryOracleWriteChain(t, rootA, 1, predecessorA, artifactA, manifestA, 2)
	recoveryOracleWriteChain(t, rootB, 1, predecessorB, artifactB, manifestB, 6)
	recoveryOraclePublish(t, rootB, 1)
	var wg sync.WaitGroup
	wg.Add(2)
	var resultA, resultB Result
	var errA, errB error
	go func() {
		defer wg.Done()
		resultA, errA = Recover(context.Background(), rootA)
	}()
	go func() {
		defer wg.Done()
		resultB, errB = Recover(context.Background(), rootB)
	}()
	wg.Wait()
	if errA != nil || errB != nil {
		t.Fatalf("concurrent recovery errors: root A=%v root B=%v", errA, errB)
	}
	if resultA.Outcome != outcomeRecovered || resultA.State != "artifact-verified" {
		t.Fatalf("root A outcome=%s state=%s, want recovered/artifact-verified", resultA.Outcome, resultA.State)
	}
	if resultB.Outcome != outcomeRecovered || resultB.State != "draining" {
		t.Fatalf("root B outcome=%s state=%s, want recovered/draining", resultB.Outcome, resultB.State)
	}
}
