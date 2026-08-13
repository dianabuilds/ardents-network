package recoverysmoke

import (
	"bytes"
	"strings"
	"testing"
)

func TestBoundedTailDrainsAndRetainsOnlyItsLimit(t *testing.T) {
	tail := &boundedTail{limit: 8}
	input := []byte("0123456789abcdef")
	if written, err := tail.Write(input); err != nil || written != len(input) {
		t.Fatalf("write=%d err=%v", written, err)
	}
	if value := tail.value(); !bytes.Equal(value, []byte("89abcdef")) {
		t.Fatalf("unexpected tail: %q", value)
	}
}

func TestExitClassificationDoesNotRetainUntrustedText(t *testing.T) {
	secret := "raw-secret-material"
	raw := []byte(`{"schema":"ardents-h3-route-observation-v1","error":"authenticate initiator: ` + secret + `"}`)
	classification := classifyExitLog(raw)
	if classification != "initiator-authentication" || strings.Contains(classification, secret) {
		t.Fatalf("unsafe or missing classification: %q", classification)
	}
}
