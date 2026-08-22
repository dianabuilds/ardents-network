package namespace

import "testing"

func TestChildBindsToLiveParentGeneration(t *testing.T) {
	t.Parallel()
	parent := claimRoot(t, "alice", "alice-key", 100)
	child := applyOK(t, nil, 101, Op{Kind: "claim", Name: "blog.alice", Generation: 1,
		Authority: "chosen-child-key", Parents: []Record{parent}})
	if child.ParentName != parent.Name || child.ParentGeneration != parent.Generation {
		t.Fatalf("parent binding missing: %+v", child)
	}
	if child.LeaseExpiresAt > parent.LeaseExpiresAt || child.GraceExpiresAt > parent.GraceExpiresAt {
		t.Fatalf("child outlives parent: child=%+v parent=%+v", child, parent)
	}
	if ok, _ := canResolve(child, 102, []Record{parent}); ok {
		t.Fatal("unbound child resolved without a Service Target")
	}
	bound := child
	bound.Target = [32]byte{4}
	if ok, reason := canResolve(bound, 102, []Record{parent}); !ok {
		t.Fatalf("valid child did not resolve: %s", reason)
	}
}

func TestChildFailsAfterParentReleaseOrReclaim(t *testing.T) {
	t.Parallel()
	parent := claimRoot(t, "alice", "alice-key", 100)
	child := applyOK(t, nil, 101, Op{Kind: "claim", Name: "blog.alice", Generation: 1,
		Authority: "chosen-child-key", Parents: []Record{parent}})
	released := applyOK(t, &parent, 102, Op{Kind: "release", Name: parent.Name,
		Authority: parent.Authority, ExpectedGeneration: parent.Generation, ExpectedRevision: parent.Revision})
	if ok, _ := canResolve(child, 102, []Record{released}); ok {
		t.Fatal("child survived parent release")
	}
	reclaimed := applyOK(t, &released, 103, Op{Kind: "claim", Name: parent.Name, Generation: 2,
		Authority: "new-key", ExpectedGeneration: 1, ExpectedRevision: 2})
	if ok, _ := canResolve(child, 103, []Record{reclaimed}); ok {
		t.Fatal("old child revived under reclaimed parent generation")
	}
}

func TestChildRejectsMissingOrWrongLineage(t *testing.T) {
	t.Parallel()
	parent := claimRoot(t, "alice", "alice-key", 100)
	for _, op := range []Op{
		{Kind: "claim", Name: "blog.alice", Generation: 1, Authority: "alice-key"},
		{Kind: "claim", Name: "blog.bob", Generation: 1, Authority: "alice-key", Parents: []Record{parent}},
	} {
		_, err := Apply(nil, 101, op, testPolicy)
		if err == nil {
			t.Fatalf("accepted invalid lineage: %+v", op)
		}
	}
}

func TestParentMayChooseDistinctChildAuthority(t *testing.T) {
	t.Parallel()
	parent := claimRoot(t, "alice", "parent-key", 100)
	child := applyOK(t, nil, 101, Op{Kind: "claim", Name: "blog.alice", Generation: 1,
		Authority: "chosen-child-key", Parents: []Record{parent}})
	if child.Authority != "chosen-child-key" || child.Authority == parent.Authority {
		t.Fatalf("child authority was not preserved: %+v", child)
	}
}
