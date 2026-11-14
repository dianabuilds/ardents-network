package completion

import (
	"strings"
	"testing"
)

func TestDescriptorACKRetainedSelectionIsClosed(t *testing.T) {
	frame, err := Encode([16]byte{1}, strings.Repeat("a", 64), "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	if err != nil {
		t.Fatal(err)
	}
	for _, history := range []uint8{0, 1} {
		ack, err := EncodeDescriptorACK(frame, history)
		if err != nil || ack[160] != history {
			t.Fatal("exact history choice", history, err)
		}
	}
	for _, history := range []uint8{2, 3, 255} {
		if _, err := EncodeDescriptorACK(frame, history); err == nil {
			t.Fatal("unknown history accepted")
		}
	}
}
