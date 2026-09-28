//go:build linux

package introduction

import (
	"context"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route/client"
)

// recoveryTestBinding is the in-package stand-in for the root serviceBinding:
// the recovery slot only sees the RecoveryBinding seam.
type recoveryTestBinding struct {
	nonce    [32]byte
	recovery *RecoveryOwner
}

func (binding *recoveryTestBinding) OwnsRecoveryLocked(recovery *RecoveryOwner) bool {
	return binding != nil && recovery != nil && binding.recovery == recovery
}

func (binding *recoveryTestBinding) ConnectionNonce() [32]byte {
	return binding.nonce
}

func TestTextIntroductionRecoveryHandoffOwnsBufferedDelivery(t *testing.T) {
	key := DeliveryKey{Generation: 2}
	key.Connection[0] = 1
	routed := RoutedDelivery{Delivery: &client.ClosedIntroductionDelivery{}, Key: key}
	recovery := &RecoveryOwner{delivery: make(chan RoutedDelivery, 1)}
	recovery.delivery <- routed
	_, stop := context.WithCancel(t.Context())
	done := make(chan struct{})
	recovery.expiryStop, recovery.expiryDone = stop, done
	waiter := &Waiter{Delivery: make(chan RoutedDelivery, 1)}

	wrong := key
	wrong.Connection[0] = 2
	if joined := recovery.HandoffLocked(wrong, waiter); joined != nil {
		t.Fatal("foreign waiter took the recovery expiry join")
	}
	if len(recovery.delivery) != 1 || len(waiter.Delivery) != 0 {
		t.Fatal("foreign waiter changed buffered delivery ownership")
	}
	joined := recovery.HandoffLocked(key, waiter)
	if joined != done || recovery.expiryStop != nil || recovery.expiryDone != nil {
		t.Fatal("matching waiter did not take the expiry join")
	}
	if len(recovery.delivery) != 0 || len(waiter.Delivery) != 1 || (<-waiter.Delivery).Delivery != routed.Delivery {
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
	routed := RoutedDelivery{Delivery: &client.ClosedIntroductionDelivery{}}
	recovery := &RecoveryOwner{delivery: make(chan RoutedDelivery, 1)}
	recovery.delivery <- routed
	ctx, stop := context.WithCancel(t.Context())
	done := make(chan struct{})
	recovery.expiryStop, recovery.expiryDone = stop, done

	joined, retired := recovery.RetireLocked()
	if joined != done || retired.Delivery != routed.Delivery || len(recovery.delivery) != 0 {
		t.Fatal("retirement did not take the exact buffered delivery and expiry join")
	}
	if ctx.Err() != context.Canceled || recovery.expiryStop != nil || recovery.expiryDone != nil {
		t.Fatal("retirement left the expiry owner active")
	}
}

func TestTextIntroductionRecoveryKeepsExactBindingAndGeneration(t *testing.T) {
	binding := &recoveryTestBinding{}
	binding.nonce[0] = 1
	recovery := &RecoveryOwner{binding: binding, generation: 2,
		delivery: make(chan RoutedDelivery, 1)}
	binding.recovery = recovery
	key := DeliveryKey{Connection: binding.nonce, Generation: 2}
	if !recovery.AcceptsLocked(key) || !recovery.AdmitGenerationLocked(binding, 3) {
		t.Fatal("exact recovery owner rejected its next generation")
	}
	if recovery.generation != 3 {
		t.Fatal("next generation was not retained")
	}
	if recovery.AdmitGenerationLocked(&recoveryTestBinding{}, 4) || recovery.AdmitGenerationLocked(binding, 5) ||
		recovery.AdmitGenerationLocked(binding, 2) || recovery.generation != 3 {
		t.Fatal("foreign, skipped, or stale waiter changed the recovery generation")
	}
	if recovery.AcceptsLocked(key) {
		t.Fatal("stale capsule matched the advanced recovery owner")
	}
	key.Generation = 4
	key.Connection[0] = 2
	if recovery.AcceptsLocked(key) {
		t.Fatal("foreign Connection matched the recovery owner")
	}
	key.Connection = binding.nonce
	if !recovery.AcceptsLocked(key) {
		t.Fatal("exact next capsule was not eligible for buffering")
	}
}
