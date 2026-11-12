package request

import (
	"context"
	"testing"
)

func TestDeliveryFilesZeroAndUnavailableCallerGrantNoCustody(t *testing.T) {
	var empty Files
	if empty.Check() == nil || empty.RetainParents("/var/lib/absent") == nil || empty.CheckClock("/var/lib/clock") == nil {
		t.Fatal("zero custody admitted native observation")
	}
	if raw, err := empty.Read("/var/lib/input", 16, true); err == nil || raw != nil {
		t.Fatal("zero custody exposed bytes")
	}
	if empty.Close() != nil {
		t.Fatal("empty Close failed")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := OpenFiles(ctx); err == nil {
		t.Fatal("canceled caller acquired original custody")
	}
	if _, err := OpenFiles(nil); err == nil {
		t.Fatal("missing caller acquired original custody")
	}
}
