//go:build linux

package introduction

import (
	"time"

	"github.com/dianabuilds/ardents-network/internal/route/client"
)

// MaximumWaiters bounds the context-local dispatch waiters.
const MaximumWaiters = 16

// ExpiryReserve is the safety margin subtracted from every delivery deadline
// before local buffering or acknowledgement is allowed.
const ExpiryReserve = 250 * time.Millisecond

// DeliveryKey identifies the exact local owner of one claimed capsule.
type DeliveryKey struct {
	Connection [32]byte
	Generation uint64
}

// Matches reports whether the claimed key belongs to the wanted identity.
// Generation 1 predates the per-Connection nonce commitment, so only the
// generation is compared there.
func (key DeliveryKey) Matches(want DeliveryKey) bool {
	return key.Generation == want.Generation && (want.Generation == 1 || key.Connection == want.Connection)
}

// RoutedDelivery is one claimed capsule together with its routing identity
// and transport deadline.
type RoutedDelivery struct {
	Delivery *client.ClosedIntroductionDelivery
	Key      DeliveryKey
	Expires  time.Time
}

// Waiter is one registered local consumer slot. Delivery has capacity one;
// the single consumer gate assigns at most one routed capsule per waiter.
type Waiter struct {
	Want     DeliveryKey
	Delivery chan RoutedDelivery
}

// Dispatch owns the context-local waiter and recovery slots,
// their single consumer gate, routing, and waiter cleanup. Slot transitions
// run under dutyContext.mu; the root consumer loop supplies live job and
// publication authority before a slot is admitted or a claimed delivery is
// inspected. The zero value is ready for use.
type Dispatch struct {
	gate     chan struct{}
	waiters  map[*Waiter]struct{}
	recovery map[*RecoveryOwner]struct{}
}

// WaiterCapacityReachedLocked reports whether the fixed waiter bound is met.
func (dispatch *Dispatch) WaiterCapacityReachedLocked() bool {
	return len(dispatch.waiters) >= MaximumWaiters
}

// AddWaiterLocked registers one waiter and returns it with the shared consumer
// gate, which is created holding its single token on first use.
func (dispatch *Dispatch) AddWaiterLocked(want DeliveryKey) (*Waiter, chan struct{}) {
	if dispatch.gate == nil {
		dispatch.gate = make(chan struct{}, 1)
		dispatch.gate <- struct{}{}
	}
	if dispatch.waiters == nil {
		dispatch.waiters = make(map[*Waiter]struct{})
	}
	waiter := &Waiter{Want: want, Delivery: make(chan RoutedDelivery, 1)}
	dispatch.waiters[waiter] = struct{}{}
	return waiter, dispatch.gate
}

// RemoveWaiterLocked drops one waiter slot.
func (dispatch *Dispatch) RemoveWaiterLocked(waiter *Waiter) {
	delete(dispatch.waiters, waiter)
}

// SelectWaiterLocked returns the first unassigned waiter that matches the
// claimed key exactly.
func (dispatch *Dispatch) SelectWaiterLocked(key DeliveryKey) *Waiter {
	for waiter := range dispatch.waiters {
		if len(waiter.Delivery) == 0 && key.Matches(waiter.Want) {
			return waiter
		}
	}
	return nil
}

// RecoveryCapacityReachedLocked reports whether the caller-supplied recovery
// bound is met.
func (dispatch *Dispatch) RecoveryCapacityReachedLocked(limit int) bool {
	return len(dispatch.recovery) >= limit
}

// AddRecoveryLocked registers one root-constructed recovery slot.
func (dispatch *Dispatch) AddRecoveryLocked(recovery *RecoveryOwner) {
	if recovery == nil {
		return
	}
	if dispatch.recovery == nil {
		dispatch.recovery = make(map[*RecoveryOwner]struct{})
	}
	dispatch.recovery[recovery] = struct{}{}
}

// RemoveRecoveryLocked drops one recovery slot.
func (dispatch *Dispatch) RemoveRecoveryLocked(recovery *RecoveryOwner) {
	delete(dispatch.recovery, recovery)
}

// SelectRecoveryLocked returns the first recovery slot that accepts the
// claimed key.
func (dispatch *Dispatch) SelectRecoveryLocked(key DeliveryKey) *RecoveryOwner {
	for recovery := range dispatch.recovery {
		if recovery.AcceptsLocked(key) {
			return recovery
		}
	}
	return nil
}

// StopLocked clears every slot and the consumer gate without joining
// anything; buffered capsules are refused by the registration retirement.
func (dispatch *Dispatch) StopLocked() {
	clear(dispatch.waiters)
	dispatch.waiters = nil
	clear(dispatch.recovery)
	dispatch.recovery = nil
	dispatch.gate = nil
}
