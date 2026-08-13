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
	if len(receipt.Operations) != 4 || !receipt.Operations["recovery-queue-full"] ||
		receipt.Classes["cancellation"] != "local timeout or cancellation" ||
		receipt.Classes["partial-write"] != "abrupt connection loss" || receipt.Counts["partial-low"] != 1024 ||
		receipt.Counts["partial-high"] != 2048 {
		t.Fatalf("stream observations are incomplete: %+v", receipt)
	}
}

func TestRunInjectsConcreteContinuityProofAttacks(t *testing.T) {
	digests := map[string]bool{}
	for _, name := range []string{"recovery-replayed-attachment", "recovery-stale-attachment", "recovery-cross-binding"} {
		receipt, err := Run(context.Background(), name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		observed := receipt.Recovery[name]
		if !observed.Passed || observed.TerminalCount != 1 || observed.InjectionKind != name ||
			len(observed.InjectionDigest) != 64 || digests[observed.InjectionDigest] {
			t.Fatalf("%s observation is incomplete or aliased: %+v", name, observed)
		}
		if name == "recovery-replayed-attachment" &&
			(observed.AttackAttempts != 2 || observed.RecoveryCount != 1 || observed.RouteGeneration != 2) {
			t.Fatalf("replay was not captured from an accepted proof: %+v", observed)
		}
		digests[observed.InjectionDigest] = true
	}
}
