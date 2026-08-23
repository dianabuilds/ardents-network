package endpoint

import "testing"

func TestStreamBoundCoversSustainedWorkload(t *testing.T) {
	if maximumStreamBytes < 750_000_000 || maximumStreamBytes > 768<<20 {
		t.Fatalf("service stream bound %d does not cover the bounded live workload", maximumStreamBytes)
	}
}
