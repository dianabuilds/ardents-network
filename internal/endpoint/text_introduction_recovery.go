//go:build linux

package endpoint

import (
	"context"
	"time"
)

// textIntroductionRecoveryOwner exists from initial acceptance until the
// Publisher Service stream retires. It can own the next valid recovery capsule
// before the native Connection detects the failed Carrier. All slot transitions
// run under textContext.mu; completion and timer joins run after unlocking.
type textIntroductionRecoveryOwner struct {
	binding    *textServiceBinding
	generation uint64
	delivery   chan textIntroductionRoutedDelivery
	expiryStop context.CancelFunc
	expiryDone chan struct{}
}

// admitGenerationLocked advances only the next generation of this exact
// binding. A stale or speculative later waiter cannot change which capsule
// the recovery slot may retain.
func (recovery *textIntroductionRecoveryOwner) admitGenerationLocked(binding *textServiceBinding,
	generation uint64) bool {
	if recovery == nil || recovery.binding != binding || generation < recovery.generation ||
		generation > recovery.generation+1 {
		return false
	}
	if generation > recovery.generation {
		recovery.generation = generation
	}
	return true
}

func (recovery *textIntroductionRecoveryOwner) acceptsLocked(key textIntroductionDeliveryKey) bool {
	return recovery != nil && recovery.binding.ownsRecoveryLocked(recovery) &&
		len(recovery.delivery) == 0 && recovery.binding.connectionNonce() == key.connection &&
		key.generation >= recovery.generation && key.generation <= recovery.generation+1
}

// handoffLocked transfers a buffered capsule to its exact waiter and returns
// the expiry join. The caller must wait for that join after dropping Context's
// lock, so an expiring capsule cannot also be completed by the waiter.
func (recovery *textIntroductionRecoveryOwner) handoffLocked(want textIntroductionDeliveryKey,
	waiter *textIntroductionWaiter) <-chan struct{} {
	select {
	case routed := <-recovery.delivery:
		if textIntroductionDeliveryMatches(routed.key, want) {
			var expiryDone <-chan struct{}
			if recovery.expiryStop != nil {
				recovery.expiryStop()
				expiryDone = recovery.expiryDone
				recovery.expiryStop, recovery.expiryDone = nil, nil
			}
			waiter.delivery <- routed
			return expiryDone
		}
		recovery.delivery <- routed
	default:
	}
	return nil
}

// bufferLocked retains only one capsule for a live recovery owner and starts
// its deadline refusal. The shared Context lock makes routing and retention
// one transition with waiter selection.
func (recovery *textIntroductionRecoveryOwner) bufferLocked(owner *textContext,
	routed textIntroductionRoutedDelivery) bool {
	if recovery == nil || !recovery.binding.ownsRecoveryLocked(recovery) ||
		len(recovery.delivery) != 0 || recovery.expiryDone != nil || routed.delivery == nil ||
		!owner.endpoint.clock().Add(textIntroductionExpiryReserve).Before(routed.expires) {
		return false
	}
	lifetime, stop := context.WithCancel(owner.lease.Context())
	done := make(chan struct{})
	recovery.delivery <- routed
	recovery.expiryStop, recovery.expiryDone = stop, done
	go recovery.expire(owner, lifetime, stop, routed, done)
	return true
}

func (recovery *textIntroductionRecoveryOwner) expire(owner *textContext, ctx context.Context,
	stop context.CancelFunc, routed textIntroductionRoutedDelivery, done chan struct{}) {
	defer stop()
	defer func() {
		owner.mu.Lock()
		if recovery.expiryDone == done {
			recovery.expiryStop, recovery.expiryDone = nil, nil
		}
		owner.mu.Unlock()
		close(done)
	}()
	wait := routed.expires.Sub(owner.endpoint.clock()) - textIntroductionExpiryReserve
	if wait < 0 {
		wait = 0
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return
	case <-timer.C:
	}
	owner.mu.Lock()
	if recovery.expiryDone != done || !recovery.binding.ownsRecoveryLocked(recovery) {
		owner.mu.Unlock()
		return
	}
	var expired textIntroductionRoutedDelivery
	select {
	case expired = <-recovery.delivery:
	default:
	}
	lifetime := owner.lease.Context()
	owner.mu.Unlock()
	if expired.delivery == nil || expired.delivery != routed.delivery || lifetime.Err() != nil {
		return
	}
	bounded, cancel := context.WithDeadline(lifetime, routed.expires)
	err := expired.delivery.Complete(bounded, 1)
	cancel()
	if err != nil && lifetime.Err() == nil {
		owner.endpoint.failTextContexts(err)
	}
}

// retireLocked removes the buffered capsule and interrupts deadline refusal.
// The caller joins expiryDone before completing the capsule from its own
// lifetime, keeping the shared Publisher registration alive for other streams.
func (recovery *textIntroductionRecoveryOwner) retireLocked() (<-chan struct{}, textIntroductionRoutedDelivery) {
	expiryStop, expiryDone := recovery.expiryStop, recovery.expiryDone
	recovery.expiryStop, recovery.expiryDone = nil, nil
	if expiryStop != nil {
		expiryStop()
	}
	var routed textIntroductionRoutedDelivery
	select {
	case routed = <-recovery.delivery:
	default:
	}
	return expiryDone, routed
}
