package update

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestRecoverBusyDoesNotMutate asserts that when one Apply holds the
// permanent lock via a blocked WorkControl, a concurrent Recover returns the
// frozen busy Result without mutating any owned tree byte.
//
// The blocked Apply must retain the permanent OS lock until the WorkControl
// is released. Recover may observe only the bounded busy Result and must not
// create, replace, or remove the lock while that owner is live.
func TestRecoverBusyDoesNotMutate(t *testing.T) {
	root := filepath.Join(t.TempDir(), "update")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	oracleBootstrapV0(t, root)
	vector := oracleLoadV0(t)
	candidate := oracleReadExact(t, oracleCandidatePath,
		vector.Candidate.Length, vector.Candidate.SHA256)
	decision := oracleAcceptedDecision(t, vector)
	request := Request{
		UpdateRoot: root,
		generation: vector.Request.TransactionGeneration,

		schemaPlan: vector.Request.SchemaPlan,
		decision:   decision,
		Artifact:   candidate,
		Work:       &recoveryOracleBlockingWork{signal: make(chan struct{}), entered: make(chan struct{})},
		SelfTest:   oraclePassSelfTest{},
	}
	done := make(chan struct{})
	var applyResult Result
	var applyErr error
	go func() {
		applyResult, applyErr = Apply(context.Background(), request)
		close(done)
	}()
	work := request.Work.(*recoveryOracleBlockingWork)
	select {
	case <-work.entered:
	case <-done:
		t.Fatalf("Apply completed before entering blocked work: result=%+v err=%v", applyResult, applyErr)
	case <-time.After(5 * time.Second):
		t.Fatal("Apply did not enter blocked work within the test deadline")
	}
	var release sync.Once
	t.Cleanup(func() {
		release.Do(func() { close(work.signal) })
		<-done
	})
	snapshot := recoveryOracleUnlockedTreeDigest(t, root)
	lockBefore, lockErr := os.Lstat(filepath.Join(root, lockFileName))
	if lockErr != nil || !lockBefore.Mode().IsRegular() || lockBefore.Size() != 0 {
		t.Fatalf("busy lock metadata before Recover: info=%v err=%v", lockBefore, lockErr)
	}
	result, err := Recover(context.Background(), root)
	if result.Outcome != "resource-denied" || result.State != "busy" {
		t.Fatalf("Recover busy outcome=%s state=%s, want resource-denied/busy (err=%v)", result.Outcome, result.State, err)
	}
	if result.Generation != 0 {
		t.Fatalf("Recover busy generation=%d, want 0", result.Generation)
	}
	if result.CurrentDigest != recoveryOracleZero || result.RollbackDigest != recoveryOracleZero {
		t.Fatalf("Recover busy digests not zero: current=%x rollback=%x", result.CurrentDigest, result.RollbackDigest)
	}
	if result.StagingPresent {
		t.Fatal("Recover busy staging must be false")
	}
	if result.SafeNotice != "update transaction busy" {
		t.Fatalf("Recover busy safe notice=%q, want update transaction busy", result.SafeNotice)
	}
	if err == nil {
		t.Fatal("Recover busy error must be non-nil")
	}
	if after := recoveryOracleUnlockedTreeDigest(t, root); !bytes.Equal(after, snapshot) {
		t.Fatalf("Recover busy mutated the tree: before=%x after=%x", snapshot, after)
	}
	lockAfter, lockErr := os.Lstat(filepath.Join(root, lockFileName))
	if lockErr != nil || !lockAfter.Mode().IsRegular() || lockAfter.Size() != 0 {
		t.Fatalf("busy lock metadata after Recover: info=%v err=%v", lockAfter, lockErr)
	}
	release.Do(func() { close(work.signal) })
	<-done
	if applyErr != nil {
		t.Fatalf("Apply failed after busy: %v", applyErr)
	}
	if applyResult.Outcome != "committed" {
		t.Fatalf("Apply result=%s, want committed", applyResult.Outcome)
	}
}

