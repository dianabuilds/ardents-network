package completion

import (
	"bytes"
	"strings"
	"testing"
)

func TestPortableHistoryACKPreservesFrameAndChoice(t *testing.T) {
	request := [160]byte([]byte(strings.Repeat("1", 32) + strings.Repeat("a", 64) + strings.Repeat("b", 64)))
	for _, choice := range []uint8{0, 1} {
		got, err := EncodeHistoryACK(request, choice)
		if err != nil || !bytes.Equal(got[:], append(bytes.Clone(request[:]), choice)) {
			t.Fatalf("frame or choice changed: %v", err)
		}
	}
	if _, err := EncodeHistoryACK(request, 2); err != ErrInput {
		t.Fatal("foreign history choice admitted", err)
	}
}
