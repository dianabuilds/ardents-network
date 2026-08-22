package update

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestRecoverCleanupPlanAndResultTable exercises the literal frozen
// cleanup-plan and expected-Result table for R00, R03, R08, R09, R10, R11,
// plus the cleanup-overrun Result row. Each R row constructs the matching
// physical checkpoint, calls public Recover, and asserts the documented
// Result. The ordered remove/move/replace/sync prefixes are the literal
// plan Gate B will bind to its private per-invocation recovery-operation
// seam; Gate A does not drive the seam itself and does not fake the
// overrun. The cleanup-overrun row documents the exact Result the
// production code must return when a continuation-budget expiry is
// observed before completion.
//
// A real red public R00 assertion lives in TestRecoverInterruptionMatrix/R00
// and is the only place where the R00 Result is exercised end-to-end. This
// table additionally fixes the plan prefix and Result contract that Gate B
// must implement.
func TestRecoverCleanupPlanAndResultTable(t *testing.T) {
	for _, row := range frozenRecoveryCleanupTable() {
		row := row
		t.Run(row.id, func(t *testing.T) {
			root, predecessor := recoveryOracleBootstrap(t)
			row.setup(t, root, predecessor)
			result, err := Recover(context.Background(), root)
			recoveryOracleAssertCleanupRowResult(t, result, err, row.id, row.expected)
		})
	}
}

// recoveryOracleCleanupRow is one row of the frozen cleanup-plan table.
type recoveryOracleCleanupRow struct {
	id        string
	setup     func(t *testing.T, root string, predecessor [32]byte)
	plan      []string
	expected  Result
	lastState string
}

