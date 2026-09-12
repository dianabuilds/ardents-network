//go:build linux

package route

import (
	"testing"
	"time"
)

func TestClosedBootstrapBoundsAdjacentDutyQueueLifetimeAndOutput(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	controller, err := NewClosedBootstrapController(func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	adjacency := [32]byte{1}
	leases := make([]*ClosedBootstrapLease, 0, closedBootstrapAdjacent)
	for range closedBootstrapAdjacent {
		lease, admitErr := controller.Admit(adjacency, now.Add(time.Minute))
		if admitErr != nil {
			t.Fatal(admitErr)
		}
		leases = append(leases, lease)
	}
	if _, err := controller.Admit(adjacency, now.Add(time.Minute)); err == nil {
		t.Fatal("admitted a fifth bootstrap lane on one adjacency")
	}
	for index := 2; index <= 4; index++ {
		for range closedBootstrapAdjacent {
			if _, err := controller.Admit([32]byte{byte(index)}, now.Add(time.Minute)); err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, err := controller.Admit([32]byte{5}, now.Add(time.Minute)); err == nil {
		t.Fatal("admitted a seventeenth bootstrap lane on one duty")
	}
	lane := leases[0]
	if err := lane.Queue(closedBootstrapLaneBytes); err != nil {
		t.Fatal(err)
	}
	if err := lane.Queue(1); err == nil {
		t.Fatal("exceeded per-lane queue bound")
	}
	if err := leases[1].Queue(closedBootstrapLaneBytes); err != nil {
		t.Fatal(err)
	}
	if err := leases[2].Queue(1); err == nil {
		t.Fatal("exceeded shared bootstrap queue bound")
	}
	if err := lane.Dequeue(closedBootstrapLaneBytes); err != nil {
		t.Fatal(err)
	}
	if err := leases[1].Dequeue(closedBootstrapLaneBytes); err != nil {
		t.Fatal(err)
	}
	if err := lane.Send(closedBootstrapOutputBurst); err != nil {
		t.Fatal(err)
	}
	if err := lane.Send(1); err == nil {
		t.Fatal("exceeded output burst")
	}
	now = now.Add(time.Second)
	if err := leases[1].Send(closedBootstrapOutputRate / 60); err != nil {
		t.Fatalf("one-second output refill = %v", err)
	}
	now = now.Add(closedBootstrapLaneLife)
	if err := lane.Send(1); err == nil {
		t.Fatal("expired bootstrap lane remained usable")
	}
	lane.Release()
	if err := leases[1].Queue(1); err == nil {
		t.Fatal("duty reaper did not release all expired lanes")
	}
}
