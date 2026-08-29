package publiccontrolsimulation_test

import (
	"testing"

	"github.com/dianabuilds/ardents-network/internal/publiccontrolsimulation"
)

func TestRunExercisesFullMechanicsAndReaderFailureMatrixWithoutQualification(t *testing.T) {
	report, err := publiccontrolsimulation.Run()
	if err != nil {
		t.Fatal(err)
	}
	if report.Schema != "ardents-h4-6c-simulation-v1" || !report.Simulation || report.Qualified {
		t.Fatalf("simulation report = %+v", report)
	}
	if len(report.Passed) != 6 || len(report.Rejected) != 16 {
		t.Fatalf("simulation coverage = %+v", report)
	}
}
