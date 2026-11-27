package release

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestReleaseHistoryCapacityRefusesBeforePublishing(t *testing.T) {
	history := freshSignedHistory(t)
	baseline := freezeInputs(history.input)
	root := filepath.Join(t.TempDir(), "history")
	verifier, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = verifier.Close() })

	// Genesis contributes a Root-only generation before the first full one.
	var lastAccepted Inputs
	var acceptedFloors FloorSet
	for version := int64(1); version <= 63; version++ {
		history.input = freezeInputs(baseline)
		history.resignOnline(t, history.keys, version)
		decision := verifier.Evaluate(t.Context(), history.input)
		if _, ok := decision.Authorization(); !ok || decision.Outcome != OutcomeReleaseAccepted {
			t.Fatalf("signed release %d refused: %s, %v", version, decision.Outcome, decision.Err())
		}
		lastAccepted, acceptedFloors = freezeInputs(history.input), decision.Floors
	}
	pointer := filepath.Join(root, "current")
	original, err := os.ReadFile(pointer)
	if err != nil {
		t.Fatal(err)
	}
	history.input = freezeInputs(baseline)
	history.resignOnline(t, history.keys, 64)
	refused := verifier.Evaluate(t.Context(), history.input)
	if _, ok := refused.Authorization(); ok || !errors.Is(refused.Err(), ErrHistoryCapacity) {
		t.Fatalf("full history did not refuse with the capacity cause: authorized=%t, error=%v", ok, refused.Err())
	}
	entries, err := os.ReadDir(filepath.Join(root, "generations"))
	if err != nil || len(entries) != 64 {
		t.Fatalf("full generation directory changed size: %d, %v", len(entries), err)
	}
	observed, err := os.ReadFile(pointer)
	if err != nil || !bytes.Equal(observed, original) {
		t.Fatalf("capacity refusal changed the original pointer: %v", err)
	}
	floors, err := verifier.CurrentFloors(t.Context())
	if err != nil || !floorSetEqual(floors, acceptedFloors) {
		t.Fatalf("live floors changed on capacity refusal: %v", err)
	}
	if err := verifier.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenRetained(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	current := reopened.Evaluate(t.Context(), lastAccepted)
	if _, ok := current.Authorization(); !ok || current.Outcome != OutcomeNoUpdate || !floorSetEqual(current.Floors, acceptedFloors) {
		t.Fatalf("last accepted signed release was not usable after retained reopen: %s, %v", current.Outcome, current.Err())
	}
}
