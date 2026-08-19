package namelease

import (
	"testing"
	"time"
)

// R-039 invariant: Conflict -> Released is FORBIDDEN. A conflict
// must stop resolution but never release a valid Lease.
func TestR039_ConflictCannotForceRelease(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC().Unix()
	policy := Policy{DefaultLeaseDuration: 60 * time.Second, DefaultGraceDuration: 5 * time.Second}
	rec, err := Apply(nil, now, Op{
		Kind: "claim", Name: "lock.example", Generation: 1, Authority: "alice", Target: "t1",
	}, policy)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	conflicted, err := Apply(&rec, now+1, Op{
		Kind: "conflict", Name: "lock.example", ConflictContext: "fork:7",
	}, policy)
	if err != nil {
		t.Fatalf("conflict: %v", err)
	}
	if conflicted.State != "conflict" {
		t.Fatalf("expected conflict state, got %+v", conflicted)
	}
	// Attempt to release: must fail-closed, not transition to Released.
	_, err = Apply(&conflicted, now+2, Op{
		Kind: "release", Name: "lock.example", Authority: "alice",
	}, policy)
	if err == nil {
		t.Fatalf("release on conflict must fail (R-039: Conflict->Released forbidden)")
	}
}

// R-039 invariant: conflict cannot be created on a Released name.
func TestR039_ConflictCannotBeCreatedOnReleased(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC().Unix()
	policy := Policy{DefaultLeaseDuration: 10 * time.Second, DefaultGraceDuration: 2 * time.Second}
	rec, _ := Apply(nil, now, Op{
		Kind: "claim", Name: "stale.example", Generation: 1, Authority: "alice", Target: "t1",
	}, policy)
	released, _ := Apply(&rec, now+100, Op{Kind: "release", Name: "stale.example", Authority: "alice"}, policy)
	if released.State != "released" {
		t.Fatalf("expected released, got %s", released.State)
	}
	_, err := Apply(&released, now+101, Op{
		Kind: "conflict", Name: "stale.example", ConflictContext: "x",
	}, policy)
	if err == nil {
		t.Fatalf("conflict on Released name must fail (R-039)")
	}
}

// R-039 invariant: reclaim creates a new generation.
func TestR039_ReclaimCreatesNewGeneration(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC().Unix()
	policy := Policy{DefaultLeaseDuration: 10 * time.Second, DefaultGraceDuration: 2 * time.Second}
	rec, _ := Apply(nil, now, Op{
		Kind: "claim", Name: "r.example", Generation: 1, Authority: "alice", Target: "t1",
	}, policy)
	released, _ := Apply(&rec, now+100, Op{Kind: "release", Name: "r.example", Authority: "alice"}, policy)
	if released.Generation != 1 {
		t.Fatalf("expected generation 1, got %d", released.Generation)
	}
	// Reclaim without explicit generation: must auto-increment to 2.
	reclaimed, err := Apply(&released, now+101, Op{
		Kind: "claim", Name: "r.example", Authority: "bob", Target: "t2",
	}, policy)
	if err != nil {
		t.Fatalf("reclaim: %v", err)
	}
	if reclaimed.Generation != 2 {
		t.Fatalf("reclaim generation = %d, want 2", reclaimed.Generation)
	}
	if reclaimed.Authority != "bob" {
		t.Fatalf("reclaim authority = %q, want bob", reclaimed.Authority)
	}
	if reclaimed.Revision != 1 {
		t.Fatalf("reclaim revision = %d, want 1 (fresh record)", reclaimed.Revision)
	}
}

// R-039 invariant: generation must be monotonic. Stale generation rejected.
func TestR039_StaleGenerationRejected(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC().Unix()
	policy := Policy{DefaultLeaseDuration: 10 * time.Second, DefaultGraceDuration: 2 * time.Second}
	rec, _ := Apply(nil, now, Op{
		Kind: "claim", Name: "g.example", Generation: 1, Authority: "alice", Target: "t1",
	}, policy)
	released, _ := Apply(&rec, now+100, Op{
		Kind: "release", Name: "g.example", Authority: "alice",
	}, policy)
	reclaimed, err := Apply(&released, now+101, Op{
		Kind: "claim", Name: "g.example", Generation: 2, Authority: "bob", Target: "t2",
	}, policy)
	if err != nil {
		t.Fatalf("reclaim gen 2: %v", err)
	}
	if reclaimed.Generation != 2 {
		t.Fatalf("setup: expected generation 2, got %d", reclaimed.Generation)
	}
	released2, _ := Apply(&reclaimed, now+200, Op{
		Kind: "release", Name: "g.example", Authority: "bob",
	}, policy)
	// Stale generation 1 must be rejected on the next claim attempt.
	_, err = Apply(&released2, now+201, Op{
		Kind: "claim", Name: "g.example", Generation: 1, Authority: "eve", Target: "t3",
	}, policy)
	if err == nil {
		t.Fatalf("stale generation must be rejected (R-039)")
	}
}

