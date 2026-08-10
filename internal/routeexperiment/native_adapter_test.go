package routeexperiment

import (
	"errors"
	"testing"
)

func TestOperationalAttemptFailureRequiresValidatedRuntime(t *testing.T) {
	t.Parallel()
	failure := errors.New("container did not start")
	invalid := nativeAttemptEvidence{summary: nativeAttemptSummary{
		Status: "failed", Checks: map[string]bool{"verified_images": true},
	}}
	if operationalAttemptError(invalid, failure) == nil {
		t.Fatal("pre-topology container failure was accepted as a technology measurement")
	}
	measured := nativeAttemptEvidence{summary: nativeAttemptSummary{
		Status: "failed", Checks: map[string]bool{
			"verified_images": true, "fixed_topology": true, "bounded_capabilities": true,
		},
	}}
	if err := operationalAttemptError(measured, failure); err != nil {
		t.Fatalf("post-validation protocol failure was not retained as a measurement: %v", err)
	}
}
