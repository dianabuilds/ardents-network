package serviceconn

import "testing"

func TestStreamBoundCoversSustainedWorkload(t *testing.T) {
	if MaximumStreamBytes < 192<<20 || MaximumStreamBytes > 256<<20 {
		t.Fatalf("service stream bound %d does not cover the bounded live workload", MaximumStreamBytes)
	}
}
