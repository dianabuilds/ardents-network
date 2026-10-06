package admission

import (
	"encoding/binary"
	"testing"
)

// Framing-only bytes deliberately contain no permission or valid holder proof.
// They cannot authorize issuance; the genuine issuer tests own that boundary.
func TestUnpadClosedTokenBatchKeepsCanonicalLengthAndRejectsTail(t *testing.T) {
	for _, count := range []int{1, 32} {
		raw := make([]byte, 16351)
		copy(raw, "ARDIBR01")
		binary.BigEndian.PutUint16(raw[623:625], uint16(count))
		want := 689 + 259*count
		batch, err := UnpadClosedTokenBatch(raw)
		if err != nil || len(batch) != want || &batch[0] != &raw[0] {
			t.Fatal("canonical prefix was changed", count, len(batch), err)
		}
		if _, err := DecodeClosedTokenBatch(batch); err == nil {
			t.Fatal("framing became successful permission verification")
		}
		raw[len(raw)-1] = 1
		if _, err := UnpadClosedTokenBatch(raw); err == nil {
			t.Fatal("second payload in padding accepted")
		}
		raw[len(raw)-1] = 0
		if _, err := UnpadClosedTokenBatch(raw[:want-1]); err == nil {
			t.Fatal("truncated batch accepted")
		}
	}
	for _, count := range []uint16{0, 33, 65535} {
		raw := make([]byte, 16351)
		copy(raw, "ARDIBR01")
		binary.BigEndian.PutUint16(raw[623:625], count)
		if _, err := UnpadClosedTokenBatch(raw); err == nil {
			t.Fatal("unbounded count accepted", count)
		}
	}
}
