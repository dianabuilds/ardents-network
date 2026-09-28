//go:build linux

package endpoint

// introductionDispatch owns the context-local waiter and recovery slots,
// their single consumer gate, routing, and waiter cleanup. Slot transitions
// run under textContext.mu; Context supplies live job and publication authority
// before a slot is admitted or a claimed delivery is inspected.
type introductionDispatch struct {
	delivery chan struct{}
	waiters  map[*introductionWaiter]struct{}
	recovery map[*introductionRecoveryOwner]struct{}
}

func (dispatch *introductionDispatch) waiterCapacityReachedLocked() bool {
	return len(dispatch.waiters) >= maximumIntroductionWaiters
}

func (dispatch *introductionDispatch) addWaiterLocked(want introductionDeliveryKey) (*introductionWaiter, chan struct{}) {
	if dispatch.delivery == nil {
		dispatch.delivery = make(chan struct{}, 1)
		dispatch.delivery <- struct{}{}
	}
	if dispatch.waiters == nil {
		dispatch.waiters = make(map[*introductionWaiter]struct{})
	}
	waiter := &introductionWaiter{want: want, delivery: make(chan introductionRoutedDelivery, 1)}
	dispatch.waiters[waiter] = struct{}{}
	return waiter, dispatch.delivery
}

func (dispatch *introductionDispatch) removeWaiterLocked(waiter *introductionWaiter) {
	delete(dispatch.waiters, waiter)
}

func (dispatch *introductionDispatch) selectWaiterLocked(key introductionDeliveryKey) *introductionWaiter {
	for waiter := range dispatch.waiters {
		if len(waiter.delivery) == 0 && introductionDeliveryMatches(key, waiter.want) {
			return waiter
		}
	}
	return nil
}

func (dispatch *introductionDispatch) recoveryCapacityReachedLocked(limit int) bool {
	return len(dispatch.recovery) >= limit
}

func (dispatch *introductionDispatch) addRecoveryLocked(binding *serviceBinding) *introductionRecoveryOwner {
	recovery := binding.claimRecoveryLocked()
	if recovery == nil {
		return nil
	}
	if dispatch.recovery == nil {
		dispatch.recovery = make(map[*introductionRecoveryOwner]struct{})
	}
	dispatch.recovery[recovery] = struct{}{}
	return recovery
}

func (dispatch *introductionDispatch) removeRecoveryLocked(recovery *introductionRecoveryOwner) {
	delete(dispatch.recovery, recovery)
}

func (dispatch *introductionDispatch) selectRecoveryLocked(key introductionDeliveryKey) *introductionRecoveryOwner {
	for recovery := range dispatch.recovery {
		if recovery.acceptsLocked(key) {
			return recovery
		}
	}
	return nil
}

func (dispatch *introductionDispatch) stopLocked() {
	clear(dispatch.waiters)
	dispatch.waiters = nil
	clear(dispatch.recovery)
	dispatch.recovery = nil
	dispatch.delivery = nil
}
