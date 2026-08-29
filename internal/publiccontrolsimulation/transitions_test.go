package publiccontrolsimulation_test

import (
	"strings"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/publiccontrolsimulation"
)

func TestRunControlledTransitionsEmitsSevenBoundedTransitions(t *testing.T) {
	report, err := publiccontrolsimulation.RunControlledTransitionsWithSourceRevision(strings.Repeat("b", 40))
	if err != nil {
		t.Fatal(err)
	}
	if report.Schema != "ardents-h4-6d-transition-simulation-v1" || report.Contract != "h4-6d-project-control-transitions-v1" ||
		report.SimulationResult != "passed" || !report.Simulation || report.Qualified || len(report.Passed) != 7 || len(report.Rejected) != 3 {
		t.Fatalf("transition report = %+v", report)
	}
	for _, expected := range []struct{ caseName, outcome string }{
		{"overlap-accepted", "overlap-accepted"}, {"expiry-stops", "stop-expired"}, {"revocation-stops", "stop-revoked"},
		{"incompatible-generation-stops", "stop-incompatible-generation"}, {"rollback-stops", "stop-rollback"},
		{"distribution-outage-stops", "unavailable-distribution"}, {"emergency-disablement-stops", "stop-emergency-disabled"},
	} {
		if !hasTransition(report.Passed, expected.caseName, expected.outcome) {
			t.Fatalf("transition report has no %s/%s: %+v", expected.caseName, expected.outcome, report)
		}
	}
	for _, expected := range []string{"overlap-without-continuity", "emergency-escalation", "emergency-expired"} {
		if !hasString(report.Rejected, expected) {
			t.Fatalf("transition report did not reject %q: %+v", expected, report)
		}
	}
}

func hasTransition(cells []publiccontrolsimulation.TransitionCell, caseName, outcome string) bool {
	for _, cell := range cells {
		if cell.Case == caseName && cell.Outcome == outcome {
			return true
		}
	}
	return false
}

func hasString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
