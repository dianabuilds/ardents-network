package namespace_test

import (
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/naming/namespace"
)

func TestPublishTargetRequiresMonotonicDistinctBinding(t *testing.T) {
	t.Parallel()
	policy := namespace.Policy{DefaultLeaseDuration: time.Hour, DefaultGraceDuration: time.Hour}
	claimed, err := namespace.Apply(nil, 100, namespace.Op{Kind: "claim", Name: "alice",
		Generation: 1, Authority: "alice-authority"}, policy)
	if err != nil {
		t.Fatal(err)
	}
	if claimed.Target != ([32]byte{}) {
		t.Fatalf("claim created Target binding: %x", claimed.Target)
	}
	targetA := [32]byte{1}
	published, err := namespace.Apply(&claimed, 101, namespace.Op{Kind: "publish", Name: "alice",
		ExpectedGeneration: 1, ExpectedRevision: 1, Authority: "alice-authority", Target: targetA}, policy)
	if err != nil {
		t.Fatal(err)
	}
	if published.Target != targetA || published.Generation != 1 || published.Revision != 2 {
		t.Fatalf("published = %+v", published)
	}
	if _, err := namespace.Apply(&published, 102, namespace.Op{Kind: "publish", Name: "alice",
		ExpectedGeneration: 1, ExpectedRevision: 2, Authority: "alice-authority", Target: targetA}, policy); err == nil {
		t.Fatal("same-Target Service Instance migration created a Name Record revision")
	}
	targetB := [32]byte{2}
	replaced, err := namespace.Apply(&published, 103, namespace.Op{Kind: "publish", Name: "alice",
		ExpectedGeneration: 1, ExpectedRevision: 2, Authority: "alice-authority", Target: targetB}, policy)
	if err != nil {
		t.Fatal(err)
	}
	if replaced.Target != targetB || replaced.Revision != 3 {
		t.Fatalf("replaced = %+v", replaced)
	}
	if _, err := namespace.Apply(&published, 104, namespace.Op{Kind: "publish", Name: "alice",
		ExpectedGeneration: 1, ExpectedRevision: 2, Authority: "other", Target: targetB}, policy); err == nil {
		t.Fatal("different Name Authority replaced Target")
	}
}
