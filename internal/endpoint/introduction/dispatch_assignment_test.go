//go:build linux

package introduction

import "testing"

func TestDispatchDrainedWaiterRemainsAssigned(t *testing.T) {
	var dispatch Dispatch
	key := DeliveryKey{Generation: 1}
	waiter, _ := dispatch.AddWaiterLocked(key)
	if dispatch.SelectWaiterLocked(key) != waiter {
		t.Fatal("registered consumer did not receive delivery")
	}
	waiter.Delivery <- RoutedDelivery{Key: key}
	<-waiter.Delivery
	if dispatch.SelectWaiterLocked(key) != nil {
		t.Fatal("draining the channel reassigned the same consumer before removal")
	}
	dispatch.RemoveWaiterLocked(waiter)
	if dispatch.SelectWaiterLocked(key) != nil {
		t.Fatal("removed consumer remained eligible")
	}
}

func TestDispatchAssignsEachWaiterOnlyOnce(t *testing.T) {
	var dispatch Dispatch
	key := DeliveryKey{Generation: 1}
	waiters := make(map[*Waiter]bool)
	for range 4 {
		waiter, _ := dispatch.AddWaiterLocked(key)
		waiters[waiter] = false
	}
	for range 4 {
		waiter := dispatch.SelectWaiterLocked(key)
		if waiter == nil || waiters[waiter] {
			t.Fatal("a second delivery displaced another registered consumer")
		}
		waiters[waiter] = true
		// The consumer can drain its channel before acquiring the owner lock to
		// remove its waiter. That interval must not make it eligible again.
		waiter.Delivery <- RoutedDelivery{Key: key}
		<-waiter.Delivery
	}
	if dispatch.SelectWaiterLocked(key) != nil {
		t.Fatal("completed delivery consumption renewed an assigned waiter")
	}
	for waiter := range waiters {
		dispatch.RemoveWaiterLocked(waiter)
	}
	next, _ := dispatch.AddWaiterLocked(key)
	if dispatch.SelectWaiterLocked(key) != next {
		t.Fatal("a fresh consumer could not receive a later delivery")
	}
}

func TestDispatchBufferedRecoveryConsumesWaiterAssignment(t *testing.T) {
	var dispatch Dispatch
	key := DeliveryKey{Generation: 2, Connection: [32]byte{1}}
	waiter, _ := dispatch.AddWaiterLocked(key)
	recovery := &RecoveryOwner{delivery: make(chan RoutedDelivery, 1)}
	recovery.delivery <- RoutedDelivery{Key: key}
	recovery.HandoffLocked(key, waiter)
	<-waiter.Delivery
	if dispatch.SelectWaiterLocked(key) != nil {
		t.Fatal("consumed recovery handoff renewed an assigned waiter")
	}
	wrong := key
	wrong.Connection[0] = 2
	other, _ := dispatch.AddWaiterLocked(wrong)
	if dispatch.SelectWaiterLocked(key) != nil {
		t.Fatal("a foreign Connection received a delivery")
	}
	if dispatch.SelectWaiterLocked(wrong) != other {
		t.Fatal("the exact unassigned Connection lost its delivery")
	}
}
