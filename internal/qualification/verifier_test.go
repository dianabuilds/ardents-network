package qualification_test

import (
	"crypto/ed25519"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/qualification"
)

func TestVerifyOfflineIndependentlyRecomputesPersistedState(t *testing.T) {
	t.Parallel()
	fixture := writeVerifierFixture(t)
	result := qualification.VerifyOffline(qualification.OfflineCase{
		Root:             fixture.root,
		NetworkID:        fixture.networkID,
		Authorities:      map[[32]byte]ed25519.PublicKey{fixture.authorityID: fixture.authorityPublic},
		Threshold:        1,
		Now:              time.Unix(fixture.now, 0),
		Materializations: fixture.materializations,
	})
	if result.Verdict != "pass" {
		t.Fatalf("verdict = %q (%s), want pass", result.Verdict, result.Reason)
	}
	if result.Generation != fixture.generation || result.Epoch != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestVerifyOfflineDistinguishesFailureFromInvalidEvidence(t *testing.T) {
	t.Parallel()
	fixture := writeVerifierFixture(t)
	input := filepath.Join(fixture.root, "generations", fixture.generation, "inputs", "0000.bin")
	raw, err := os.ReadFile(input)
	if err != nil {
		t.Fatal(err)
	}
	raw[10] ^= 0xff
	if err := os.WriteFile(input, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	result := qualification.VerifyOffline(qualification.OfflineCase{
		Root:             fixture.root,
		NetworkID:        fixture.networkID,
		Authorities:      map[[32]byte]ed25519.PublicKey{fixture.authorityID: fixture.authorityPublic},
		Threshold:        1,
		Now:              time.Unix(fixture.now, 0),
		Materializations: fixture.materializations,
	})
	if result.Verdict != "fail" {
		t.Fatalf("tampered candidate verdict = %q (%s), want fail", result.Verdict, result.Reason)
	}

	invalid := qualification.VerifyOffline(qualification.OfflineCase{Root: filepath.Join(t.TempDir(), "missing")})
	if invalid.Verdict != "invalid" {
		t.Fatalf("missing evidence verdict = %q, want invalid", invalid.Verdict)
	}
}