// frozenRecoveryCleanupTable returns the literal frozen cleanup-plan and
// expected-Result table for R00, R03, R08, R09, R10, R11 plus the
// cleanup-overrun Result. The plan slice is the exact ordered
// remove/move/replace/sync prefix Gate B must bind to its private
// per-invocation recovery-operation seam.
//
// Validation is pre-plan and is not a cleanup operation; therefore the
// R10/R11 envelope verification precedes the plan prefix below and is not
// part of the listed remove/move/replace/sync sequence. cleanup-incomplete
// rows preserve the verified transaction Generation (R00=0 because no
// entry, R03=1, R10=1) and the assertion uses row.expected.Generation,
// never zero.
func frozenRecoveryCleanupTable() []recoveryOracleCleanupRow {
	prevArtifact := recoveryOracleDecodeHex(recoveryOraclePreviousDigestHex)
	return []recoveryOracleCleanupRow{
		{
			id: "R00",
			setup: func(t *testing.T, root string, _ [32]byte) {
				if err := os.MkdirAll(filepath.Join(root, "transactions", "1", "journal"), 0o700); err != nil {
					t.Fatalf("FIXTURE: create empty R00 transaction journal: %v", err)
				}
			},
			plan: []string{
				"remove transactions/1/journal",
				"sync transactions/1",
				"remove transactions/1",
				"sync transactions",
			},
			expected: Result{
				Outcome:        "recovered",
				State:          "idle",
				Generation:     0,
				CurrentDigest:  prevArtifact,
				RollbackDigest: recoveryOracleZero,
				StagingPresent: false,
				SafeNotice:     "update interrupted",
				CustodyNotice:  recoveryOracleCustodyNotice,
			},
			lastState: "idle",
		},
		{
			id: "R03",
			setup: func(t *testing.T, root string, predecessor [32]byte) {
				artifact, manifest := recoveryOracleStage(t, root, 1)
				recoveryOracleWriteChain(t, root, 1, predecessor, artifact, manifest, 2)
			},
			plan: []string{
				"remove staging/1/artifact",
				"sync staging/1",
				"remove staging/1/manifest.bin",
				"sync staging/1",
				"remove staging/1",
				"sync staging",
			},
			expected: Result{
				Outcome:        "recovered",
				State:          "artifact-verified",
				Generation:     1,
				CurrentDigest:  prevArtifact,
				RollbackDigest: recoveryOracleZero,
				StagingPresent: false,
				SafeNotice:     "update interrupted",
				CustodyNotice:  recoveryOracleCustodyNotice,
			},
			lastState: "artifact-verified",
		},
		{
			id: "R08",
			setup: func(t *testing.T, root string, predecessor [32]byte) {
				artifact, manifest := recoveryOracleStage(t, root, 1)
				recoveryOracleWriteChain(t, root, 1, predecessor, artifact, manifest, 6)
				recoveryOraclePublish(t, root, 1)
			},
			plan: []string{
				"move generations/1 to staging/1",
				"sync generations",
				"sync staging",
			},
			expected: Result{
				Outcome:        "recovered",
				State:          "draining",
				Generation:     1,
				CurrentDigest:  prevArtifact,
				RollbackDigest: recoveryOracleZero,
				StagingPresent: true,
				SafeNotice:     "update interrupted",
				CustodyNotice:  recoveryOracleCustodyNotice,
			},
			lastState: "draining",
		},
		{
			id: "R09",
			setup: func(t *testing.T, root string, predecessor [32]byte) {
				artifact, manifest := recoveryOracleStage(t, root, 1)
				recoveryOracleWriteChain(t, root, 1, predecessor, artifact, manifest, 6)
				recoveryOraclePublish(t, root, 1)
				length := recoveryOracleCandidateLength()
				recoveryOracleWriteCurrentTemp(t, root, ".current.0123456789abcdef.tmp",
					artifact, manifest, prevArtifact,
					recoveryOracleBootstrapManifestDigest(t, root), length, recoveryOraclePreviousLength)
			},
			plan: []string{
				"remove .current.0123456789abcdef.tmp",
				"sync root",
				"move generations/1 to staging/1",
				"sync generations",
				"sync staging",
			},
			expected: Result{
				Outcome:        "recovered",
				State:          "draining",
				Generation:     1,
				CurrentDigest:  prevArtifact,
				RollbackDigest: recoveryOracleZero,
				StagingPresent: true,
				SafeNotice:     "update interrupted",
				CustodyNotice:  recoveryOracleCustodyNotice,
			},
			lastState: "draining",
		},
		{
			id: "R10",
			setup: func(t *testing.T, root string, predecessor [32]byte) {
				artifact, manifest := recoveryOracleStage(t, root, 1)
				recoveryOracleWriteChain(t, root, 1, predecessor, artifact, manifest, 6)
				recoveryOraclePublish(t, root, 1)
				length := recoveryOracleCandidateLength()
				recoveryOracleSuccessorCurrent(t, root, 1, artifact, manifest, prevArtifact,
					recoveryOracleBootstrapManifestDigest(t, root), length, recoveryOraclePreviousLength)
			},
			plan: []string{
				"atomic replace current with canonical predecessor (selected=0, no rollback)",
				"sync root",
				"move generations/1 to staging/1",
				"sync generations",
				"sync staging",
			},
			expected: Result{
				Outcome:        "recovered",
				State:          "draining",
				Generation:     1,
				CurrentDigest:  prevArtifact,
				RollbackDigest: recoveryOracleZero,
				StagingPresent: true,
				SafeNotice:     "update interrupted",
				CustodyNotice:  recoveryOracleCustodyNotice,
			},
			lastState: "draining",
		},
		{
			id: "R11",
			setup: func(t *testing.T, root string, predecessor [32]byte) {
				artifact, manifest := recoveryOracleStage(t, root, 1)
				recoveryOracleWriteChain(t, root, 1, predecessor, artifact, manifest, 6)
				recoveryOraclePublish(t, root, 1)
				length := recoveryOracleCandidateLength()
				recoveryOracleSuccessorCurrent(t, root, 1, artifact, manifest, prevArtifact,
					recoveryOracleBootstrapManifestDigest(t, root), length, recoveryOraclePreviousLength)
			},
			plan: []string{
				"atomic replace current with canonical predecessor (selected=0, no rollback)",
				"sync root",
				"move generations/1 to staging/1",
				"sync generations",
				"sync staging",
			},
			expected: Result{
				Outcome:        "recovered",
				State:          "draining",
				Generation:     1,
				CurrentDigest:  prevArtifact,
				RollbackDigest: recoveryOracleZero,
				StagingPresent: true,
				SafeNotice:     "update interrupted",
				CustodyNotice:  recoveryOracleCustodyNotice,
			},
			lastState: "draining",
		},
		{
			id: "cleanup-overrun-R00",
			setup: func(t *testing.T, _ string, _ [32]byte) {
				t.Skip("cleanup-overrun contract: Gate B binds the private seam; no public Recover driver in Gate A")
			},
			plan: []string{
				"check 5s continuation budget before remove transactions/1/journal",
				"remove transactions/1/journal returns ctx.Err() = DeadlineExceeded",
				"observe overrun, return cleanup-incomplete; no later operation starts",
			},
			expected: Result{
				Outcome:    "cleanup-incomplete",
				State:      "idle",
				Generation: 0,
				SafeNotice: "update cleanup incomplete",
			},
			lastState: "idle",
		},
		{
			id: "cleanup-overrun-R03",
			setup: func(t *testing.T, _ string, _ [32]byte) {
				t.Skip("cleanup-overrun contract: Gate B binds the private seam; no public Recover driver in Gate A")
			},
			plan: []string{
				"check 5s continuation budget before remove staging/1/artifact",
				"remove staging/1/artifact returns ctx.Err() = DeadlineExceeded",
				"observe overrun, return cleanup-incomplete with verified generation=1 and last coherent journal state (artifact-verified)",
			},
			expected: Result{
				Outcome:    "cleanup-incomplete",
				State:      "artifact-verified",
				Generation: 1,
				SafeNotice: "update cleanup incomplete",
			},
			lastState: "artifact-verified",
		},
		{
			id: "cleanup-overrun-R10",
			setup: func(t *testing.T, _ string, _ [32]byte) {
				t.Skip("cleanup-overrun contract: Gate B binds the private seam; no public Recover driver in Gate A")
			},
			plan: []string{
				"check 5s continuation budget before atomic replace current",
				"atomic replace current returns DeadlineExceeded after 5s",
				"observe overrun, return cleanup-incomplete with verified generation=1 and last coherent journal state (draining)",
				"no further move generations/1 to staging/1 starts",
			},
			expected: Result{
				Outcome:    "cleanup-incomplete",
				State:      "draining",
				Generation: 1,
				SafeNotice: "update cleanup incomplete",
			},
			lastState: "draining",
		},
	}
}

