//go:build linux

package introduction

import (
	"context"
	"sync"
	"time"
)

// Host is the narrow seam to the duty context for the only two recovery
// transitions that need context authority: the buffered-capsule deadline
// refusal and its failure publication. The root supplies it per call; the
// RecoveryOwner itself retains no host reference.
type Host interface {
	// Mu returns the dutyContext mutex. The expiry goroutine locks it around
	// every slot transition; BufferLocked is called with it already held.
	Mu() *sync.Mutex
	// Now returns the endpoint clock instant.
	Now() time.Time
	// LeaseContext returns the bounded lifetime of the admitted duty.
	LeaseContext() context.Context
	// FailDutyContexts terminalizes every duty context of the endpoint after
	// a delivery completion failure outside a retired lease.
	FailDutyContexts(error)
}

// RecoveryBinding is the seam to the root's service binding that owns one
// recovery slot. It breaks the bidirectional binding/recovery reference: the
// slot verifies live ownership and identity through this interface only.
type RecoveryBinding interface {
	OwnsRecoveryLocked(recovery *RecoveryOwner) bool
	ConnectionNonce() [32]byte
}

// RecoveryOwner exists from initial acceptance until the
// Publisher Service stream retires. It can own the next valid recovery capsule
// before the native Connection detects the failed Carrier. All slot transitions
// run under dutyContext.mu; completion and timer joins run after unlocking.
type RecoveryOwner struct {
	binding    RecoveryBinding
	generation uint64
	delivery   chan RoutedDelivery
	expiryStop context.CancelFunc
	expiryDone chan struct{}
}

// NewRecoveryOwner creates the one recovery slot of a binding. The shared
// Context lock makes creation and dispatcher registration one transition; the
// caller registers the result through Dispatch.AddRecoveryLocked.
func NewRecoveryOwner(binding RecoveryBinding) *RecoveryOwner {
	return &RecoveryOwner{binding: binding, generation: 2,
		delivery: make(chan RoutedDelivery, 1)}
}

// OwnedBy reports whether the slot was constructed for exactly this binding.
func (recovery *RecoveryOwner) OwnedBy(binding RecoveryBinding) bool {
	return recovery != nil && recovery.binding == binding
}

// AdmitGenerationLocked advances only the next generation of this exact
// binding. A stale or speculative later waiter cannot change which capsule
// the recovery slot may retain.
func (recovery *RecoveryOwner) AdmitGenerationLocked(binding RecoveryBinding,
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

// AcceptsLocked reports whether the claimed key belongs to this live slot and
// its buffer is free.
func (recovery *RecoveryOwner) AcceptsLocked(key DeliveryKey) bool {
	return recovery != nil && recovery.binding.OwnsRecoveryLocked(recovery) &&
		len(recovery.delivery) == 0 && recovery.binding.ConnectionNonce() == key.Connection &&
		key.Generation >= recovery.generation && key.Generation <= recovery.generation+1
}

// HandoffLocked transfers a buffered capsule to its exact waiter and returns
// the expiry join. The caller must wait for that join after dropping Context's
// lock, so an expiring capsule cannot also be completed by the waiter.
func (recovery *RecoveryOwner) HandoffLocked(want DeliveryKey,
	waiter *Waiter) <-chan struct{} {
	select {
	case routed := <-recovery.delivery:
		if routed.Key.Matches(want) {
			var expiryDone <-chan struct{}
			if recovery.expiryStop != nil {
				recovery.expiryStop()
				expiryDone = recovery.expiryDone
				recovery.expiryStop, recovery.expiryDone = nil, nil
			}
			waiter.Delivery <- routed
			return expiryDone
		}
		recovery.delivery <- routed
	default:
	}
	return nil
}

// BufferLocked retains only one capsule for a live recovery owner and starts
// its deadline refusal. The shared Context lock makes routing and retention
// one transition with waiter selection.
func (recovery *RecoveryOwner) BufferLocked(host Host, routed RoutedDelivery) bool {
	if recovery == nil || !recovery.binding.OwnsRecoveryLocked(recovery) ||
		len(recovery.delivery) != 0 || recovery.expiryDone != nil || routed.Delivery == nil ||
		!host.Now().Add(ExpiryReserve).Before(routed.Expires) {
		return false
	}
	lifetime, stop := context.WithCancel(host.LeaseContext())
	done := make(chan struct{})
	recovery.delivery <- routed
	recovery.expiryStop, recovery.expiryDone = stop, done
	go recovery.expire(host, lifetime, stop, routed, done)
	return true
}

func (recovery *RecoveryOwner) expire(host Host, ctx context.Context,
	stop context.CancelFunc, routed RoutedDelivery, done chan struct{}) {
	defer stop()
	defer func() {
		host.Mu().Lock()
		if recovery.expiryDone == done {
			recovery.expiryStop, recovery.expiryDone = nil, nil
		}
		host.Mu().Unlock()
		close(done)
	}()
	wait := routed.Expires.Sub(host.Now()) - ExpiryReserve
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
	host.Mu().Lock()
	if recovery.expiryDone != done || !recovery.binding.OwnsRecoveryLocked(recovery) {
		host.Mu().Unlock()
		return
	}
	var expired RoutedDelivery
	select {
	case expired = <-recovery.delivery:
	default:
	}
	lifetime := host.LeaseContext()
	host.Mu().Unlock()
	if expired.Delivery == nil || expired.Delivery != routed.Delivery || lifetime.Err() != nil {
		return
	}
	bounded, cancel := context.WithDeadline(lifetime, routed.Expires)
	err := expired.Delivery.Complete(bounded, 1)
	cancel()
	if err != nil && lifetime.Err() == nil {
		host.FailDutyContexts(err)
	}
}

// RetireLocked removes the buffered capsule and interrupts deadline refusal.
// The caller joins the returned channel before completing the capsule from its
// own lifetime, keeping the shared Publisher registration alive for other
// streams.
func (recovery *RecoveryOwner) RetireLocked() (<-chan struct{}, RoutedDelivery) {
	expiryStop, expiryDone := recovery.expiryStop, recovery.expiryDone
	recovery.expiryStop, recovery.expiryDone = nil, nil
	if expiryStop != nil {
		expiryStop()
	}
	var routed RoutedDelivery
	select {
	case routed = <-recovery.delivery:
	default:
	}
	return expiryDone, routed
}
