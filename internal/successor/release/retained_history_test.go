package release

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenRetainedNeverCreatesInitialTrust(t *testing.T) {
	for _, present := range []bool{false, true} {
		root := filepath.Join(t.TempDir(), "history")
		if present {
			if err := os.Mkdir(root, 0700); err != nil {
				t.Fatal(err)
			}
		}
		v, err := OpenRetained(root)
		if !errors.Is(err, ErrTrustUnavailable) || v != nil {
			t.Fatalf("cold history became trust: %v", err)
		}
		if !present {
			if _, err := os.Lstat(root); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("retained open created a root")
			}
		} else {
			entries, err := os.ReadDir(root)
			if err != nil || len(entries) != 0 {
				t.Fatal("retained open initialized empty history")
			}
		}
	}
}

func TestOpenRetainedRequiresCompleteAuthenticatedFloors(t *testing.T) {
	h := freshSignedHistory(t)
	root := filepath.Join(t.TempDir(), "history")
	v, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	// A genuine signed Root can survive a substituted artifact. That is not
	// sufficient established metadata history for candidate acceptance.
	bad := h.input
	bad.Artifact = append([]byte(nil), h.input.Artifact...)
	bad.Artifact[0] ^= 1
	if decision := v.Evaluate(t.Context(), bad); decision.Outcome != OutcomeReleaseInvalid {
		_ = v.Close()
		t.Fatalf("substituted artifact accepted: %s", decision.Outcome)
	}
	floors, err := v.CurrentFloors(t.Context())
	if err != nil || floors.RootVersion != 1 || floors.TargetsVersion != 0 {
		_ = v.Close()
		t.Fatal("fixture did not establish actual Root-only history")
	}
	if err := v.Close(); err != nil {
		t.Fatal(err)
	}
	if retained, err := OpenRetained(root); !errors.Is(err, ErrTrustUnavailable) || retained != nil {
		if retained != nil {
			_ = retained.Close()
		}
		t.Fatalf("Root-only history accepted: %v", err)
	}
	v, err = Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if decision := v.Evaluate(t.Context(), h.input); decision.Outcome != OutcomeReleaseAccepted {
		_ = v.Close()
		t.Fatalf("genuine initial metadata refused: %s", decision.Outcome)
	}
	if err := v.Close(); err != nil {
		t.Fatal(err)
	}
	v, err = OpenRetained(root)
	if err != nil {
		t.Fatal(err)
	}
	if decision := v.Evaluate(t.Context(), h.input); decision.Outcome != OutcomeNoUpdate {
		_ = v.Close()
		t.Fatalf("retained retry refused: %s", decision.Outcome)
	}
	if err := v.Close(); err != nil {
		t.Fatal(err)
	}
}
