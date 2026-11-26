package transport

import (
	"crypto/tls"
	"slices"
	"testing"
)

// The expectation is an independent literal. An empty, extra or reordered
// element fails, and no caller can change a later configuration's groups.
func TestClosedProfileKeyExchangeGroupsAreExactAndIsolated(t *testing.T) {
	want := []tls.CurveID{tls.X25519MLKEM768, tls.X25519}
	first := ClosedProfileKeyExchangeGroups()
	if !slices.Equal(first, want) {
		t.Fatalf("configured %v, want %v", first, want)
	}
	first[0], first[1] = tls.CurveP256, tls.CurveP384
	second := ClosedProfileKeyExchangeGroups()
	if !slices.Equal(second, want) {
		t.Fatalf("changed caller slice affected a later call: %v", second)
	}
	clear(second)
	third := ClosedProfileKeyExchangeGroups()
	if !slices.Equal(third, want) || len(third) != 2 {
		t.Fatalf("cleared caller slice affected a later call: %v", third)
	}
}
