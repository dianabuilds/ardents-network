package namespacelifecyclesimulation_test

import (
	"strings"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/namespacelifecyclesimulation"
)

func TestRunWithSourceRevisionMaterializesTheCompleteLifecycle(t *testing.T) {
	report, err := namespacelifecyclesimulation.RunWithSourceRevision(strings.Repeat("c", 40))
	if err != nil {
		t.Fatal(err)
	}
	if report.Schema != "ardents-h4-4b-lifecycle-simulation-v1" || report.Contract != "h4-4b-project-control-lifecycle-v1" ||
		report.SimulationResult != "passed" || !report.Simulation || report.Qualified || len(report.Passed) != 6 || len(report.Rejected) != 4 {
		t.Fatalf("lifecycle report = %+v", report)
	}
	for _, expected := range []struct{ name, outcome string }{
		{"publication-current", "threshold-current"}, {"update-current", "threshold-current"},
		{"expiry-grace", "grace-warning"}, {"released-unavailable", "unavailable"},
		{"reclaim-next-generation", "threshold-current"}, {"restart-preserves-current", "threshold-current"},
	} {
		if !hasCell(report.Passed, expected.name, expected.outcome) {
			t.Fatalf("lifecycle report has no %s/%s: %+v", expected.name, expected.outcome, report)
		}
	}
	for _, expected := range []string{"stale-replay", "forked-successor", "old-generation-reclaim"} {
		if !hasString(report.Rejected, expected) {
			t.Fatalf("lifecycle report did not reject %q: %+v", expected, report)
		}
	}
}

func hasCell(cells []namespacelifecyclesimulation.Cell, name, outcome string) bool {
	for _, cell := range cells {
		if cell.Case == name && cell.Outcome == outcome {
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
