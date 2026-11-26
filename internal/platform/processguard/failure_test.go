package processguard

import (
	"errors"
	"fmt"
	"testing"
	"time"
)

// Failure is a portable copied projection: it must detach every retained
// pointer so a diagnostic consumer cannot mutate the original admission facts,
// and it must reach the observed failure through wrapped/joined errors without
// exposing native error text. The native observation mechanism stays Linux-only.
func TestFailureCopiesDetachedFactsThroughWrappedErrors(t *testing.T) {
	elapsed := int64(120)
	remaining := int64(45)
	observation := Observation{Started: time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC), MemoryBytes: 4096, GoMemoryBytes: 2048}
	original := &observedFailure{
		cause: errors.New("original native cause"),
		facts: FailureFacts{Phase: "observation", Cause: "native-evidence", ElapsedMS: &elapsed, DeadlineRemainingMS: &remaining, Observation: &observation},
	}
	for _, err := range []error{original, fmt.Errorf("wrapped: %w", original), errors.Join(errors.New("sibling"), original)} {
		facts := Failure(err)
		if facts == nil || facts.Phase != "observation" || facts.Cause != "native-evidence" {
			t.Fatalf("joined/wrapped failure lost its facts: %+v", facts)
		}
		if facts.ElapsedMS == original.facts.ElapsedMS || facts.DeadlineRemainingMS == original.facts.DeadlineRemainingMS || facts.Observation == original.facts.Observation {
			t.Fatal("failure projection retained mutable original pointers")
		}
		if *facts.ElapsedMS != 120 || *facts.DeadlineRemainingMS != 45 || facts.Observation.MemoryBytes != 4096 {
			t.Fatalf("copied facts differ: %+v", facts)
		}
		*facts.ElapsedMS = 999
		*facts.DeadlineRemainingMS = 999
		facts.Observation.MemoryBytes = 999
		facts.Phase = "mutated"
	}
	if *original.facts.ElapsedMS != 120 || *original.facts.DeadlineRemainingMS != 45 || original.facts.Observation.MemoryBytes != 4096 || original.facts.Phase != "observation" {
		t.Fatal("consumer mutation reached the original admission facts")
	}
}

func TestFailureMissingObservationStaysUnavailable(t *testing.T) {
	elapsed := int64(7)
	original := &observedFailure{cause: errors.New("cause"), facts: FailureFacts{Phase: "startup", Cause: "profile", ElapsedMS: &elapsed}}
	facts := Failure(original)
	if facts == nil || facts.Observation != nil || facts.DeadlineRemainingMS != nil {
		t.Fatalf("missing measurement became a measured zero: %+v", facts)
	}
	if facts.ElapsedMS == original.facts.ElapsedMS || *facts.ElapsedMS != 7 {
		t.Fatal("elapsed copy differs")
	}
}

func TestFailureIgnoresForeignErrors(t *testing.T) {
	if Failure(nil) != nil {
		t.Fatal("nil error produced facts")
	}
	if Failure(errors.New("plain")) != nil {
		t.Fatal("foreign error produced facts")
	}
	if Failure(fmt.Errorf("wrapped: %w", errors.New("plain"))) != nil {
		t.Fatal("wrapped foreign error produced facts")
	}
}
