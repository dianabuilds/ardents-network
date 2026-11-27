package endpointstream

import (
	"errors"
	"strings"
	"testing"
)

func TestRefuseCarriesBoundedOutcome(t *testing.T) {
	outcome := Outcome{Class: ServiceUnavailable, Reason: "selected service is unavailable"}
	err := Refuse(outcome)
	var refusal SetupRefusalError
	if !errors.As(err, &refusal) {
		t.Fatalf("Refuse returned %T, want SetupRefusalError", err)
	}
	if got := refusal.Outcome(); got != outcome {
		t.Fatalf("refusal outcome = %+v, want %+v", got, outcome)
	}
	if got, want := refusal.Error(), string(ServiceUnavailable)+": selected service is unavailable"; got != want {
		t.Fatalf("refusal error = %q, want %q", got, want)
	}
}

func TestRefuseWithoutReasonPrintsClass(t *testing.T) {
	err := Refuse(Outcome{Class: CapacityUnavailable})
	if got, want := err.Error(), string(CapacityUnavailable); got != want {
		t.Fatalf("refusal error = %q, want %q", got, want)
	}
}

func TestRefuseRejectsInvalidOutcome(t *testing.T) {
	for _, test := range []struct {
		name    string
		outcome Outcome
	}{
		{name: "empty class", outcome: Outcome{Reason: "bounded"}},
		{name: "oversized class", outcome: Outcome{Class: OutcomeClass(strings.Repeat("a", maximumOutcomeClassBytes+1))}},
		{name: "oversized reason", outcome: Outcome{Class: ServiceUnavailable, Reason: strings.Repeat("a", maximumOutcomeReasonBytes+1)}},
		{name: "invalid class utf8", outcome: Outcome{Class: OutcomeClass(string([]byte{0xff}))}},
		{name: "invalid reason utf8", outcome: Outcome{Class: ServiceUnavailable, Reason: string([]byte{0xff})}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := Refuse(test.outcome); err == nil {
				t.Fatalf("Refuse(%+v) accepted an invalid outcome", test.outcome)
			}
		})
	}
}
