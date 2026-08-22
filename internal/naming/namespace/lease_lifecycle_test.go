package namespace

import (
	"testing"
	"time"
)

var testPolicy = Policy{DefaultLeaseDuration: 12 * time.Second, DefaultGraceDuration: 4 * time.Second}

func TestLeaseLifecycleRequiresExactGenerationAndRevision(t *testing.T) {
	t.Parallel()
	claimed := claimRoot(t, "site", "alice", 100)
	if claimed.Target != ([32]byte{}) {
		t.Fatalf("initial Lease claim bound a Service Target: %+v", claimed)
	}
	if ok, _ := canResolve(claimed, 100, nil); ok {
		t.Fatal("unbound initial Lease resolved without a Service Target")
	}

	for _, op := range []Op{
		{Kind: "renew", Name: claimed.Name, Authority: "alice", ExpectedGeneration: 2, ExpectedRevision: 1},
		{Kind: "renew", Name: claimed.Name, Authority: "alice", ExpectedGeneration: 1, ExpectedRevision: 0},
		{Kind: "release", Name: claimed.Name, Authority: "alice", ExpectedGeneration: 0, ExpectedRevision: 0},
	} {
		if _, err := Apply(&claimed, 101, op, testPolicy); err == nil {
			t.Fatalf("accepted stale operation: %+v", op)
		}
	}

	renewed := applyOK(t, &claimed, 105, Op{Kind: "renew", Name: claimed.Name,
		Authority: "alice", ExpectedGeneration: 1, ExpectedRevision: 1})
	if renewed.Lease != leaseActive || renewed.Revision != 2 {
		t.Fatalf("unexpected renewal: %+v", renewed)
	}
	grace := applyOK(t, &renewed, 118, exactOp("advance", renewed))
	if grace.Lease != leaseGrace || grace.Revision != 3 {
		t.Fatalf("unexpected grace transition: %+v", grace)
	}
	released := applyOK(t, &grace, 122, exactOp("advance", grace))
	if released.Lease != leaseReleased || released.Revision != 4 {
		t.Fatalf("unexpected release transition: %+v", released)
	}
}

func TestReclaimCreatesExactNextGeneration(t *testing.T) {
	t.Parallel()
	claimed := claimRoot(t, "site", "alice", 100)
	released := applyOK(t, &claimed, 101, Op{Kind: "release", Name: claimed.Name,
		Authority: "alice", ExpectedGeneration: 1, ExpectedRevision: 1})

	stale := Op{Kind: "claim", Name: released.Name, Generation: 1, Authority: "bob",
		ExpectedGeneration: released.Generation, ExpectedRevision: released.Revision}
	if _, err := Apply(&released, 102, stale, testPolicy); err == nil {
		t.Fatal("reclaim accepted an old generation")
	}
	stale.Generation = 2
	reclaimed := applyOK(t, &released, 102, stale)
	if reclaimed.Generation != 2 || reclaimed.Revision != 1 || reclaimed.Continuity != 2 {
		t.Fatalf("unexpected reclaim: %+v", reclaimed)
	}
}

func TestConflictIsOrthogonalToLease(t *testing.T) {
	t.Parallel()
	claimed := claimRoot(t, "site", "alice", 100)
	conflicted := applyOK(t, &claimed, 101, Op{Kind: "conflict", Name: claimed.Name,
		ExpectedGeneration: 1, ExpectedRevision: 1, ConflictContext: "fork:7"})
	if conflicted.Lease != claimed.Lease || conflicted.LeaseExpiresAt != claimed.LeaseExpiresAt ||
		conflicted.GraceExpiresAt != claimed.GraceExpiresAt || conflicted.Recovery != claimed.Recovery {
		t.Fatalf("conflict mutated another state dimension: before=%+v after=%+v", claimed, conflicted)
	}
	if conflicted.Consistency != consistencyConflict {
		t.Fatalf("consistency = %q", conflicted.Consistency)
	}
	if ok, _ := canResolve(conflicted, 101, nil); ok {
		t.Fatal("conflicted record resolved")
	}
	if _, err := Apply(&conflicted, 102, Op{Kind: "release", Name: conflicted.Name,
		Authority: "alice", ExpectedGeneration: 1, ExpectedRevision: 2}, testPolicy); err == nil {
		t.Fatal("conflict forced a release")
	}
}

func TestUndecidedOperationsAreUnavailable(t *testing.T) {
	t.Parallel()
	claimed := claimRoot(t, "site", "alice", 100)
	for _, kind := range []string{"transfer", "start-recovery", "install-successor", "resolve-conflict"} {
		op := exactOp(kind, claimed)
		if _, err := Apply(&claimed, 101, op, testPolicy); err == nil {
			t.Fatalf("S6.1 accepted undecided operation %q", kind)
		}
	}
}

func claimRoot(t *testing.T, name, authority string, now int64) Record {
	t.Helper()
	return applyOK(t, nil, now, Op{Kind: "claim", Name: name, Generation: 1,
		Authority: authority})
}

func exactOp(kind string, record Record) Op {
	return Op{Kind: kind, Name: record.Name, ExpectedGeneration: record.Generation,
		ExpectedRevision: record.Revision}
}

func applyOK(t *testing.T, current *Record, now int64, op Op) Record {
	t.Helper()
	record, err := Apply(current, now, op, testPolicy)
	if err != nil {
		t.Fatalf("Apply(%s): %v", op.Kind, err)
	}
	return record
}
