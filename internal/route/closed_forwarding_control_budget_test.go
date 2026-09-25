package route

import (
	"bytes"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route/ardp"
)

func TestClosedForwardingControlQueueCountsCompleteFrames(t *testing.T) {
	_, _, lease, now := closedOuterAdmissionFixture(t)
	channel, err := newForwardingTestChannel(lease, func(ClosedOpen) error { return nil }, func() time.Time { return *now })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = channel.Cancel() })
	open := ClosedOpen{NextNodeID: [32]byte{1}, NextDutyGeneration: 1, Purpose: ardp.PurposeForwarding, Deadline: now.Add(time.Minute)}
	body, err := EncodeClosedOpen(open)
	if err != nil {
		t.Fatal(err)
	}
	// Each canonical OPEN occupies 16 header bytes plus 50 body bytes.
	// 248 fit in the selected 16 KiB control queue; a 249th does not.
	for lane := uint32(1); lane < 497; lane += 2 {
		if _, err := channel.Accept(ardp.Frame{Kind: ardp.KindOpen, Lane: lane, Body: body}); err != nil {
			t.Fatalf("admit lane %d: %v", lane, err)
		}
	}
	if _, err := channel.Accept(ardp.Frame{Kind: ardp.KindOpen, Lane: 497, Body: body}); err == nil {
		t.Fatal("control queue admitted more than 16 KiB of complete frames")
	}
	if event, ok := channel.NextAvailable(nil); !ok || event.Kind != ardp.KindOpen || event.Lane != 1 {
		t.Fatal("control pressure lost previously admitted work")
	}
	if _, err := channel.Accept(ardp.Frame{Kind: ardp.KindOpen, Lane: 499, Body: body}); err != nil {
		t.Fatalf("consumed control frame did not release capacity: %v", err)
	}
}

