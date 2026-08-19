package namelease

import (
	"testing"
	"time"
)

// TestR039_ChildClaimPreservesParentName per R-039 § Fixed product
// contract: a child Name Record carries the parent Name. The parent
// binding is part of the canonical Record and must survive subsequent
// transitions within the same generation.
func TestR039_ChildClaimPreservesParentName(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC().Unix()
	policy := Policy{DefaultLeaseDuration: 60 * time.Second, DefaultGraceDuration: 5 * time.Second}
	rec, err := Apply(nil, now, Op{
		Kind: "claim", Name: "blog.alice", Generation: 1, Authority: "alice",
		Target: "t1", ParentName: "alice",
	}, policy)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if rec.ParentName != "alice" {
		t.Fatalf("parent name = %q, want alice", rec.ParentName)
	}
	// Renew within the same generation must not change ParentName.
	renewed, err := Apply(&rec, now+10, Op{
		Kind: "renew", Name: "blog.alice", Authority: "alice", Target: "t1",
	}, policy)
	if err != nil {
		t.Fatalf("renew: %v", err)
	}
	if renewed.ParentName != "alice" {
		t.Fatalf("renewed parent name = %q, want alice", renewed.ParentName)
	}
	if renewed.Generation != rec.Generation {
		t.Fatalf("renewed generation = %d, want %d", renewed.Generation, rec.Generation)
	}
}

// TestR039_ReleaseDoesNotReviveParent per S6.1 DoD: a Release on a
// child does not let an adversary reclaim the parent to revive
// the child lineage. Reclaim is a new generation.
func TestR039_ReleaseDoesNotReviveParent(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC().Unix()
	policy := Policy{DefaultLeaseDuration: 60 * time.Second, DefaultGraceDuration: 5 * time.Second}
	rec, _ := Apply(nil, now, Op{
		Kind: "claim", Name: "blog.alice", Generation: 1, Authority: "alice",
		Target: "t1", ParentName: "alice",
	}, policy)
	released, _ := Apply(&rec, now+100, Op{
		Kind: "release", Name: "blog.alice", Authority: "alice",
	}, policy)
	if released.State != "released" {
		t.Fatalf("expected released, got %s", released.State)
	}
	// Reclaim creates a new generation, not a revival of generation 1.
	reclaimed, err := Apply(&released, now+101, Op{
		Kind: "claim", Name: "blog.alice", Authority: "bob", Target: "t2",
	}, policy)
	if err != nil {
		t.Fatalf("reclaim: %v", err)
	}
	if reclaimed.Generation <= 1 {
		t.Fatalf("reclaim must produce a new generation, got %d", reclaimed.Generation)
	}
	if reclaimed.Authority == "alice" {
		t.Fatalf("reclaim must require a different authority (alice was released)")
	}
}

// TestR039_TransferPreservesParentName per R-039: authority transfer
// is a successor transition within the same generation. It must not
// change the parent binding.
func TestR039_TransferPreservesParentName(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC().Unix()
	policy := Policy{DefaultLeaseDuration: 60 * time.Second, DefaultGraceDuration: 5 * time.Second}
	rec, _ := Apply(nil, now, Op{
		Kind: "claim", Name: "blog.alice", Generation: 1, Authority: "alice",
		Target: "t1", ParentName: "alice",
	}, policy)
	transferred, err := Apply(&rec, now+5, Op{
		Kind: "transfer", Name: "blog.alice", Authority: "alice", NewAuthority: "bob",
	}, policy)
	if err != nil {
		t.Fatalf("transfer: %v", err)
	}
	if transferred.ParentName != "alice" {
		t.Fatalf("transfer changed parent name: %q", transferred.ParentName)
	}
	if transferred.Generation != rec.Generation {
		t.Fatalf("transfer changed generation: %d -> %d", rec.Generation, transferred.Generation)
	}
	if transferred.Authority != "bob" {
		t.Fatalf("transfer did not set new authority: %q", transferred.Authority)
	}
}
