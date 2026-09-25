package route

import (
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route/ardp"
)

func TestForwardingControlReservationSurvivesFullWorkSet(t *testing.T) {
	limits, err := NewClosedDutyLimits(time.Now)
	if err != nil {
		t.Fatal(err)
	}
	channel, err := limits.reserveChannel()
	if err != nil {
		t.Fatal(err)
	}
	defer channel.release()
	for range 256 {
		if reserved, err := channel.reserveChildCapacity(false); err != nil || reserved {
			t.Fatalf("work: %t %v", reserved, err)
		}
	}
	if _, err := channel.reserveChildCapacity(false); err == nil {
		t.Fatal("257th work lane accepted")
	}
	for range 2 {
		if reserved, err := channel.reserveChildCapacity(true); err != nil || !reserved {
			t.Fatalf("control reserve: %t %v", reserved, err)
		}
	}
	if _, err := channel.reserveChildCapacity(true); err == nil {
		t.Fatal("third reserved control lane accepted")
	}
	channel.releaseChildCapacity(true)
	if reserved, err := channel.reserveChildCapacity(true); err != nil || !reserved {
		t.Fatalf("control capacity not returned: %t %v", reserved, err)
	}
	channel.releaseChildCapacity(false)
	if reserved, err := channel.reserveChildCapacity(false); err != nil || reserved {
		t.Fatalf("work capacity not returned: %t %v", reserved, err)
	}
	channel.release()
	if limits.children != 0 {
		t.Fatal("channel retirement retained child reservations")
	}
}

func TestForwardingDataCannotClaimReservedControl(t *testing.T) {
	for _, purpose := range []ardp.Purpose{ardp.PurposeForwarding, ardp.PurposeDataJoin, ardp.PurposeName, 0, 255} {
		if closedControlPurpose(purpose) {
			t.Fatalf("purpose %d obtained control capacity", purpose)
		}
	}
}
