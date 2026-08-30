//go:build browsercompat

package main

import "testing"

func TestTransitRelayByteLimitAlignsDynamicC2RolesWithProductRendezvous(t *testing.T) {
	if limit := (config{}).transitRelayByteLimit(); limit != 256<<10 {
		t.Fatalf("ordinary C2 relay byte limit = %d, want %d", limit, 256<<10)
	}
	input := config{DynamicWorkload: dynamicWorkloadConfig{Cycles: 1800, BytesEachDirection: 4 << 20}}
	if limit := input.transitRelayByteLimit(); limit != 225<<20 {
		t.Fatalf("A11 relay byte limit = %d, want %d", limit, 225<<20)
	}
	input.DynamicWorkload.BytesEachDirection = 64 << 20
	if limit := input.transitRelayByteLimit(); limit != 225<<20 {
		t.Fatalf("maximum dynamic relay byte limit = %d, want %d", limit, 225<<20)
	}
}