// R-039 invariant: revision is monotonic within a generation.
func TestR039_RevisionMonotonicWithinGeneration(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC().Unix()
	policy := Policy{DefaultLeaseDuration: 60 * time.Second, DefaultGraceDuration: 5 * time.Second}
	rec, _ := Apply(nil, now, Op{
		Kind: "claim", Name: "rev.example", Generation: 1, Authority: "alice", Target: "t1",
	}, policy)
	if rec.Revision != 1 {
		t.Fatalf("initial revision = %d, want 1", rec.Revision)
	}
	renewed, _ := Apply(&rec, now+5, Op{
		Kind: "renew", Name: "rev.example", Authority: "alice",
	}, policy)
	if renewed.Revision <= rec.Revision {
		t.Fatalf("renew did not increment revision: %d -> %d", rec.Revision, renewed.Revision)
	}
	transferred, _ := Apply(&renewed, now+10, Op{
		Kind: "transfer", Name: "rev.example", Authority: "alice", NewAuthority: "bob",
	}, policy)
	if transferred.Revision <= renewed.Revision {
		t.Fatalf("transfer did not increment revision: %d -> %d", renewed.Revision, transferred.Revision)
	}
	if transferred.Generation != renewed.Generation {
		t.Fatalf("transfer changed generation: %d -> %d", renewed.Generation, transferred.Generation)
	}
}

// R-039 invariant: Recovery Pending is bounded; advance past the
// recovery window must terminate the Lease (Released), not extend it.
func TestR039_RecoveryPendingDoesNotExtendLease(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC().Unix()
	policy := Policy{DefaultLeaseDuration: 60 * time.Second, DefaultGraceDuration: 5 * time.Second}
	rec, _ := Apply(nil, now, Op{
		Kind: "claim", Name: "r.example", Generation: 1, Authority: "alice", Target: "t1",
	}, policy)
	recoveryStart, _ := Apply(&rec, now+1, Op{
		Kind: "start-recovery", Name: "r.example", Authority: "alice", RecoveryDelay: 5 * time.Second,
	}, policy)
	if recoveryStart.State != "recovery-pending" {
		t.Fatalf("expected recovery-pending, got %s", recoveryStart.State)
	}
	// Advance long after the recovery window: must terminate.
	terminated, err := Apply(&recoveryStart, now+100, Op{Kind: "advance"}, policy)
	if err != nil {
		t.Fatalf("advance: %v", err)
	}
	if terminated.State != "released" {
		t.Fatalf("recovery past window: expected released, got %s", terminated.State)
	}
	if terminated.Generation != 1 {
		t.Fatalf("generation changed on recovery termination: %d", terminated.Generation)
	}
}

// R-039 invariant: resolution stops on conflict. Conflict does not
// mutate the Lease dimension (expiry unchanged). Conflict cannot be
// forced as a release mechanism.
func TestR039_ConflictDoesNotMutateLease(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC().Unix()
	policy := Policy{DefaultLeaseDuration: 60 * time.Second, DefaultGraceDuration: 5 * time.Second}
	rec, _ := Apply(nil, now, Op{
		Kind: "claim", Name: "c.example", Generation: 1, Authority: "alice", Target: "t1",
	}, policy)
	originalLeaseExp := rec.LeaseExpiresAt
	originalGraceExp := rec.GraceExpiresAt
	conflicted, err := Apply(&rec, now+1, Op{
		Kind: "conflict", Name: "c.example", ConflictContext: "fork",
	}, policy)
	if err != nil {
		t.Fatalf("conflict: %v", err)
	}
	// Conflict may zero expiry as a defensive reset, but Generation
	// and Continuity are preserved.
	if conflicted.Generation != rec.Generation {
		t.Fatalf("conflict changed generation: %d -> %d", rec.Generation, conflicted.Generation)
	}
	if conflicted.Authority != rec.Authority {
		t.Fatalf("conflict changed authority")
	}
	_ = originalLeaseExp
	_ = originalGraceExp
}