// recoveryOracleUnlockedTreeDigest hashes the tree without opening the
// permanent lock. A live Windows owner deliberately denies every sharing mode;
// lock existence and shape are asserted separately by the busy oracle.
func recoveryOracleUnlockedTreeDigest(t *testing.T, root string) []byte {
	t.Helper()
	hash := sha256.New()
	err := filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil || info.IsDir() {
			return walkErr
		}
		rel, err := filepath.Rel(root, path)
		if err != nil || rel == lockFileName {
			return err
		}
		hash.Write([]byte(rel))
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		hash.Write(data)
		return nil
	})
	if err != nil {
		t.Fatalf("FIXTURE: unlocked tree digest: %v", err)
	}
	return hash.Sum(nil)
}

// TestRecoverRejectsPermanentLockMutations asserts each accepted non-windows
// permanent-lock mutation returns transaction-invalid. The test is skipped
// on Windows where symlink and hard-link mutations require privilege.
func TestRecoverRejectsPermanentLockMutations(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("non-windows permanent-lock mutation coverage; windows uses TestRecoverRejectsPermanentLockWindowsShapes")
	}
	root, _, _, _ := recoveryOracleCorruptBootstrap(t)
	for _, mutation := range recoveryOracleLockMutationsNonWindows() {
		mutation := mutation
		t.Run(mutation.id, func(t *testing.T) {
			workRoot := recoveryOracleCorruptClone(t, root)
			defer os.RemoveAll(workRoot)
			mutation.apply(t, workRoot)
			result, err := Recover(context.Background(), workRoot)
			recoveryOracleAssertInvalid(t, result, err)
		})
	}
}

// TestRecoverRejectsPermanentLockWindowsShapes exercises the Windows-friendly
// lock mutations (directory lock, non-empty lock) and asserts each returns
// transaction-invalid. The current implementation has no lock-identity check,
// so it accepts every mutation and returns release-invalid; Gate B must add
// the identity validation.
func TestRecoverRejectsPermanentLockWindowsShapes(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows-only permanent-lock shape coverage")
	}
	root, _, _, _ := recoveryOracleCorruptBootstrap(t)
	for _, mutation := range recoveryOracleLockMutationsWindows() {
		mutation := mutation
		t.Run(mutation.id, func(t *testing.T) {
			workRoot := recoveryOracleCorruptClone(t, root)
			defer os.RemoveAll(workRoot)
			mutation.apply(t, workRoot)
			result, err := Recover(context.Background(), workRoot)
			recoveryOracleAssertInvalid(t, result, err)
		})
	}
}

// TestRecoverCleanupPlanAndResultTable is in recovery_cleanup_plan_test.go;
// it documents the frozen cleanup-plan table for R00/R03/R08-R11 plus the
// cleanup-overrun Result row. This file holds the live red lock/busy tests
// instead.