func TestClosedForwardingDataPressurePreservesEveryChannelControl(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	limits, err := NewClosedDutyLimits(func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	var channels []*ClosedForwardingChannel
	for index := 0; index < 16; index++ {
		// Reuse the same receiver governor, isolating its queue ownership
		// from credential verification already covered by admission tests.
		reservation, err := limits.reserveChannel()
		if err != nil {
			t.Fatal(err)
		}
		lease := &ClosedAdmission{Class: 2, Bytes: 32 << 20, Deadline: now.Add(time.Minute), claim: newClosedAdmissionClaim(reservation, nil)}
		channel, err := newForwardingTestChannel(lease, func(ClosedOpen) error { return nil }, func() time.Time { return now })
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = channel.Cancel() })
		channels = append(channels, channel)
	}
	body, err := EncodeClosedOpen(ClosedOpen{NextNodeID: [32]byte{1}, NextDutyGeneration: 1, Purpose: ardp.PurposeForwarding, Deadline: now.Add(time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	data := bytes.Repeat([]byte{1}, 16<<10)
	for _, channel := range channels {
		for lane := uint32(1); lane < 128; lane += 2 {
			if _, err := channel.Accept(ardp.Frame{Kind: ardp.KindOpen, Lane: lane, Body: body}); err != nil {
				t.Fatalf("admit control during data pressure: %v", err)
			}
			if event, ok := channel.NextAvailable(nil); !ok || event.Kind != ardp.KindOpen || event.Lane != lane {
				t.Fatal("data pressure prevented control progress")
			}
			for range 4 {
				// Filling is allowed to stop at the ancestor's data limit.
				if _, err := channel.Accept(ardp.Frame{Kind: ardp.KindBytes, Lane: lane, Body: data}); err != nil {
					break
				}
			}
		}
	}
	if reservation, err := limits.reserveChannel(); err == nil {
		reservation.release()
		t.Fatal("admitted a channel without room for its control reserve")
	}
	for _, channel := range channels {
		if err := channel.QueueReverse(ardp.Frame{Kind: ardp.KindClose, Lane: 3, Body: []byte{0}}); err != nil {
			t.Fatalf("data consumed reverse termination reserve: %v", err)
		}
		if _, err := channel.Accept(ardp.Frame{Kind: ardp.KindClose, Lane: 1, Body: []byte{0}}); err != nil {
			t.Fatalf("data consumed an admitted channel's termination reserve: %v", err)
		}
		if event, ok := channel.NextAvailable(nil); !ok || event.Kind != ardp.KindClose || event.Lane != 1 {
			t.Fatal("termination did not precede queued data")
		}
	}
	reservation, err := limits.reserveChannel()
	if err != nil {
		t.Fatal("released data capacity could not admit a channel reserve")
	}
	reservation.release()
}

func TestClosedForwardingControlDirectionsShareOneBound(t *testing.T) {
	_, _, lease, now := closedOuterAdmissionFixture(t)
	channel, err := newForwardingTestChannel(lease, func(ClosedOpen) error { return nil }, func() time.Time { return *now })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = channel.Cancel() })
	body, err := EncodeClosedOpen(ClosedOpen{NextNodeID: [32]byte{1}, NextDutyGeneration: 1, Purpose: ardp.PurposeForwarding, Deadline: now.Add(time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	for lane := uint32(1); lane <= 199; lane += 2 {
		if _, err := channel.Accept(ardp.Frame{Kind: ardp.KindOpen, Lane: lane, Body: body}); err != nil {
			t.Fatal(err)
		}
	}
	credit := ardp.Frame{Kind: ardp.KindCredit, Lane: 1, Body: []byte{0, 0, 0, 1}}
	// 100 outgoing Node OPEN frames use 6,600 bytes. Another 489 reverse
	// CREDIT frames use 9,780 bytes: 16,380 together, leaving only four.
	for range 489 {
		if err := channel.QueueReverse(credit); err != nil {
			t.Fatal(err)
		}
	}
	if err := channel.QueueReverse(credit); err == nil {
		t.Fatal("forward and reverse controls multiplied the 16 KiB reserve")
	}
	if event, ok := channel.NextAvailable(nil); !ok || event.Kind != ardp.KindOpen {
		t.Fatal("control pressure lost admitted OPEN")
	}
	// Releasing one 66-byte OPEN makes room for three 20-byte credits.
	for range 3 {
		if err := channel.QueueReverse(credit); err != nil {
			t.Fatal("forward consumption did not release shared control capacity")
		}
	}
	if err := channel.QueueReverse(credit); err == nil {
		t.Fatal("control release returned more than its complete frame size")
	}
}

func TestClosedForwardingLateControlReleaseCannotDebitSibling(t *testing.T) {
	_, _, lease, now := closedOuterAdmissionFixture(t)
	limits := lease.claim.duty.limits
	channel, err := newForwardingTestChannel(lease, func(ClosedOpen) error { return nil }, func() time.Time { return *now })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = channel.Cancel() })
	body, err := EncodeClosedOpen(ClosedOpen{NextNodeID: [32]byte{1}, NextDutyGeneration: 1, Purpose: ardp.PurposeForwarding, Deadline: now.Add(time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	for _, lane := range []uint32{1, 3} {
		if _, err := channel.Accept(ardp.Frame{Kind: ardp.KindOpen, Lane: lane, Body: body}); err != nil {
			t.Fatal(err)
		}
		if _, ok := channel.NextAvailable(nil); !ok {
			t.Fatal("missing OPEN")
		}
	}
	retired := ardp.Frame{Kind: ardp.KindClose, Lane: 1, Body: []byte{0}}
	if err := channel.QueueReverse(retired); err != nil {
		t.Fatal(err)
	}
	if _, err := channel.Accept(retired); err != nil {
		t.Fatal(err)
	}
	if _, ok := channel.NextAvailable(nil); !ok {
		t.Fatal("missing CLOSE")
	}
	credit := ardp.Frame{Kind: ardp.KindCredit, Lane: 3, Body: []byte{0, 0, 0, 1}}
	for range 819 {
		if err := channel.QueueReverse(credit); err != nil {
			t.Fatal(err)
		}
	}
	channel.ReleaseReverse(retired)
	if err := channel.QueueReverse(credit); err == nil {
		t.Fatal("late closed-lane writer released sibling control capacity")
	}
	channel.ReleaseReverse(credit)
	if err := channel.QueueReverse(credit); err != nil {
		t.Fatal("live writer did not release its control capacity")
	}
	channel.Cancel()
	channel.Cancel()
	channel.ReleaseReverse(credit)
	// Only the fixture's outer channel survives. Cancel must return this
	// channel's complete reservation, including its outstanding controls.
	if err := limits.queue((64 << 20) - (16 << 10)); err != nil {
		t.Fatal("cancel retained control capacity in the ancestor")
	}
	limits.dequeue((64 << 20) - (16 << 10))
	reservation, err := limits.reserveChannel()
	if err != nil {
		t.Fatal("released capacity could not admit a successor channel")
	}
	reservation.release()
}
