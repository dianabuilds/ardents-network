package publiccontrolsimulation_test

import (
	"strings"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/publiccontrolsimulation"
)

func TestRunExercisesFullMechanicsAndReaderFailureMatrixWithoutQualification(t *testing.T) {
	report, err := publiccontrolsimulation.RunWithSourceRevision(strings.Repeat("a", 40))
	if err != nil {
		t.Fatal(err)
	}
	if report.Schema != "ardents-h4-6c-simulation-v1" || report.Contract != "h4-6c-project-control-simulation-v1" ||
		report.SimulationResult != "passed" || report.DeclaredSourceRevision != strings.Repeat("a", 40) || !strings.HasPrefix(report.ReceiptDigest, "sha256:") || !report.Simulation || report.Qualified {
		t.Fatalf("simulation report = %+v", report)
	}
	if len(report.Passed) != 6 || len(report.Rejected) != 16 {
		t.Fatalf("simulation coverage = %+v", report)
	}
}
