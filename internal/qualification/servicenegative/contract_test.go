package servicenegative

import (
	"context"
	"testing"
)

func TestRunExercisesDistinctMandatoryCases(t *testing.T) {
	receipt, err := Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(receipt.Negatives) != 24 || len(receipt.Mechanisms) != 24 {
		t.Fatalf("unexpected receipt sizes: %+v", receipt)
	}
	seen := map[string]bool{}
	for name, passed := range receipt.Negatives {
		mechanism := receipt.Mechanisms[name]
		if !passed || mechanism == "" || seen[mechanism] {
			t.Fatalf("case %q is missing a distinct successful observation", name)
		}
		seen[mechanism] = true
	}
	if len(receipt.Operations) != 3 || receipt.Classes["cancellation"] != "local timeout or cancellation" ||
		receipt.Classes["partial-write"] != "abrupt connection loss" || receipt.Counts["partial-low"] != 1024 ||
		receipt.Counts["partial-high"] != 2048 {
		t.Fatalf("stream observations are incomplete: %+v", receipt)
	}
}
