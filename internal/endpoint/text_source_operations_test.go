//go:build linux

package endpoint

import (
	"context"
	"github.com/dianabuilds/ardents-network/internal/route"
	"testing"
	"time"
)

func TestTextRefreshWaitsForActualSourceUse(t *testing.T) {
	for _, carrier := range []route.CarrierProfile{route.ClosedCarrierTCP, route.ClosedCarrierQUIC} {
		t.Run(string(carrier), func(t *testing.T) {
			_, owner, _ := textJoinedNetworkFixture(t, carrier)
			release, err := owner.acquireTextSourceOperation(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			held := true
			defer func() {
				if held {
					release()
				}
			}()
			owner.mu.Lock()
			first, refresh := owner.registration, owner.refresh
			first.refreshAt = time.Now().Add(-time.Second)
			owner.signalTextRegistrationsLocked()
			owner.mu.Unlock()
			// The scheduler may begin, but cannot treat legitimate Source ownership
			// as lost publication authority. Issue real tokens under that reservation.
			select {
			case <-refresh.done:
				t.Fatal("refresh ended while Source was in use")
			case <-time.After(40 * time.Millisecond):
			}
			selection := selectTextSource(t, owner)
			if err := owner.issueTextTokensForOpening(t.Context(), [][32]byte{selection.EntryNodeID}, 2, nil, false); err != nil {
				t.Fatal(err)
			}
			release()
			held = false
			waitTextRefreshCondition(t, owner, func() bool {
				return owner.registration != nil && owner.registration != first && !owner.registration.refreshAt.IsZero()
			})
			owner.mu.Lock()
			valid := owner.previousRegistration == first && refresh.err == nil && owner.permission.batches == 2
			owner.mu.Unlock()
			if !valid {
				t.Fatal("refresh lost original registration or repeated bootstrap")
			}
		})
	}
}

func TestTextSourceWaitCancellationDoesNotStealReservation(t *testing.T) {
	for _, revoke := range []bool{false, true} {
		name := "caller"
		if revoke {
			name = "context"
		}
		t.Run(name, func(t *testing.T) {
			_, owner, _ := textSourceContextFixture(t)
			release, err := owner.acquireTextSourceOperation(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			defer release()
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			result := make(chan error, 1)
			go func() {
				unlock, err := owner.acquireTextSourceOperation(ctx)
				if unlock != nil {
					unlock()
				}
				result <- err
			}()
			select {
			case <-result:
				t.Fatal("waiter acquired another owner's reservation")
			case <-time.After(20 * time.Millisecond):
			}
			if revoke {
				if err := owner.Close(); err != nil {
					t.Fatal(err)
				}
			} else {
				cancel()
			}
			select {
			case err := <-result:
				if err == nil {
					t.Fatal("cancelled waiter succeeded")
				}
			case <-time.After(time.Second):
				t.Fatal("cancelled waiter did not join")
			}
			owner.mu.Lock()
			occupied := len(owner.sourceOperations) == 1
			owner.mu.Unlock()
			if !occupied {
				t.Fatal("cancelled waiter released the active owner's reservation")
			}
		})
	}
}
