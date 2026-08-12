package node

import (
	"context"
	"strings"
	"testing"
)

func TestEventEmitterRejectsUnboundedFieldsBeforeEncoding(t *testing.T) {
	emit := EventEmitter(nil)
	if err := emit(context.Background(), Event{Reason: strings.Repeat("x", 257)}); err == nil {
		t.Fatal("unbounded lifecycle event was accepted")
	}
}
