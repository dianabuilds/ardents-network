//go:build linux

package source

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestOperationReservationSurvivesCancellationAndRetirement(t *testing.T) {
	for _, cancelLease := range []bool{false, true} {
		name := "caller"
		if cancelLease {
			name = "lease"
		}
		t.Run(name, func(t *testing.T) {
			var lifecycle Lifecycle
			release, err := lifecycle.AcquireOperation(t.Context(), t.Context())
			if err != nil {
				t.Fatal(err)
			}
			held := true
			defer func() {
				if held {
					release()
				}
			}()

			caller, cancelCaller := context.WithCancel(t.Context())
			defer cancelCaller()
			lease, revokeLease := context.WithCancel(t.Context())
			defer revokeLease()
			result := make(chan error, 1)
			go func() {
				unlock, err := lifecycle.AcquireOperation(caller, lease)
				if unlock != nil {
					unlock()
				}
				result <- err
			}()
			select {
			case <-result:
				t.Fatal("waiter acquired the active reservation")
			case <-time.After(20 * time.Millisecond):
			}
			if cancelLease {
				revokeLease()
			} else {
				cancelCaller()
			}
			select {
			case err := <-result:
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("cancelled waiter = %v, want context cancellation", err)
				}
			case <-time.After(time.Second):
				t.Fatal("cancelled waiter did not return")
			}

			lifecycle.StopLocked()
			waitCtx, stopWaiting := context.WithTimeout(t.Context(), 20*time.Millisecond)
			defer stopWaiting()
			unlock, err := lifecycle.AcquireOperation(waitCtx, t.Context())
			if unlock != nil {
				unlock()
				t.Fatal("retirement released another caller's reservation")
			}
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("retired reservation wait = %v, want deadline", err)
			}

			release()
			held = false
			unlock, err = lifecycle.AcquireOperation(t.Context(), t.Context())
			if err != nil {
				t.Fatalf("reservation after release = %v", err)
			}
			unlock()
		})
	}
}
