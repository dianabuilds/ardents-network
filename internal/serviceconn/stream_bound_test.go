package serviceconn

import "testing"

func TestStreamBoundCoversStage43SustainedWorkload(t *testing.T) {
	if maximumStream < 192<<20 || maximumStream > 256<<20 {
		t.Fatalf("service stream bound %d does not cover the bounded S4.3 workload", maximumStream)
	}
}
