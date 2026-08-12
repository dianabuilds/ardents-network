package state

import "testing"

func TestByDigestResponseRejectsDifferentObject(t *testing.T) {
	t.Parallel()
	requested, returned := [32]byte{1}, [32]byte{2}
	if err := validateByDigestResponse(requested, returned); err == nil {
		t.Fatal("BY_DIGEST accepted an object other than its exact selector")
	}
}

func TestProtocolStatusesKeepDistinctTerminalOutcomes(t *testing.T) {
	t.Parallel()
	want := [...]byte{sourceOutcomeNotFound, sourceOutcomeBusy, sourceOutcomeBadRequest, sourceOutcomeInternal}
	for status := byte(1); status < byte(len(sourceStatusErrors)); status++ {
		if got := classifySourceOutcome(sourceStatusErrors[status]); got != want[status-1] {
			t.Fatalf("status %d outcome=%d want=%d", status, got, want[status-1])
		}
	}
}
