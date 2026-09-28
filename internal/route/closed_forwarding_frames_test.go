package route

import (
	"bytes"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route/ardp"
)

func forwardingFramesTestChannel(t *testing.T) *ClosedForwardingChannel {
	t.Helper()
	now := time.Unix(1_800_000_000, 0).UTC()
	clock := func() time.Time { return now }
	limits, err := NewClosedDutyLimits(clock)
	if err != nil {
		t.Fatal(err)
	}
	reservation, err := limits.reserveChannel()
	if err != nil {
		t.Fatal(err)
	}
	lease := ClosedAdmission{Class: 2, Bytes: 32 << 20, Deadline: now.Add(time.Hour), claim: newClosedAdmissionClaim(reservation, nil)}
	channel, err := newForwardingTestChannel(&lease, func(ClosedOpen) error { return nil }, clock)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := channel.Cancel(); err != nil {
			t.Error(err)
		}
	})
	open, err := EncodeClosedOpen(ClosedOpen{NextNodeID: [32]byte{1}, NextDutyGeneration: 2,
		Purpose: ardp.PurposeForwarding, Deadline: now.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := channel.Accept(ardp.Frame{Kind: ardp.KindOpen, Lane: 1, Body: open}); err != nil {
		t.Fatal(err)
	}
	if event, ok := channel.NextAvailable(nil); !ok || event.Kind != ardp.KindOpen || event.Lane != 1 {
		t.Fatalf("OPEN event = %+v, %t", event, ok)
	}
	return channel
}

func TestClosedForwardingFramesKeepEmittedAndSkippedBytesStable(t *testing.T) {
	channel := forwardingFramesTestChannel(t)
	input := []byte{1, 2, 3}
	if _, err := channel.Accept(ardp.Frame{Kind: ardp.KindBytes, Lane: 1, Body: input}); err != nil {
		t.Fatal(err)
	}
	input[0] = 9
	if _, err := channel.Accept(ardp.Frame{Kind: ardp.KindBytes, Lane: 1, Body: []byte{4}}); err != nil {
		t.Fatal(err)
	}
	var skipped []byte
	if event, ok := channel.NextAvailable(func(event ClosedForwardingEvent) bool {
		skipped = event.Bytes
		return false
	}); ok || event.Kind != 0 {
		t.Fatalf("skipped work escaped queue: %+v, %t", event, ok)
	}
	first, ok := channel.NextAvailable(nil)
	if !ok || first.Kind != ardp.KindBytes || !bytes.Equal(first.Bytes, []byte{1, 2, 3}) || !bytes.Equal(skipped, first.Bytes) {
		t.Fatalf("first owned frame = %+v, %t", first, ok)
	}
	if cap(first.Bytes) != len(first.Bytes) {
		t.Fatalf("emitted frame exposes later slab bytes: len=%d cap=%d", len(first.Bytes), cap(first.Bytes))
	}
	if _, err := channel.Credit(1, uint32(len(first.Bytes))); err != nil {
		t.Fatal(err)
	}
	if _, err := channel.Accept(ardp.Frame{Kind: ardp.KindBytes, Lane: 1, Body: []byte{5}}); err != nil {
		t.Fatal(err)
	}
	if _, err := channel.Accept(ardp.Frame{Kind: ardp.KindClose, Lane: 1, Body: []byte{7}}); err != nil {
		t.Fatal(err)
	}
	if err := channel.Cancel(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first.Bytes, []byte{1, 2, 3}) || !bytes.Equal(skipped, first.Bytes) {
		t.Fatalf("emitted or skipped frame changed after credit, append, close and cancel: %v / %v", first.Bytes, skipped)
	}
}

func TestClosedForwardingFramesPreserveLargeBoundariesAndZeroRefusal(t *testing.T) {
	channel := forwardingFramesTestChannel(t)
	if _, err := channel.Accept(ardp.Frame{Kind: ardp.KindBytes, Lane: 1}); err == nil {
		t.Fatal("accepted an empty BYTES frame")
	}
	if child := channel.children[1]; child.frames.count != 0 || child.queued != 0 {
		t.Fatal("refused empty BYTES frame entered the queue")
	}
	for _, body := range [][]byte{{1}, bytes.Repeat([]byte{2}, 255), bytes.Repeat([]byte{3}, 4096), bytes.Repeat([]byte{4}, 16384)} {
		if _, err := channel.Accept(ardp.Frame{Kind: ardp.KindBytes, Lane: 1, Body: body}); err != nil {
			t.Fatal(err)
		}
	}
	for _, want := range [][]byte{{1}, bytes.Repeat([]byte{2}, 255), bytes.Repeat([]byte{3}, 4096), bytes.Repeat([]byte{4}, 16384)} {
		event, ok := channel.NextAvailable(nil)
		if !ok || event.Kind != ardp.KindBytes || !bytes.Equal(event.Bytes, want) || len(event.Bytes) != len(want) {
			t.Fatalf("frame boundary: got %+v, %t; want %d bytes", event, ok, len(want))
		}
	}

	full := forwardingFramesTestChannel(t)
	if _, err := full.Accept(ardp.Frame{Kind: ardp.KindBytes, Lane: 1, Body: bytes.Repeat([]byte{6}, int(closedLaneCredit))}); err != nil {
		t.Fatal(err)
	}
	event, ok := full.NextAvailable(nil)
	if !ok || event.Kind != ardp.KindBytes || len(event.Bytes) != int(closedLaneCredit) || event.Bytes[0] != 6 || event.Bytes[len(event.Bytes)-1] != 6 {
		t.Fatalf("direct 64-KiB frame boundary = %d bytes, %t", len(event.Bytes), ok)
	}
}

func TestClosedForwardingFramesReleaseOldSlabsDuringChurn(t *testing.T) {
	channel := forwardingFramesTestChannel(t)
	if _, err := channel.Accept(ardp.Frame{Kind: ardp.KindBytes, Lane: 1, Body: []byte{42}}); err != nil {
		t.Fatal(err)
	}
	var first []byte
	for index := range 20_000 {
		if _, err := channel.Accept(ardp.Frame{Kind: ardp.KindBytes, Lane: 1, Body: []byte{byte(index)}}); err != nil {
			t.Fatal(err)
		}
		event, ok := channel.NextAvailable(nil)
		if !ok || event.Kind != ardp.KindBytes || len(event.Bytes) != 1 {
			t.Fatalf("churn event %d = %+v, %t", index, event, ok)
		}
		if index == 0 {
			first = event.Bytes
		}
		if _, err := channel.Credit(1, 1); err != nil {
			t.Fatal(err)
		}
		child := channel.children[1]
		if child.frames.count != 1 || len(child.frames.slabs) > 2 {
			t.Fatalf("churn queue %d retains %d frames and %d slabs", index, child.frames.count, len(child.frames.slabs))
		}
	}
	if !bytes.Equal(first, []byte{42}) {
		t.Fatalf("previously emitted frame changed during churn: %v", first)
	}
}
