package enforcement

import "testing"

func TestSnapshotCarriesStateAndReason(t *testing.T) {
	snapshot := Snapshot("enforced", "service type is denied by policy")
	if snapshot.State != "enforced" {
		t.Fatalf("snapshot state = %q, want enforced", snapshot.State)
	}
	if snapshot.Reason == "" {
		t.Fatal("expected snapshot reason")
	}
}
