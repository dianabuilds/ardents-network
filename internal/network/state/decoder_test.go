package state

import "testing"

func TestReaderRejectsTruncation(t *testing.T) {
	t.Parallel()
	reader := stateReader{raw: []byte{0, 2}}
	if value, err := reader.Uint16(); err != nil || value != 2 {
		t.Fatalf("uint16=%d err=%v", value, err)
	}
	if reader.Consumed() != 2 {
		t.Fatal("reader did not consume input")
	}
	if _, err := (&stateReader{raw: []byte{1}}).Uint32(); err == nil {
		t.Fatal("truncated uint32 accepted")
	}
}
