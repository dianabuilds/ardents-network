package node

import (
	"testing"
	"time"
)

func TestAdmissionTimeoutHasOneFiniteImplementationBound(t *testing.T) {
	if validAdmissionTimeout(0) {
		t.Fatal("zero admission timeout is valid")
	}
	if validAdmissionTimeout(maximumAdmissionTimeout + time.Nanosecond) {
		t.Fatal("admission timeout beyond the implementation bound is valid")
	}
	if !validAdmissionTimeout(time.Millisecond) || !validAdmissionTimeout(maximumAdmissionTimeout) {
		t.Fatal("finite admission timeout within the implementation bound is invalid")
	}
}

func TestBoundedAdmissionDeadlineNeverOutlivesState(t *testing.T) {
	now := time.Unix(2_000_000_000, 0).UTC()
	if deadline := boundedAdmissionDeadline(now, 30*time.Second, now.Add(10*time.Second)); !deadline.Equal(now.Add(10 * time.Second)) {
		t.Fatalf("State-capped admission deadline = %s", deadline)
	}
	if deadline := boundedAdmissionDeadline(now, 10*time.Second, now.Add(30*time.Second)); !deadline.Equal(now.Add(10 * time.Second)) {
		t.Fatalf("local admission deadline = %s", deadline)
	}
}
