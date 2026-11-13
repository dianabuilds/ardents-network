package completion

import (
	"bytes"
	"strings"
	"testing"
)

func TestJoinACKIndependentExactBytesAndClosedHistoryChoice(t *testing.T) {
	request := [160]byte([]byte(strings.Repeat("1", 32) + strings.Repeat("a", 64) + strings.Repeat("b", 64)))
	for _, history := range []uint8{0} {
		reply, err := EncodeJoinACK(request, history)
		want := append(bytes.Clone(request[:]), history)
		if err != nil || !bytes.Equal(reply[:], want) {
			t.Fatal("independent 161-byte Join ACK differs", err)
		}
	}
	for _, history := range []uint8{1, 2, 255} {
		if _, err := EncodeJoinACK(request, history); err != ErrInput {
			t.Fatal("unknown history byte accepted", err)
		}
	}
	request[0] = 'X'
	if _, err := EncodeJoinACK(request, 0); err != ErrInput {
		t.Fatal("noncanonical original request accepted", err)
	}
}
