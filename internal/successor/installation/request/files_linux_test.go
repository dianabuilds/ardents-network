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
	// Both deliberately invalid callers must refuse before native custody.
	for _, caller := range []context.Context{nil, ctx} {
		if h, err := OpenFiles(caller); err == nil || h.original != nil {
			_ = h.Close()
			t.Fatal("unavailable caller acquired original custody", err)
		}
	}
}
