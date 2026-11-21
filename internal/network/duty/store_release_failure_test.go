package duty

import "testing"

func TestStoreCloseRetainsLeaseReleaseFailure(t *testing.T) {
	owner := &store{lease: invalidReleaseLease(t)}
	first := owner.Close()
	if first == nil {
		t.Fatal("Close hid the lease release failure")
	}
	if second := owner.Close(); second == nil || second.Error() != first.Error() {
		t.Fatalf("repeated Close = %v, want retained %v", second, first)
	}
}
