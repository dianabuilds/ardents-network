package route

import (
	"bytes"
	"sync"
	"testing"
	"time"
)

func TestClosedForwardingChannelBoundsAuthorizedOddChild(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	limits, err := NewClosedDutyLimits(func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	reservation, err := limits.reserveChannel()
	if err != nil {
		t.Fatal(err)
	}
	lease := ClosedAdmission{Class: 2, Bytes: 32 << 20, Deadline: now.Add(time.Minute), duty: reservation}
	allowed := ClosedOpen{NextNodeID: [32]byte{1}, NextDutyGeneration: 2, Purpose: ClosedPurposeForwarding, Deadline: now.Add(30 * time.Second)}
	channel, err := NewClosedForwardingChannel(&lease, func(open ClosedOpen) error {
		if open != allowed {
			return errUnexpectedForwardOpen
		}
		return nil
	}, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	lease.Release()
	body, err := EncodeClosedOpen(allowed)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := channel.Accept(ClosedLaneFrame{Kind: closedFrameOpen, Lane: 1, Body: body}); err != nil {
		t.Fatal(err)
	}
	if event, available := channel.Next(); !available || event.Kind != closedFrameOpen || event.Open != allowed || event.Restriction != ClosedChildOrdinary {
		t.Fatalf("scheduled open = %+v / %t", event, available)
	}
	bytesFrame := ClosedLaneFrame{Kind: closedFrameBytes, Lane: 1, Body: bytes.Repeat([]byte{3}, int(closedLaneCredit))}
	if _, err := channel.Accept(bytesFrame); err != nil {
		t.Fatal(err)
	}
	if _, err := channel.Accept(ClosedLaneFrame{Kind: closedFrameBytes, Lane: 1, Body: []byte{1}}); err == nil {
		t.Fatal("accepted bytes beyond fixed credit")
	}
	if event, available := channel.Next(); !available || event.Kind != closedFrameBytes || len(event.Bytes) != int(closedLaneCredit) {
		t.Fatalf("scheduled bytes = %+v / %t", event, available)
	}
	credit, err := channel.Credit(1, 1)
	if err != nil || credit.Kind != closedFrameCredit {
		t.Fatalf("credit = %+v / %v", credit, err)
	}
	if _, err := channel.Accept(ClosedLaneFrame{Kind: closedFrameBytes, Lane: 1, Body: []byte{1}}); err != nil {
		t.Fatal(err)
	}
	if _, err := channel.Accept(ClosedLaneFrame{Kind: closedFrameOpen, Lane: 1, Body: body}); err == nil {
		t.Fatal("reused child lane")
	}
	if _, err := channel.Accept(ClosedLaneFrame{Kind: closedFrameOpen, Lane: 2, Body: body}); err == nil {
		t.Fatal("accepted endpoint even lane")
	}
	if _, err := channel.Accept(ClosedLaneFrame{Kind: closedFrameEOF, Lane: 1}); err != nil {
		t.Fatal(err)
	}
	if event, available := channel.Next(); !available || event.Kind != closedFrameBytes || len(event.Bytes) != 1 {
		t.Fatalf("scheduled trailing bytes = %+v / %t", event, available)
	}
	if event, available := channel.Next(); !available || event.Kind != closedFrameEOF {
		t.Fatalf("scheduled EOF = %+v / %t", event, available)
	}
	if _, err := channel.Accept(ClosedLaneFrame{Kind: closedFrameBytes, Lane: 1, Body: []byte{1}}); err == nil {
		t.Fatal("accepted bytes after EOF")
	}
}

func TestClosedForwardingChannelSerializesConcurrentLaneReuse(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	limits, err := NewClosedDutyLimits(func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	reservation, err := limits.reserveChannel()
	if err != nil {
		t.Fatal(err)
	}
	allowed := ClosedOpen{NextNodeID: [32]byte{11}, NextDutyGeneration: 12, Purpose: ClosedPurposeForwarding, Deadline: now.Add(time.Minute)}
	lease := ClosedAdmission{Class: 2, Bytes: 32 << 20, Deadline: now.Add(time.Minute), duty: reservation}
	channel, err := NewClosedForwardingChannel(&lease,
		func(open ClosedOpen) error {
			if open != allowed {
				return errUnexpectedForwardOpen
			}
			return nil
		}, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	body, err := EncodeClosedOpen(allowed)
	if err != nil {
		t.Fatal(err)
	}
	var group sync.WaitGroup
	errs := make(chan error, 2)
	for range 2 {
		group.Add(1)
		go func() {
			defer group.Done()
			_, err := channel.Accept(ClosedLaneFrame{Kind: closedFrameOpen, Lane: 1, Body: body})
			errs <- err
		}()
	}
	group.Wait()
	close(errs)
	succeeded := 0
	for err := range errs {
		if err == nil {
			succeeded++
		}
	}
	if succeeded != 1 {
		t.Fatalf("concurrent lane reuse successes = %d", succeeded)
	}
	channel.Cancel()
}

func TestClosedForwardingChannelBoundsOnePrefixQueueBeforeDutyQueue(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	limits, err := NewClosedDutyLimits(func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	reservation, err := limits.reserveChannel()
	if err != nil {
		t.Fatal(err)
	}
	lease := ClosedAdmission{Class: 2, Bytes: 32 << 20, Deadline: now.Add(time.Minute), duty: reservation}
	allowed := ClosedOpen{NextNodeID: [32]byte{21}, NextDutyGeneration: 22, Purpose: ClosedPurposeForwarding, Deadline: now.Add(time.Minute)}
	channel, err := NewClosedForwardingChannel(&lease, func(open ClosedOpen) error {
		if open != allowed {
			return errUnexpectedForwardOpen
		}
		return nil
	}, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	defer channel.Cancel()
	openBody, err := EncodeClosedOpen(allowed)
	if err != nil {
		t.Fatal(err)
	}
	frame := ClosedLaneFrame{Kind: closedFrameBytes, Body: bytes.Repeat([]byte{4}, int(closedLaneCredit))}
	for index := uint32(0); index < uint32(closedPrefixQueueBytes/closedLaneCredit); index++ {
		lane := index*2 + 1
		if _, err := channel.Accept(ClosedLaneFrame{Kind: closedFrameOpen, Lane: lane, Body: openBody}); err != nil {
			t.Fatalf("open lane %d: %v", lane, err)
		}
		frame.Lane = lane
		if _, err := channel.Accept(frame); err != nil {
			t.Fatalf("queue lane %d: %v", lane, err)
		}
	}
	lane := uint32(closedPrefixQueueBytes/closedLaneCredit)*2 + 1
	if _, err := channel.Accept(ClosedLaneFrame{Kind: closedFrameOpen, Lane: lane, Body: openBody}); err != nil {
		t.Fatal(err)
	}
	frame.Lane = lane
	if _, err := channel.Accept(frame); err == nil {
		t.Fatal("accepted bytes over the forwarding prefix queue")
	}
}

func TestClosedForwardingChannelSchedulesControlThenRoundRobinData(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	limits, err := NewClosedDutyLimits(func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	reservation, err := limits.reserveChannel()
	if err != nil {
		t.Fatal(err)
	}
	lease := ClosedAdmission{Class: 2, Bytes: 32 << 20, Deadline: now.Add(time.Minute), duty: reservation}
	allowed := ClosedOpen{NextNodeID: [32]byte{31}, NextDutyGeneration: 32, Purpose: ClosedPurposeForwarding, Deadline: now.Add(time.Minute)}
	channel, err := NewClosedForwardingChannel(&lease, func(open ClosedOpen) error {
		if open != allowed {
			return errUnexpectedForwardOpen
		}
		return nil
	}, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	defer channel.Cancel()
	openBody, err := EncodeClosedOpen(allowed)
	if err != nil {
		t.Fatal(err)
	}
	for _, lane := range []uint32{1, 3} {
		if _, err := channel.Accept(ClosedLaneFrame{Kind: closedFrameOpen, Lane: lane, Body: openBody}); err != nil {
			t.Fatal(err)
		}
	}
	for _, lane := range []uint32{1, 3} {
		event, available := channel.Next()
		if !available || event.Kind != closedFrameOpen || event.Lane != lane {
			t.Fatalf("control schedule = %+v / %t", event, available)
		}
	}
	for _, frame := range []ClosedLaneFrame{
		{Kind: closedFrameBytes, Lane: 1, Body: []byte{1}},
		{Kind: closedFrameBytes, Lane: 3, Body: []byte{3}},
		{Kind: closedFrameBytes, Lane: 1, Body: []byte{2}},
	} {
		if _, err := channel.Accept(frame); err != nil {
			t.Fatal(err)
		}
	}
	for _, want := range []struct {
		lane  uint32
		value byte
	}{{1, 1}, {3, 3}, {1, 2}} {
		event, available := channel.Next()
		if !available || event.Kind != closedFrameBytes || event.Lane != want.lane || len(event.Bytes) != 1 || event.Bytes[0] != want.value {
			t.Fatalf("round-robin schedule = %+v / %t", event, available)
		}
	}
	if _, err := channel.Accept(ClosedLaneFrame{Kind: closedFrameClose, Lane: 1, Body: []byte{5}}); err != nil {
		t.Fatal(err)
	}
	if event, available := channel.Next(); !available || event.Kind != closedFrameClose || event.Lane != 1 || len(event.Bytes) != 1 || event.Bytes[0] != 5 {
		t.Fatalf("scheduled close = %+v / %t", event, available)
	}
}

var errUnexpectedForwardOpen = &unexpectedForwardOpen{}

type unexpectedForwardOpen struct{}

func (*unexpectedForwardOpen) Error() string { return "unexpected forwarding OPEN" }
