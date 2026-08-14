package recoverysmoke

import "testing"

func TestRecoveryClaimMatchesRequestedSlice(t *testing.T) {
	if got := recoveryClaim("s4.1"); got != "S4.1 local development evidence only" {
		t.Fatalf("S4.1 claim = %q", got)
	}
	want := "S4.2 four-position local development tracer only; does not qualify split-leg/Introduction topology"
	if got := recoveryClaim("s4.2"); got != want {
		t.Fatalf("S4.2 claim = %q; want %q", got, want)
	}
}
