package journal

import "testing"

// The shared canonical cursor (R5, #804) must keep the distribution journal's
// exact refusal split: a truncated magic read stays wrapped in the journal's
// own prose, while a truncated header read surfaces the cursor refusal.
func TestDistributionStateTruncationRefusals(t *testing.T) {
	t.Parallel()
	if _, err := Decode([]byte("ARDS1D4")); err == nil || err.Error() != "distribution state magic is invalid" {
		t.Fatalf("truncated magic refusal = %v", err)
	}
	if _, err := Decode([]byte("ARDS1D4\x00")); err == nil || err.Error() != "truncated canonical bytes" {
		t.Fatalf("truncated header refusal = %v", err)
	}
}
