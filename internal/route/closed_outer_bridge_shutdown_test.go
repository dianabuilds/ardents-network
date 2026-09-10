package route

import (
	"io"
	"testing"
	"time"
)

func TestClosedOuterBridgeShutdownRetainsReservationsUntilHandlersJoin(t *testing.T) {
	now := time.Unix(1_800_300_000, 0).UTC()
	receiver := closedOuterHandshakeReceiver(now)
	limits, err := NewClosedDutyLimits(func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	outer, err := NewClosedOuterHandshake(receiver, limits, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	defer outer.Close()
	writes := 0
	bridge, err := NewClosedOuterBridge(outer, func(uint32, time.Time) error { return nil }, func(ClosedLaneFrame, func() time.Time) error { writes++; return nil })
	if err != nil {
		t.Fatal(err)
	}
	hello := ClosedHello{NetworkID: receiver.NetworkID, StateGeneration: receiver.StateGeneration, StateDigest: receiver.StateDigest,
		ProfileDigest: receiver.ProfileDigest, RecipientNodeID: receiver.NodeID, RecipientDutyGeneration: receiver.DutyGeneration,
		Purpose: ClosedPurposeForwarding, ChannelNonce: [32]byte{9}, Deadline: receiver.Deadline}
	body, err := EncodeClosedHello(hello)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bridge.Accept(ClosedLaneFrame{Kind: closedFrameHello, Body: body}); err != nil {
		t.Fatal(err)
	}
	body, err = EncodeClosedNodeOpen(ClosedOpen{NextNodeID: receiver.NodeID, NextDutyGeneration: receiver.DutyGeneration, Purpose: ClosedPurposeIssuer, Deadline: receiver.Deadline}, ClosedChildOrdinary)
	if err != nil {
		t.Fatal(err)
	}
	lane, err := bridge.Accept(ClosedLaneFrame{Kind: closedFrameOpen, Lane: 1, Body: body})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bridge.Accept(ClosedLaneFrame{Kind: closedFrameBytes, Lane: 1, Body: []byte{1}}); err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadFull(lane, make([]byte, 1)); err != nil {
		t.Fatal(err)
	}
	if err := lane.BeginInnerHello(); err != nil {
		t.Fatal(err)
	}
	if _, err := bridge.Accept(ClosedLaneFrame{Kind: closedFrameBytes, Lane: 1, Body: []byte{2, 3}}); err != nil {
		t.Fatal(err)
	}
	waiting, err := bridge.Accept(ClosedLaneFrame{Kind: closedFrameOpen, Lane: 3, Body: body})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { _, err := waiting.Read(make([]byte, 1)); done <- err }()
	bridge.Close()
	bridge.Close()
	select {
	case err := <-done:
		if err != io.EOF {
			t.Fatalf("reader: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("reader survived shutdown")
	}
	if _, err := lane.Read(make([]byte, 1)); err != io.EOF {
		t.Fatalf("buffer remained readable after shutdown: %v", err)
	}
	if _, err := lane.Write([]byte{1}); err == nil {
		t.Fatal("retired child wrote")
	}
	if _, err := bridge.Accept(ClosedLaneFrame{Kind: closedFrameOpen, Lane: 5, Body: body}); err == nil {
		t.Fatal("closed bridge allocated a child")
	}
	if err := lane.CloseWithStatus(0); err != nil {
		t.Fatal(err)
	}
	if writes != 1 {
		t.Fatalf("shutdown sent frames on retired transport: %d", writes)
	}
	if limits.channels != 1 || limits.children != 2 || limits.queued != 2 {
		t.Fatalf("cleanup released reservations too early: channels=%d children=%d queued=%d", limits.channels, limits.children, limits.queued)
	}
	outer.Close() // Transport owner does this only after its handlers return.
	if limits.channels != 0 || limits.children != 0 || limits.queued != 0 {
		t.Fatalf("joined owner leaked reservations: channels=%d children=%d queued=%d", limits.channels, limits.children, limits.queued)
	}
}
