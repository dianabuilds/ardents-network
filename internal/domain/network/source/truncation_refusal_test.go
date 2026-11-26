package source

import (
	"errors"
	"testing"
)

// The shared canonical cursor (R5, #804) must keep the bundle's exact refusal
// split: a truncated magic read stays wrapped in the bundle's own prose, while
// a truncated member payload surfaces the cursor refusal under ErrFraming.
func TestDecodeBundleTruncationRefusals(t *testing.T) {
	t.Parallel()
	if _, err := DecodeBundle([]byte("ARDH3B1")); !errors.Is(err, ErrFraming) || err.Error() != "distribution source framing failed: source bundle magic is invalid" {
		t.Fatalf("truncated magic refusal = %v", err)
	}
	truncatedMember := append([]byte("ARDH3B1\x00"), 0, 0, 0, 4, 0x01)
	if _, err := DecodeBundle(truncatedMember); !errors.Is(err, ErrFraming) || err.Error() != "distribution source framing failed: truncated canonical bytes" {
		t.Fatalf("truncated member refusal = %v", err)
	}
}
