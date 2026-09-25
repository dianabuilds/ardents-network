//go:build linux

package endpoint

import (
	"context"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

func TestTextIntroductionRecoveryHandoffOwnsBufferedDelivery(t *testing.T) {
	key := textIntroductionDeliveryKey{generation: 2}
	key.connection[0] = 1
	routed := textIntroductionRoutedDelivery{delivery: &route.ClosedIntroductionDelivery{}, key: key}
	recovery := &textIntroductionRecoveryOwner{delivery: make(chan textIntroductionRoutedDelivery, 1)}
	recovery.delivery <- routed
	_, stop := context.WithCancel(t.Context())
	done := make(chan struct{})
	recovery.expiryStop, recovery.expiryDone = stop, done
	waiter := &textIntroductionWaiter{delivery: make(chan textIntroductionRoutedDelivery, 1)}

	wrong := key
	wrong.connection[0] = 2
	if joined := recovery.handoffLocked(wrong, waiter); joined != nil {
		t.Fatal("foreign waiter took the recovery expiry join")
	}
	if len(recovery.delivery) != 1 || len(waiter.delivery) != 0 {
		t.Fatal("foreign waiter changed buffered delivery ownership")
	}
	joined := recovery.handoffLocked(key, waiter)
	if joined != done || recovery.expiryStop != nil || recovery.expiryDone != nil {
		t.Fatal("matching waiter did not take the expiry join")
	}
	if len(recovery.delivery) != 0 || len(waiter.delivery) != 1 || (<-waiter.delivery).delivery != routed.delivery {
		t.Fatal("matching waiter did not take the exact buffered delivery")
	}
	close(done)
	select {
	case <-joined:
	case <-time.After(time.Second):
		t.Fatal("expiry join did not complete")
	}
}

func TestTextIntroductionRecoveryRetirementOwnsBufferedDelivery(t *testing.T) {
	routed := textIntroductionRoutedDelivery{delivery: &route.ClosedIntroductionDelivery{}}
	recovery := &textIntroductionRecoveryOwner{delivery: make(chan textIntroductionRoutedDelivery, 1)}
	recovery.delivery <- routed
	ctx, stop := context.WithCancel(t.Context())
	done := make(chan struct{})
	recovery.expiryStop, recovery.expiryDone = stop, done

	joined, retired := recovery.retireLocked()
	if joined != done || retired.delivery != routed.delivery || len(recovery.delivery) != 0 {
		t.Fatal("retirement did not take the exact buffered delivery and expiry join")
	}
	if ctx.Err() != context.Canceled || recovery.expiryStop != nil || recovery.expiryDone != nil {
		t.Fatal("retirement left the expiry owner active")
	}
}
