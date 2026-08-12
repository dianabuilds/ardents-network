package state_test

import (
	"os"
	"path/filepath"
	"testing"

	statequalification "github.com/dianabuilds/ardents-network/internal/qualification/state"
)

func TestVerifyIndependentlyRecomputesPersistedState(t *testing.T) {
	t.Parallel()
	fixture := mountFrozenFixture(t)
	result := verifyFrozenFixture(fixture)
	if result.Verdict != "pass" {
		t.Fatalf("verdict = %q (%s), want pass", result.Verdict, result.Reason)
	}
	if result.Generation != fixture.generation || result.Epoch != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestVerifyDistinguishesFailureFromInvalidEvidence(t *testing.T) {
	t.Parallel()
	fixture := mountFrozenFixture(t)
	input := filepath.Join(fixture.root, "generations", fixture.generation, "inputs", "0000.bin")
	raw, err := os.ReadFile(input)
	if err != nil {
		t.Fatal(err)
	}
	raw[10] ^= 0xff
	if err := os.WriteFile(input, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	result := verifyFrozenFixture(fixture)
	if result.Verdict != "fail" {
		t.Fatalf("tampered candidate verdict = %q (%s), want fail", result.Verdict, result.Reason)
	}

	invalid := statequalification.Verify(statequalification.Case{Root: filepath.Join(t.TempDir(), "missing")})
	if invalid.Verdict != "invalid" {
		t.Fatalf("missing evidence verdict = %q, want invalid", invalid.Verdict)
	}
}
