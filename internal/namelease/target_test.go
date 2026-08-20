package namelease_test

import (
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/namelease"
)

func TestPublishTargetRequiresMonotonicDistinctBinding(t *testing.T) {
	t.Parallel()
	policy := namelease.Policy{DefaultLeaseDuration: time.Hour, DefaultGraceDuration: time.Hour}
	claimed, err := namelease.Apply(nil, 100, namelease.Op{Kind: "claim", Name: "alice",
		Generation: 1, Authority: "alice-authority"}, policy)
	if err != nil {
		t.Fatal(err)
	}
	if claimed.Target != ([32]byte{}) {
		t.Fatalf("claim created Target binding: %x", claimed.Target)
	}
	targetA := [32]byte{1}
	published, err := namelease.Apply(&claimed, 101, namelease.Op{Kind: "publish", Name: "alice",
		ExpectedGeneration: 1, ExpectedRevision: 1, Authority: "alice-authority", Target: targetA}, policy)
	if err != nil {
		t.Fatal(err)
	}
	if published.Target != targetA || published.Generation != 1 || published.Revision != 2 {
		t.Fatalf("published = %+v", published)
	}
	if _, err := namelease.Apply(&published, 102, namelease.Op{Kind: "publish", Name: "alice",
		ExpectedGeneration: 1, ExpectedRevision: 2, Authority: "alice-authority", Target: targetA}, policy); err == nil {
		t.Fatal("same-Target Service Instance migration created a Name Record revision")
	}
	targetB := [32]byte{2}
	replaced, err := namelease.Apply(&published, 103, namelease.Op{Kind: "publish", Name: "alice",
		ExpectedGeneration: 1, ExpectedRevision: 2, Authority: "alice-authority", Target: targetB}, policy)
	if err != nil {
		t.Fatal(err)
	}
	if replaced.Target != targetB || replaced.Revision != 3 {
		t.Fatalf("replaced = %+v", replaced)
	}
	if _, err := namelease.Apply(&published, 104, namelease.Op{Kind: "publish", Name: "alice",
		ExpectedGeneration: 1, ExpectedRevision: 2, Authority: "other", Target: targetB}, policy); err == nil {
		t.Fatal("different Name Authority replaced Target")
	}
}
