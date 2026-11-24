package epoch

import "testing"

// The shared canonical cursor (R5, #804) must surface its exact bounds refusal
// through the Epoch grammars unchanged, including the materialization member
// read that propagates the cursor error without wrapping.
func TestEpochGrammarsSurfaceCanonicalTruncationRefusal(t *testing.T) {
	t.Parallel()
	if _, err := parseRecord([]byte("ARNR\x01")); err == nil || err.Error() != "truncated canonical bytes" {
		t.Fatalf("parseRecord truncation refusal = %v", err)
	}
	if _, err := parseEpoch([]byte("AREP\x03")); err == nil || err.Error() != "truncated canonical bytes" {
		t.Fatalf("parseEpoch truncation refusal = %v", err)
	}
	if _, err := decodeMaterialization(make([]byte, 32)); err == nil || err.Error() != "truncated canonical bytes" {
		t.Fatalf("decodeMaterialization truncation refusal = %v", err)
	}
}
