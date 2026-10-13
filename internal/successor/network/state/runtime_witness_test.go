package state

import (
	"errors"
	"testing"
)

func TestRuntimeWitnessRetainsOriginalRefusal(t *testing.T) {
	for _, fault := range []string{"close", "physical-read", "retirement"} {
		t.Run(fault, func(t *testing.T) {
			owner, raw := closedProfileStoreFixture(t)
			if _, err := owner.AcceptClosedProfile(raw); err != nil {
				t.Fatal(err)
			}
			witness, err := owner.CurrentRuntimeWitness()
			if err != nil {
				t.Fatal(err)
			}
			profile := witness.View().Profile().ProfileBinding
			if err := witness.CheckCommit(profile); err != nil {
				t.Fatal("live original refused", err)
			}
			switch fault {
			case "close":
				if err := owner.Close(); err != nil {
					t.Fatal(err)
				}
			case "physical-read":
				if err := owner.storage.Close(); err != nil {
					t.Fatal(err)
				}
				if _, err := owner.CurrentRuntime(); err == nil {
					t.Fatal("closed physical root accepted")
				}
			case "retirement":
				owner.mu.Lock()
				owner.retireStateLocked()
				owner.mu.Unlock()
			}
			// The delayed checked value remains immutable. Its local original
			// witness must nevertheless deny the final result without new I/O.
			if err := witness.View().Check(witness.View().ObservedAt()); err != nil {
				t.Fatal("facts mutated", err)
			}
			if err := witness.CheckCommit(profile); err == nil {
				t.Fatal("retired original allowed late handoff")
			}
		})
	}
}

func TestRuntimeWitnessDoesNotAdoptForeignOrChangedFacts(t *testing.T) {
	owner, raw := closedProfileStoreFixture(t)
	if _, err := owner.AcceptClosedProfile(raw); err != nil {
		t.Fatal(err)
	}
	witness, err := owner.CurrentRuntimeWitness()
	if err != nil {
		t.Fatal(err)
	}
	profile := witness.View().Profile().ProfileBinding
	if err := (RuntimeWitness{}).CheckCommit(profile); err == nil {
		t.Fatal("zero witness accepted")
	}
	changed := profile
	changed.Digest[0] ^= 1
	if err := witness.CheckCommit(changed); err == nil {
		t.Fatal("foreign profile accepted")
	}
	repeated, err := owner.CurrentRuntimeWitness()
	if err != nil || !witness.SameOriginal(repeated) {
		t.Fatal("unchanged original replaced", err)
	}
	if _, err := owner.AcceptClosedProfile(raw); err != nil {
		t.Fatal(err)
	}
	if err := witness.CheckCommit(profile); err != nil {
		t.Fatal("exact duplicate revoked original", err)
	}
	other, otherRaw := closedProfileStoreFixture(t)
	if _, err := other.AcceptClosedProfile(otherRaw); err != nil {
		t.Fatal(err)
	}
	foreign, err := other.CurrentRuntimeWitness()
	if err != nil || witness.SameOriginal(foreign) {
		t.Fatal("different State adopted", err)
	}
}

func TestRuntimeWitnessCannotReviveAfterObservedRefusal(t *testing.T) {
	owner, raw := closedProfileStoreFixture(t)
	if _, err := owner.AcceptClosedProfile(raw); err != nil {
		t.Fatal(err)
	}
	original, err := owner.CurrentRuntimeWitness()
	if err != nil {
		t.Fatal(err)
	}
	refusal := errors.New("original runtime admission refused")
	owner.config.permitWork = func() error { return refusal }
	if _, err := owner.CurrentRuntime(); !errors.Is(err, refusal) {
		t.Fatal("full observation ignored refusal", err)
	}
	owner.config.permitWork = nil
	later, err := owner.CurrentRuntimeWitness()
	if err != nil {
		t.Fatal(err)
	}
	profile := original.View().Profile().ProfileBinding
	if err := original.CheckCommit(profile); err == nil || original.SameOriginal(later) {
		t.Fatal("later observation revived original handoff", err)
	}
	if err := later.CheckCommit(profile); err != nil {
		t.Fatal("new independent observation refused", err)
	}
}