// recoveryOracleAssertCleanupRowResult asserts the public Recover Result
// matches the documented table entry for the named row. Success rows
// (recovered/committed) require err == nil; cleanup-incomplete rows
// require err != nil. The plan prefix is not observed through public
// Recover; Gate B's private seam will enforce it. cleanup-incomplete
// rows preserve the verified transaction Generation; the assertion reads
// row.expected.Generation (R00=0, R03=1, R10=1) rather than a hardcoded
// zero. The expected physical fields remain zero digests, no staging, and
// no custody notice.
func recoveryOracleAssertCleanupRowResult(t *testing.T, result Result, err error, id string, expected Result) {
	t.Helper()
	if expected.Outcome == "cleanup-incomplete" {
		if err == nil {
			t.Fatalf("RECOVER: %s cleanup-incomplete must have err != nil, got nil", id)
		}
		if result.Outcome != "cleanup-incomplete" {
			t.Fatalf("RECOVER: %s outcome=%s state=%s, want cleanup-incomplete", id, result.Outcome, result.State)
		}
		if result.State != expected.State {
			t.Fatalf("RECOVER: %s state=%s, want %s", id, result.State, expected.State)
		}
		if result.Generation != expected.Generation {
			t.Fatalf("RECOVER: %s generation=%d, want %d (verified transaction generation must be preserved)", id, result.Generation, expected.Generation)
		}
		if result.CurrentDigest != recoveryOracleZero || result.RollbackDigest != recoveryOracleZero {
			t.Fatalf("RECOVER: %s digests not zero: current=%x rollback=%x", id, result.CurrentDigest, result.RollbackDigest)
		}
		if result.StagingPresent {
			t.Fatalf("RECOVER: %s staging must be false", id)
		}
		if result.CustodyNotice != "" {
			t.Fatalf("RECOVER: %s custody=%q, want empty", id, result.CustodyNotice)
		}
		if result.SafeNotice != "update cleanup incomplete" {
			t.Fatalf("RECOVER: %s safe notice=%q, want update cleanup incomplete", id, result.SafeNotice)
		}
		return
	}
	if err != nil {
		t.Fatalf("RECOVER: %s success Result must have err == nil: err=%v", id, err)
	}
	if result.Outcome != expected.Outcome || result.State != expected.State {
		t.Fatalf("RECOVER: %s outcome=%s state=%s, want %s/%s", id, result.Outcome, result.State, expected.Outcome, expected.State)
	}
	if result.Generation != expected.Generation {
		t.Fatalf("RECOVER: %s generation=%d, want %d", id, result.Generation, expected.Generation)
	}
	if result.CurrentDigest != expected.CurrentDigest {
		t.Fatalf("RECOVER: %s current digest=%x, want %x", id, result.CurrentDigest, expected.CurrentDigest)
	}
	if result.RollbackDigest != expected.RollbackDigest {
		t.Fatalf("RECOVER: %s rollback digest=%x, want %x", id, result.RollbackDigest, expected.RollbackDigest)
	}
	if result.StagingPresent != expected.StagingPresent {
		t.Fatalf("RECOVER: %s staging=%v, want %v", id, result.StagingPresent, expected.StagingPresent)
	}
	if result.SafeNotice != expected.SafeNotice {
		t.Fatalf("RECOVER: %s safe notice=%q, want %q", id, result.SafeNotice, expected.SafeNotice)
	}
	if result.CustodyNotice != expected.CustodyNotice {
		t.Fatalf("RECOVER: %s custody=%q, want %q", id, result.CustodyNotice, expected.CustodyNotice)
	}
}
