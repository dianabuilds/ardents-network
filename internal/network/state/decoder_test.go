package state

import "testing"

func TestReaderRejectsTruncationAndNonCanonicalText(t *testing.T) {
	t.Parallel()
	reader := stateReader{raw: []byte{0, 2, 2, 'o', 'k'}}
	if value, err := reader.Uint16(); err != nil || value != 2 {
		t.Fatalf("uint16=%d err=%v", value, err)
	}
	if value, err := reader.Text(3); err != nil || value != "ok" {
		t.Fatalf("text=%q err=%v", value, err)
	}
	if reader.Consumed() != 5 {
		t.Fatal("reader did not consume input")
	}
	if _, err := (&stateReader{raw: []byte{1}}).Uint32(); err == nil {
		t.Fatal("truncated uint32 accepted")
	}
}
