package rootclaimsimulation_test

import (
	"strings"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/rootclaimsimulation"
)

func TestRunWithSourceRevisionMaterializesAuthenticatedRootClaim(t *testing.T) {
	report, err := rootclaimsimulation.RunWithSourceRevision(strings.Repeat("d", 40))
	if err != nil {
		t.Fatal(err)
	}
	if report.Schema != "ardents-h4-4c-root-claim-simulation-v1" || report.Contract != "h4-4c-project-control-root-claims-v1" || !report.Simulation || report.Qualified || report.SimulationResult != "passed" || len(report.Passed) != 4 || len(report.Rejected) != 4 {
		t.Fatalf("report=%+v", report)
	}
	for _, expected := range []string{"withheld-reveal", "incomplete-evidence", "rule-fork", "control-fork"} {
		if !contains(report.Rejected, expected) {
			t.Fatalf("rejected=%+v", report.Rejected)
		}
	}
}

func contains(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
