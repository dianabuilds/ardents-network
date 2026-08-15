package route

import "testing"

func TestOpaqueRelayBoundCoversStage43SustainedWorkload(t *testing.T) {
	if maximumOpaqueBytes < 192<<20 || maximumOpaqueBytes > 256<<20 {
		t.Fatalf("opaque relay bound %d does not cover the bounded S4.3 workload", maximumOpaqueBytes)
	}
}
