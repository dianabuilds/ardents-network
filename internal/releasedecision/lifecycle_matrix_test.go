package releasedecision

import (
	"testing"
	"time"
)

func TestEvaluateB9ProtocolPhaseMachine(t *testing.T) {
	t.Parallel()
	refTime := testRefTime
	cases := []struct {
		name    string
		options syntheticOptions
		want    Outcome
	}{
		{"announced", syntheticOptions{protocolPhase: "announced", omitProtocolOverlap: true}, outcomeReleaseAccepted},
		{"overlap-supported", syntheticOptions{protocolPhase: "overlap-supported"}, outcomeReleaseAccepted},
		{"preferred", syntheticOptions{protocolPhase: "preferred"}, outcomeReleaseAccepted},
		{"required-before-window", syntheticOptions{protocolPhase: "required", protocolOverlappedSince: refTime.Add(-30 * 24 * time.Hour)}, outcomeNoUpdate},
		{"required-ready", syntheticOptions{protocolPhase: "required", protocolOverlappedSince: refTime.Add(-100 * 24 * time.Hour)}, outcomeReleaseAccepted},
		{"retired", syntheticOptions{protocolPhase: "retired"}, outcomeReleaseIncompatible},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			repo := newSyntheticRepository(t, test.options)
			decision := evaluateWithStore(t, repo, newMemoryStoreForTest(), refTime)
			if decision.Outcome != test.want {
				t.Fatalf("outcome = %s, want %s (notice: %s)", decision.Outcome, test.want, decision.Notice)
			}
		})
	}
}

func TestEvaluateB9BuildStateMachine(t *testing.T) {
	t.Parallel()
	refTime := testRefTime
	cases := []struct {
		state string
		want  Outcome
	}{
		{"current", outcomeReleaseAccepted},
		{"superseded", outcomeReleaseAccepted},
		{"vulnerable", outcomeReleaseAccepted},
		{"revoked", outcomeReleaseRevoked},
	}
	for _, test := range cases {
		t.Run(test.state, func(t *testing.T) {
			repo := newSyntheticRepository(t, syntheticOptions{buildState: test.state})
			decision := evaluateWithStore(t, repo, newMemoryStoreForTest(), refTime)
			if decision.Outcome != test.want {
				t.Fatalf("outcome = %s, want %s (notice: %s)", decision.Outcome, test.want, decision.Notice)
			}
		})
	}
}

func TestEvaluateB9QualificationStateMachine(t *testing.T) {
	t.Parallel()
	refTime := testRefTime
	cases := []struct {
		state string
		want  Outcome
	}{
		{"qualified", outcomeReleaseAccepted},
		{"development-only", outcomeReleaseIncompatible},
		{"revoked", outcomeReleaseRevoked},
		{"unavailable", outcomeReleaseUnavailable},
	}
	for _, test := range cases {
		t.Run(test.state, func(t *testing.T) {
			repo := newSyntheticRepository(t, syntheticOptions{qualification: test.state})
			decision := evaluateWithStore(t, repo, newMemoryStoreForTest(), refTime)
			if decision.Outcome != test.want {
				t.Fatalf("outcome = %s, want %s (notice: %s)", decision.Outcome, test.want, decision.Notice)
			}
		})
	}
}

func TestEvaluateRejectsMismatchedBuilderAttestation(t *testing.T) {
	t.Parallel()
	for _, options := range []syntheticOptions{
		{attestationDigestMismatch: true},
		{attestationInputMismatch: true},
	} {
		repo := newSyntheticRepository(t, options)
		decision := evaluateWithStore(t, repo, newMemoryStoreForTest(), testRefTime)
		if decision.Outcome != outcomeReleaseInvalid {
			t.Fatalf("outcome = %s, want %s", decision.Outcome, outcomeReleaseInvalid)
		}
	}
}