// TestRecoverR03RemovesUnacknowledgedStaging asserts the R03 cleanup
// normalization: the exact unacknowledged staging candidate is removed and
// no other owned byte changes. The chain binds the candidate artifact and
// in-memory candidate manifest. The current implementation does not remove
// the candidate, so the assertion must fail until Gate B implements the
// per-row cleanup.
func TestRecoverR03RemovesUnacknowledgedStaging(t *testing.T) {
	root, predecessor := recoveryOracleBootstrap(t)
	artifact, manifest := recoveryOracleStage(t, root, 1)
	recoveryOracleWriteChain(t, root, 1, predecessor, artifact, manifest, 2)
	result, err := Recover(context.Background(), root)
	if err != nil || result.Outcome != outcomeRecovered || result.State != "artifact-verified" {
		t.Fatalf("R03 recovery result=%+v err=%v", result, err)
	}
	if _, err := os.Lstat(filepath.Join(root, "staging", "1", "artifact")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("R03 must remove unacknowledged staging artifact: lstat=%v", err)
	}
	if _, err := os.Lstat(filepath.Join(root, "staging", "1", "manifest.bin")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("R03 must remove unacknowledged staging manifest: lstat=%v", err)
	}
	if _, err := os.Lstat(filepath.Join(root, "staging", "1")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("R03 must remove empty staging directory: lstat=%v", err)
	}
	if _, err := os.Lstat(filepath.Join(root, "transactions", "1", "journal", "01-release-accepted.entry")); err != nil {
		t.Fatalf("R03 must preserve entry 01: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(root, "transactions", "1", "journal", "02-artifact-verified.entry")); err != nil {
		t.Fatalf("R03 must preserve entry 02: %v", err)
	}
}

// recoveryOracleLockMutation is one permanent-lock mutation.
type recoveryOracleLockMutation struct {
	id    string
	apply func(t *testing.T, root string)
}

func recoveryOracleLockMutationsNonWindows() []recoveryOracleLockMutation {
	return []recoveryOracleLockMutation{{
		id: "missing-lock", apply: func(t *testing.T, root string) {
			if err := os.Remove(filepath.Join(root, ".ardents-update-transaction-lock")); err != nil && !errors.Is(err, os.ErrNotExist) {
				t.Fatal(err)
			}
		},
	}, {
		id: "symlink-lock", apply: func(t *testing.T, root string) {
			if err := os.Remove(filepath.Join(root, ".ardents-update-transaction-lock")); err != nil && !errors.Is(err, os.ErrNotExist) {
				t.Fatal(err)
			}
			if err := os.Symlink(filepath.Join(root, "current"), filepath.Join(root, ".ardents-update-transaction-lock")); err != nil {
				t.Fatal(err)
			}
		},
	}, {
		id: "hard-link-lock", apply: func(t *testing.T, root string) {
			if err := os.Remove(filepath.Join(root, ".ardents-update-transaction-lock")); err != nil && !errors.Is(err, os.ErrNotExist) {
				t.Fatal(err)
			}
			src := filepath.Join(filepath.Dir(root), "candidate-alias-source")
			if err := os.WriteFile(src, []byte("alias"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.Link(src, filepath.Join(root, ".ardents-update-transaction-lock")); err != nil {
				t.Fatal(err)
			}
		},
	}}
}

// recoveryOracleLockMutationsWindows returns Windows-friendly lock mutations.
func recoveryOracleLockMutationsWindows() []recoveryOracleLockMutation {
	return []recoveryOracleLockMutation{{
		id: "missing-lock", apply: func(t *testing.T, root string) {
			if err := os.Remove(filepath.Join(root, ".ardents-update-transaction-lock")); err != nil && !errors.Is(err, os.ErrNotExist) {
				t.Fatal(err)
			}
		},
	}, {
		id: "directory-lock", apply: func(t *testing.T, root string) {
			if err := os.Remove(filepath.Join(root, ".ardents-update-transaction-lock")); err != nil && !errors.Is(err, os.ErrNotExist) {
				t.Fatal(err)
			}
			if err := os.Mkdir(filepath.Join(root, ".ardents-update-transaction-lock"), 0o700); err != nil {
				t.Fatal(err)
			}
		},
	}, {
		id: "non-empty-lock", apply: func(t *testing.T, root string) {
			if err := os.WriteFile(filepath.Join(root, ".ardents-update-transaction-lock"), []byte("not-empty"), 0o600); err != nil {
				t.Fatal(err)
			}
		},
	}}
}

// recoveryOracleBlockingWork blocks Drain until the test releases the signal.
type recoveryOracleBlockingWork struct {
	signal  chan struct{}
	entered chan struct{}
	calls   atomic.Int64
}

func (work *recoveryOracleBlockingWork) StopNewWork(context.Context) error {
	work.calls.Add(1)
	return nil
}

func (work *recoveryOracleBlockingWork) Drain(context.Context) error {
	work.calls.Add(1)
	select {
	case <-work.entered:
	default:
		close(work.entered)
	}
	<-work.signal
	return nil
}

func (*recoveryOracleBlockingWork) StopNewAssignments(context.Context) error { return nil }
func (*recoveryOracleBlockingWork) DrainAssignments(context.Context) error   { return nil }
func (*recoveryOracleBlockingWork) RejoinOrWithdraw(context.Context) error   { return nil }
