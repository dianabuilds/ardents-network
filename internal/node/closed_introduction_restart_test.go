//go:build linux

package node

import (
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

func TestClosedIntroductionRestartDoesNotReviveWithdrawnSlot(t *testing.T) {
	for _, carrier := range []route.CarrierProfile{route.ClosedCarrierTCP, route.ClosedCarrierQUIC} {
		t.Run(string(carrier), func(t *testing.T) {
			fixture := newPrivateRecipientNetworkFixture(t, carrier, route.ClosedPurposeIntroduction, 3)
			request := route.ClosedRegistrationRequest{Nonce: [32]byte{111}, Slot: [32]byte{112}, Revision: 1, Expiry: time.Now().UTC().Add(60 * time.Second).Truncate(time.Second)}
			connection, closeFirst, status := registerIntroductionFixture(t, fixture, 0, request)
			if status != 0 {
				t.Fatal("initial registration refused")
			}
			withdraw := route.ClosedRegistrationRequest{Nonce: [32]byte{113}, Slot: request.Slot, Revision: request.Revision, Withdraw: true}
			if sendRegistrationFixture(t, connection, withdraw) != 0 {
				t.Fatal("withdrawal refused")
			}
			closeFirst()
			fixture.restart()
			request.Nonce[0]++
			_, closeReclaim, status := registerIntroductionFixture(t, fixture, 1, request)
			closeReclaim()
			if status != 1 {
				t.Fatal("restart revived a withdrawn slot with a fresh token")
			}
			request.Slot[0]++
			request.Nonce[0]++
			_, closeFresh, status := registerIntroductionFixture(t, fixture, 2, request)
			closeFresh()
			if status != 0 {
				t.Fatal("restarted recipient refused a fresh slot")
			}
			fixture.restart()
			request.Nonce[0]++
			_, closeLost, status := registerIntroductionFixture(t, fixture, 3, request)
			closeLost()
			if status != 1 {
				t.Fatal("restart revived a slot after carrier loss")
			}
		})
	}
}
