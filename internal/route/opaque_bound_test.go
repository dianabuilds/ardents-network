package route

import "testing"

func TestOpaqueRelayBoundCoversSustainedWorkload(t *testing.T) {
	if maximumOpaqueBytes < 750_000_000 || maximumOpaqueBytes > 768<<20 {
		t.Fatalf("opaque relay bound %d does not cover the bounded live workload", maximumOpaqueBytes)
	}
}
