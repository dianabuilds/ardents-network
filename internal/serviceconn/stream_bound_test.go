package serviceconn

import "testing"

func TestStreamBoundCoversSustainedWorkload(t *testing.T) {
	if MaximumStreamBytes < 750_000_000 || MaximumStreamBytes > 768<<20 {
		t.Fatalf("service stream bound %d does not cover the bounded live workload", MaximumStreamBytes)
	}
}
