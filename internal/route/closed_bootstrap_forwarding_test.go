//go:build linux

package route

import (
	"bytes"
	"encoding/binary"
	"testing"
	"time"
)

func TestClosedBootstrapForwardingOwnsRealReservationAndRefusesPrivateUpgrade(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	clock := func() time.Time { return now }
	governor, err := NewClosedBootstrapController(clock)
	if err != nil {
		t.Fatal(err)
	}
	limits, err := NewClosedDutyLimits(clock)
	if err != nil {
		t.Fatal(err)
	}
	lease, err := governor.Admit([32]byte{1}, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	channel, err := NewClosedBootstrapForwardingChannel(lease, limits, func(ClosedOpen) error { return nil }, clock)
	if err != nil {
		t.Fatal(err)
	}
	defer channel.Cancel()
	lease.Release()
	if _, err := NewClosedBootstrapForwardingChannel(lease, limits, func(ClosedOpen) error { return nil }, clock); err == nil {
		t.Fatal("bootstrap reservation transferred twice")
	}
	if len(governor.leases) != 1 || limits.channels != 1 {
		t.Fatal("source release removed transferred pressure")
	}
	private, _ := EncodeClosedOpen(ClosedOpen{NextNodeID: [32]byte{2}, NextDutyGeneration: 1, Purpose: ClosedPurposeDataJoin, Deadline: now.Add(time.Second)})
	if _, err := channel.Accept(ClosedLaneFrame{Kind: 4, Lane: 1, Body: private}); err == nil {
		t.Fatal("bootstrap admitted private purpose")
	}
	if _, err := channel.Accept(ClosedLaneFrame{Kind: 2, Lane: 0, Body: []byte{2}}); err == nil {
		t.Fatal("bootstrap silently upgraded to ADMIT")
	}
	open, _ := EncodeClosedOpen(ClosedOpen{NextNodeID: [32]byte{3}, NextDutyGeneration: 2, Purpose: ClosedPurposeIssuer, Deadline: now.Add(10 * time.Second)})
	if _, err := channel.Accept(ClosedLaneFrame{Kind: 4, Lane: 1, Body: open}); err != nil {
		t.Fatal(err)
	}
	if governor.queued == 0 || limits.queued == 0 {
		t.Fatal("OPEN control queue is unaccounted")
	}
	if event, ok := channel.Next(); !ok || event.Restriction != ClosedChildIssuerBootstrap {
		t.Fatal("OPEN missing")
	}
	if governor.queued != 0 || limits.queued != 0 {
		t.Fatal("consumed OPEN retained queue")
	}
	frame := ClosedLaneFrame{Kind: 6, Lane: 1, Body: bytes.Repeat([]byte{1}, 16<<10)}
	if err := channel.QueueReverse(frame); err != nil {
		t.Fatal(err)
	}
	if err := channel.AccountOutput(frame); err != nil {
		t.Fatal(err)
	}
	channel.ReleaseReverse(1, uint64(16+len(frame.Body)))
	credit := ClosedLaneFrame{Kind: 7, Lane: 1, Body: binary.BigEndian.AppendUint32(nil, uint32(len(frame.Body)))}
	if _, err := channel.Accept(credit); err != nil {
		t.Fatal(err)
	}
	if event, ok := channel.Next(); !ok || event.Kind != 7 {
		t.Fatal("consumed reverse credit not forwarded")
	}
	if _, err := channel.Accept(credit); err == nil {
		t.Fatal("duplicate credit enlarged receive window")
	}
	now = now.Add(10 * time.Second)
	if _, err := channel.Accept(ClosedLaneFrame{Kind: 6, Lane: 1, Body: []byte{1}}); err == nil {
		t.Fatal("expired bootstrap accepted data")
	}
	if err := channel.AccountOutput(frame); err == nil {
		t.Fatal("expired bootstrap accepted output")
	}
	channel.Cancel()
	channel.Cancel()
	if limits.queued != 0 || limits.channels != 0 || limits.children != 0 || governor.queued != 0 || len(governor.leases) != 0 {
		t.Fatal("bootstrap cleanup retained capacity")
	}
}

func TestClosedBootstrapForwardingSharesQueueAcrossBothDirectionsAndDuties(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	clock := func() time.Time { return now }
	governor, _ := NewClosedBootstrapController(clock)
	limits, _ := NewClosedDutyLimits(clock)
	var channels []*ClosedForwardingChannel
	for index := byte(1); index <= 4; index++ {
		lease, err := governor.Admit([32]byte{index}, now.Add(time.Second))
		if err != nil {
			t.Fatal(err)
		}
		channel, err := NewClosedBootstrapForwardingChannel(lease, limits, func(ClosedOpen) error { return nil }, clock)
		if err != nil {
			t.Fatal(err)
		}
		channels = append(channels, channel)
		defer channel.Cancel()
		body, _ := EncodeClosedOpen(ClosedOpen{NextNodeID: [32]byte{8}, NextDutyGeneration: 1, Purpose: ClosedPurposeIssuer, Deadline: now.Add(time.Second)})
		if _, err := channel.Accept(ClosedLaneFrame{Kind: 4, Lane: 1, Body: body}); err != nil {
			t.Fatal(err)
		}
		channel.Next()
		if _, err := channel.Accept(ClosedLaneFrame{Kind: 6, Lane: 1, Body: bytes.Repeat([]byte{1}, 32<<10)}); err != nil {
			t.Fatal(err)
		}
		// Together with input this consumes exactly 64 KiB in the shared queue.
		if err := channel.QueueReverse(ClosedLaneFrame{Kind: 6, Lane: 1, Body: bytes.Repeat([]byte{2}, (32<<10)-16)}); err != nil {
			t.Fatal(err)
		}
	}
	if governor.queued != 256<<10 {
		t.Fatalf("shared queue = %d", governor.queued)
	}
	if err := channels[0].QueueReverse(ClosedLaneFrame{Kind: 6, Lane: 1, Body: []byte{1}}); err == nil {
		t.Fatal("shared bootstrap queue exceeded")
	}
	for _, channel := range channels {
		channel.Cancel()
	}
	if governor.queued != 0 || limits.queued != 0 {
		t.Fatal("cancel retained aggregate queue")
	}
}
