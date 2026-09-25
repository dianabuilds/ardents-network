//go:build linux

package endpoint

// textIntroductionDispatch owns the context-local waiter and recovery slots.
// Every transition runs under textContext.mu; Context still decides whether a
// job and its authority are live before admitting a slot.
type textIntroductionDispatch struct {
	delivery chan struct{}
	waiters  map[*textIntroductionWaiter]struct{}
	recovery map[*textIntroductionRecoveryOwner]struct{}
}

func (dispatch *textIntroductionDispatch) waiterCapacityReachedLocked() bool {
	return len(dispatch.waiters) >= maximumTextIntroductionWaiters
}

func (dispatch *textIntroductionDispatch) addWaiterLocked(want textIntroductionDeliveryKey) (*textIntroductionWaiter, chan struct{}) {
	if dispatch.delivery == nil {
		dispatch.delivery = make(chan struct{}, 1)
		dispatch.delivery <- struct{}{}
	}
	if dispatch.waiters == nil {
		dispatch.waiters = make(map[*textIntroductionWaiter]struct{})
	}
	waiter := &textIntroductionWaiter{want: want, delivery: make(chan textIntroductionRoutedDelivery, 1)}
	dispatch.waiters[waiter] = struct{}{}
	return waiter, dispatch.delivery
}

func (dispatch *textIntroductionDispatch) removeWaiterLocked(waiter *textIntroductionWaiter) {
	delete(dispatch.waiters, waiter)
}

func (dispatch *textIntroductionDispatch) selectWaiterLocked(key textIntroductionDeliveryKey) *textIntroductionWaiter {
	for waiter := range dispatch.waiters {
		if len(waiter.delivery) == 0 && textIntroductionDeliveryMatches(key, waiter.want) {
			return waiter
		}
	}
	return nil
}

func (dispatch *textIntroductionDispatch) recoveryCapacityReachedLocked(limit int) bool {
	return len(dispatch.recovery) >= limit
}

func (dispatch *textIntroductionDispatch) addRecoveryLocked(binding *textServiceBinding) *textIntroductionRecoveryOwner {
	if dispatch.recovery == nil {
		dispatch.recovery = make(map[*textIntroductionRecoveryOwner]struct{})
	}
	recovery := &textIntroductionRecoveryOwner{binding: binding, generation: 2,
		delivery: make(chan textIntroductionRoutedDelivery, 1)}
	binding.recovery = recovery
	dispatch.recovery[recovery] = struct{}{}
	return recovery
}

func (dispatch *textIntroductionDispatch) removeRecoveryLocked(recovery *textIntroductionRecoveryOwner) {
	delete(dispatch.recovery, recovery)
}

func (dispatch *textIntroductionDispatch) selectRecoveryLocked(key textIntroductionDeliveryKey) *textIntroductionRecoveryOwner {
	for recovery := range dispatch.recovery {
		if recovery.binding != nil && recovery.binding.recovery == recovery && len(recovery.delivery) == 0 &&
			recovery.binding.facts.ConnectionNonce == key.connection && key.generation >= recovery.generation &&
			key.generation <= recovery.generation+1 {
			return recovery
		}
	}
	return nil
}

func (dispatch *textIntroductionDispatch) stopLocked() {
	clear(dispatch.waiters)
	dispatch.waiters = nil
	clear(dispatch.recovery)
	dispatch.recovery = nil
	dispatch.delivery = nil
}
