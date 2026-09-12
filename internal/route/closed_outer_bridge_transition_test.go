package route

import (
	"io"
	"sync"
	"testing"
	"time"
)

// TLS completion races the outer reader delivering the first encrypted HELLO.
// Either accounting phase may receive it, but consumption must never mint
// credit for bytes that were charged only to the handshake allowance.
func TestClosedOuterBridgeSerializesTLSCompletionWithIncomingBytes(t *testing.T) {
	now := time.Unix(1_800_300_000, 0).UTC()
	receiver := closedOuterHandshakeReceiver(now)
	for attempt := 0; attempt < 1000; attempt++ {
		limits, err := NewClosedDutyLimits(func() time.Time { return now })
		if err != nil {
			t.Fatal(err)
		}
		handshake, err := NewClosedOuterHandshake(receiver, limits, func() time.Time { return now })
		if err != nil {
			t.Fatal(err)
		}
		bridge, err := NewClosedOuterBridge(handshake, func(uint32, time.Time) error { return nil }, func(ClosedLaneFrame, func() time.Time) error { return nil })
		if err != nil {
			t.Fatal(err)
		}
		hello := ClosedHello{NetworkID: receiver.NetworkID, StateGeneration: receiver.StateGeneration, StateDigest: receiver.StateDigest,
			ProfileDigest: receiver.ProfileDigest, RecipientNodeID: receiver.NodeID, RecipientDutyGeneration: receiver.DutyGeneration,
			Purpose: ClosedPurposeForwarding, ChannelNonce: [32]byte{21}, Deadline: receiver.Deadline}
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
		var received [1]byte
		if _, err := io.ReadFull(lane, received[:]); err != nil {
			t.Fatal(err)
		}
		start := make(chan struct{})
		var workers sync.WaitGroup
		var beginErr, acceptErr error
		workers.Go(func() { <-start; beginErr = lane.BeginInnerHello() })
		workers.Go(func() {
			<-start
			_, acceptErr = bridge.Accept(ClosedLaneFrame{Kind: closedFrameBytes, Lane: 1, Body: []byte{2}})
		})
		close(start)
		workers.Wait()
		if beginErr != nil || acceptErr != nil {
			t.Fatalf("transition %d: begin=%v accept=%v", attempt, beginErr, acceptErr)
		}
		if _, err := io.ReadFull(lane, received[:]); err != nil || received[0] != 2 {
			t.Fatalf("transition %d: read=%v byte=%d", attempt, err, received[0])
		}
		handshake.Close()
	}
}
