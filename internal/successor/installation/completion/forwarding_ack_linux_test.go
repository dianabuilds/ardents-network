package completion

import (
	"bytes"
	"strings"
	"testing"
)

func TestForwardingACKIndependentExactBytesAndClosedHistoryChoice(t *testing.T) {
	request := [160]byte([]byte(strings.Repeat("1", 32) + strings.Repeat("a", 64) + strings.Repeat("b", 64)))
	for _, history := range []uint8{0, 1} {
		reply, err := EncodeForwardingACK(request, history)
		want := append(bytes.Clone(request[:]), history)
		if err != nil || !bytes.Equal(reply[:], want) {
			t.Fatal("independent 161-byte Forwarding ACK differs", err)
		}
	}
	for _, history := range []uint8{2, 255} {
		if _, err := EncodeForwardingACK(request, history); err != ErrInput {
			t.Fatal("unknown history byte accepted", err)
		}
	}
	request[0] = 'X'
	if _, err := EncodeForwardingACK(request, 0); err != ErrInput {
		t.Fatal("noncanonical original request accepted", err)
	}
}
