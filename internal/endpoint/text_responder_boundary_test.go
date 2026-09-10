//go:build linux

package endpoint

import "testing"

// Interrupt the real retained prefix inside the final authority read. The
// callback is a scheduling seam, not a substitute transport or close result.
func checkTextResponderRetirementBoundary(t *testing.T, owner *textContext, job *textJobIdentity, accepted *textIntroductionAttempt, source *textSourceStateFixture) {
	t.Helper()
	prefix := owner.responder.prefix
	var closeErr error
	boundary := &textCapsuleBoundaryState{textSourceStateFixture: source, atRead: 3, atBinding: func() { closeErr = prefix.Close() }}
	owner.endpoint.closedState = boundary
	err := owner.prepareTextResponder(t.Context(), job, accepted)
	owner.endpoint.closedState = source
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	if boundary.reads < 3 {
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
