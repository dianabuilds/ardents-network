package route

import (
	"testing"
	"time"
)

func TestClosedOuterRetainedCarrierKeepsFreshChildDeadlines(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	receiver := closedOuterHandshakeReceiver(now)
	receiver.Deadline = now.Add(time.Hour)
	limits, err := NewClosedDutyLimits(func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	handshake, err := NewClosedOuterHandshake(receiver, limits, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	defer handshake.Close()
	parentEnd := now.Add(time.Minute)
	hello := ClosedHello{NetworkID: receiver.NetworkID, StateGeneration: receiver.StateGeneration, StateDigest: receiver.StateDigest,
		ProfileDigest: receiver.ProfileDigest, RecipientNodeID: receiver.NodeID, RecipientDutyGeneration: receiver.DutyGeneration,
		Purpose: ClosedPurposeForwarding, ChannelNonce: [32]byte{8}, Deadline: parentEnd}
	body, err := EncodeClosedHello(hello)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := handshake.Accept(ClosedLaneFrame{Kind: closedFrameHello, Body: body}); err != nil {
		t.Fatal(err)
	}
	open := func(id uint32, end time.Time) error {
		t.Helper()
		body, err := EncodeClosedNodeOpen(ClosedOpen{NextNodeID: receiver.NodeID, NextDutyGeneration: receiver.DutyGeneration,
			Purpose: ClosedPurposeIssuer, Deadline: end}, ClosedChildIssuerBootstrap)
		if err != nil {
			t.Fatal(err)
		}
		_, err = handshake.Accept(ClosedLaneFrame{Kind: closedFrameOpen, Lane: id, Body: body})
		return err
	}
	if err := open(1, now.Add(10*time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := handshake.Accept(ClosedLaneFrame{Kind: closedFrameClose, Lane: 1, Body: []byte{0}}); err != nil {
		t.Fatal(err)
	}
	now = now.Add(11 * time.Second)
	if err := open(3, now.Add(10*time.Second)); err != nil {
		t.Fatalf("fresh child on retained Carrier: %v", err)
	}
	if err := open(5, now.Add(11*time.Second)); err == nil {
		t.Fatal("extended ten-second child limit")
	}
	now = parentEnd.Add(-time.Second)
	if err := open(5, parentEnd.Add(time.Second)); err == nil {
		t.Fatal("extended authenticated outer HELLO deadline")
	}
	now = parentEnd
	if err := open(5, now.Add(time.Second)); err == nil {
		t.Fatal("accepted child after parent expiry")
	}
}
