package route

import "testing"

func TestOpaqueRelayBoundCoversSustainedWorkload(t *testing.T) {
	if maximumOpaqueBytes < 192<<20 || maximumOpaqueBytes > 256<<20 {
		t.Fatalf("opaque relay bound %d does not cover the bounded live workload", maximumOpaqueBytes)
	}
}
