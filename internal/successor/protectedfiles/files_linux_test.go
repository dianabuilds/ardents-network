package protectedfiles

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

func TestProtectedInputPathRetainsOriginalCanonicalRules(t *testing.T) {
	for _, tc := range []struct {
		path  string
		valid bool
	}{
		{"/var/lib/input", true}, {"/var/lib/a b", true}, {"/", false},
		{"relative", false}, {"/var//input", false}, {"/var/../input", false},
		{"/var/input\nname", false}, {"/var/input\x00name", false},
	} {
		if got := deliveryPath(tc.path); got != tc.valid {
			t.Errorf("%q: got %v, want %v", tc.path, got, tc.valid)
		}
	}
}
