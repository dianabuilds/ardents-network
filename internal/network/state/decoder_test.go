package state

import "testing"

func TestReaderRejectsTruncation(t *testing.T) {
	t.Parallel()
	reader := newDecoder([]byte{0, 2})
	if value, err := reader.uint16(); err != nil || value != 2 {
		t.Fatalf("uint16=%d err=%v", value, err)
	}
	if !reader.done() {
		t.Fatal("reader did not consume input")
	}
	truncated := newDecoder([]byte{1})
	if _, err := truncated.uint32(); err == nil {
		t.Fatal("truncated uint32 accepted")
	}
}
