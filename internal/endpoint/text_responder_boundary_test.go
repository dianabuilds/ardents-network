//go:build linux

package endpoint

import "testing"

// Interrupt the real retained prefix inside the final authority read. The
// callback is a scheduling seam, not a substitute transport or close result.
func checkTextResponderRetirementBoundary(t *testing.T, owner *textContext, job *textJobIdentity, accepted *textIntroductionAttempt, source *textSourceStateFixture) {
	t.Helper()
	prefix := owner.responder.prefix
	// Measure the already-ready path so Source preparation may add authority
	// reads without moving the interruption away from the final handover.
	probe := &textCapsuleBoundaryState{textSourceStateFixture: source}
	owner.endpoint.closedState = probe
	probeErr := owner.prepareTextResponder(t.Context(), job, accepted)
	owner.endpoint.closedState = source
	if probeErr != nil || probe.reads == 0 || owner.responder.prefix != prefix {
		t.Fatalf("cannot establish stable Responder handover boundary: %v", probeErr)
	}
	var closeErr error
	boundary := &textCapsuleBoundaryState{textSourceStateFixture: source, atRead: probe.reads, atBinding: func() { closeErr = prefix.Close() }}
	owner.endpoint.closedState = boundary
	err := owner.prepareTextResponder(t.Context(), job, accepted)
	owner.endpoint.closedState = source
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	if boundary.reads != probe.reads {
		t.Fatal("did not reach final Responder authority boundary")
	}
	if err == nil {
		t.Error("retired Responder returned successful forwarding readiness")
	}
	if err := owner.prepareTextResponder(t.Context(), job, accepted); err != nil {
		t.Fatal(err)
	}
	if owner.responder.prefix == nil || owner.responder.prefix == prefix {
		t.Fatal("explicit retry did not obtain fresh admitted prefix")
	}
}
