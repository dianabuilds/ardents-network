package route

import (
	"bytes"
	"testing"
	"time"
)

func TestClosedForwardingControlQueueCountsCompleteFrames(t *testing.T) {
	_, _, lease, now := closedOuterAdmissionFixture(t)
	channel, err := NewClosedForwardingChannel(lease, func(ClosedOpen) error { return nil }, func() time.Time { return *now })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(channel.Cancel)
	open := ClosedOpen{NextNodeID: [32]byte{1}, NextDutyGeneration: 1, Purpose: ClosedPurposeForwarding, Deadline: now.Add(time.Minute)}
	body, err := EncodeClosedOpen(open)
	if err != nil {
		t.Fatal(err)
	}
	// Each canonical OPEN occupies 16 header bytes plus 50 body bytes.
	// 248 fit in the selected 16 KiB control queue; a 249th does not.
	for lane := uint32(1); lane < 497; lane += 2 {
		if _, err := channel.Accept(ClosedLaneFrame{Kind: closedFrameOpen, Lane: lane, Body: body}); err != nil {
			t.Fatalf("admit lane %d: %v", lane, err)
		}
	}
	if _, err := channel.Accept(ClosedLaneFrame{Kind: closedFrameOpen, Lane: 497, Body: body}); err == nil {
		t.Fatal("control queue admitted more than 16 KiB of complete frames")
	}
	if event, ok := channel.Next(); !ok || event.Kind != closedFrameOpen || event.Lane != 1 {
		t.Fatal("control pressure lost previously admitted work")
	}
	if _, err := channel.Accept(ClosedLaneFrame{Kind: closedFrameOpen, Lane: 499, Body: body}); err != nil {
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
		lease := &ClosedAdmission{Class: 2, Bytes: 32 << 20, Deadline: now.Add(time.Minute), duty: reservation}
		channel, err := NewClosedForwardingChannel(lease, func(ClosedOpen) error { return nil }, func() time.Time { return now })
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(channel.Cancel)
		channels = append(channels, channel)
	}
	body, err := EncodeClosedOpen(ClosedOpen{NextNodeID: [32]byte{1}, NextDutyGeneration: 1, Purpose: ClosedPurposeForwarding, Deadline: now.Add(time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	data := bytes.Repeat([]byte{1}, 16<<10)
	for _, channel := range channels {
		for lane := uint32(1); lane < 128; lane += 2 {
			if _, err := channel.Accept(ClosedLaneFrame{Kind: closedFrameOpen, Lane: lane, Body: body}); err != nil {
				t.Fatalf("admit control during data pressure: %v", err)
			}
			if event, ok := channel.Next(); !ok || event.Kind != closedFrameOpen || event.Lane != lane {
				t.Fatal("data pressure prevented control progress")
			}
			for range 4 {
				// Filling is allowed to stop at the ancestor's data limit.
				if _, err := channel.Accept(ClosedLaneFrame{Kind: closedFrameBytes, Lane: lane, Body: data}); err != nil {
					break
				}
			}
		}
	}
	for _, channel := range channels {
		if err := channel.QueueReverse(ClosedLaneFrame{Kind: closedFrameClose, Lane: 3, Body: []byte{0}}); err != nil {
			t.Fatalf("data consumed reverse termination reserve: %v", err)
		}
		if _, err := channel.Accept(ClosedLaneFrame{Kind: closedFrameClose, Lane: 1, Body: []byte{0}}); err != nil {
			t.Fatalf("data consumed an admitted channel's termination reserve: %v", err)
		}
		if event, ok := channel.Next(); !ok || event.Kind != closedFrameClose || event.Lane != 1 {
			t.Fatal("termination did not precede queued data")
		}
	}
}
