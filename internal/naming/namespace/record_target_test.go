package namespace_test

import (
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/naming/namespace"
)

func TestPublishTargetRequiresMonotonicDistinctBinding(t *testing.T) {
	t.Parallel()
	policy := namespace.Policy{DefaultLeaseDuration: time.Hour, DefaultGraceDuration: time.Hour}
	claimed, err := namespace.ApplyLegacy(nil, 100, namespace.Op{Kind: "claim", Name: "alice",
		Generation: 1, Authority: "alice-authority"}, policy)
	if err != nil {
		t.Fatal(err)
	}
	if claimed.Target != ([32]byte{}) {
		t.Fatalf("claim created Target binding: %x", claimed.Target)
	}
	targetA := [32]byte{1}
	published, err := namespace.ApplyLegacy(&claimed, 101, namespace.Op{Kind: "publish", Name: "alice",
		ExpectedGeneration: 1, ExpectedRevision: 1, Authority: "alice-authority", Target: targetA,
		RecordNotAfter: 3_600_000}, policy)
	if err != nil {
		t.Fatal(err)
	}
	if published.Target != targetA || published.Generation != 1 || published.Revision != 2 {
		t.Fatalf("published = %+v", published)
	}
	if _, err := namespace.ApplyLegacy(&published, 102, namespace.Op{Kind: "publish", Name: "alice",
		ExpectedGeneration: 1, ExpectedRevision: 2, Authority: "alice-authority", Target: targetA,
		RecordNotAfter: 3_600_000}, policy); err == nil {
		t.Fatal("same-Target Service Instance migration created a Name Record revision")
	}
	targetB := [32]byte{2}
	replaced, err := namespace.ApplyLegacy(&published, 103, namespace.Op{Kind: "publish", Name: "alice",
		ExpectedGeneration: 1, ExpectedRevision: 2, Authority: "alice-authority", Target: targetB,
		RecordNotAfter: 3_600_000}, policy)
	if err != nil {
		t.Fatal(err)
	}
	if replaced.Target != targetB || replaced.Revision != 3 {
		t.Fatalf("replaced = %+v", replaced)
	}
	if _, err := namespace.ApplyLegacy(&published, 104, namespace.Op{Kind: "publish", Name: "alice",
		ExpectedGeneration: 1, ExpectedRevision: 2, Authority: "other", Target: targetB,
		RecordNotAfter: 3_600_000}, policy); err == nil {
		t.Fatal("different Name Authority replaced Target")
	}
}

func TestPublishRequiresFutureValidityInsideTheFullLeaseLineage(t *testing.T) {
	t.Parallel()
	policy := namespace.Policy{DefaultLeaseDuration: time.Hour, DefaultGraceDuration: time.Hour}
	root, err := namespace.ApplyLegacy(nil, 100, namespace.Op{Kind: "claim", Name: "root", Generation: 1,
		Authority: "root-authority", LeaseDuration: time.Minute}, policy)
	if err != nil {
		t.Fatal(err)
	}
	child, err := namespace.ApplyLegacy(nil, 101, namespace.Op{Kind: "claim", Name: "child.root", Generation: 1,
		Authority: "child-authority", Parents: []namespace.Record{root}}, policy)
	if err != nil {
		t.Fatal(err)
	}
	grandchild, err := namespace.ApplyLegacy(nil, 102, namespace.Op{Kind: "claim", Name: "grandchild.child.root", Generation: 1,
		Authority: "grandchild-authority", Parents: []namespace.Record{child, root}}, policy)
	if err != nil {
		t.Fatal(err)
	}
	base := namespace.Op{Kind: "publish", Name: grandchild.Name, Authority: grandchild.Authority,
		ExpectedGeneration: grandchild.Generation, ExpectedRevision: grandchild.Revision, Target: [32]byte{3},
		Parents: []namespace.Record{child, root}}
	if _, err := namespace.ApplyLegacy(&grandchild, 103, base, policy); err == nil {
		t.Fatal("publish without a signed Record validity was accepted")
	}
	past := base
	past.RecordNotAfter = 103_000
	if _, err := namespace.ApplyLegacy(&grandchild, 103, past, policy); err == nil {
		t.Fatal("publish at its decision boundary was accepted")
	}
	afterLineage := base
	afterLineage.RecordNotAfter = 161_000
	if _, err := namespace.ApplyLegacy(&grandchild, 103, afterLineage, policy); err == nil {
		t.Fatal("publish outlived the root Lease")
	}
	valid := base
	valid.RecordNotAfter = 160_000
	published, err := namespace.ApplyLegacy(&grandchild, 103, valid, policy)
	if err != nil || published.RecordNotAfter != valid.RecordNotAfter {
		t.Fatalf("published=%+v err=%v", published, err)
	}
}
