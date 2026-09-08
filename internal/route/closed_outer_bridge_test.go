package route

import (
	"bytes"
	"encoding/binary"
	"io"
	"testing"
	"time"
)

func TestClosedOuterBridgeCarriesOpaqueInnerLaneWithCredit(t *testing.T) {
	now := time.Unix(1_800_300_000, 0).UTC()
	receiver := closedOuterHandshakeReceiver(now)
	limits, err := NewClosedDutyLimits(func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	handshake, err := NewClosedOuterHandshake(receiver, limits, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	defer handshake.Close()
	written := make(chan ClosedLaneFrame, 8)
	bridge, err := NewClosedOuterBridge(handshake, func(frame ClosedLaneFrame) error { written <- frame; return nil })
	if err != nil {
		t.Fatal(err)
	}
	hello := ClosedHello{NetworkID: receiver.NetworkID, StateGeneration: receiver.StateGeneration, StateDigest: receiver.StateDigest,
		ProfileDigest: receiver.ProfileDigest, RecipientNodeID: receiver.NodeID, RecipientDutyGeneration: receiver.DutyGeneration,
		Purpose: ClosedPurposeForwarding, ChannelNonce: [32]byte{19}, Deadline: receiver.Deadline}
	helloBody, err := EncodeClosedHello(hello)
	if err != nil {
		t.Fatal(err)
	}
	if lane, err := bridge.Accept(ClosedLaneFrame{Kind: closedFrameHello, Body: helloBody}); err != nil || lane != nil {
		t.Fatalf("outer HELLO = %v / %v", lane, err)
	}
	if accepted := <-written; accepted.Kind != closedFrameAccept || accepted.Lane != 0 {
		t.Fatalf("outer accept = %+v", accepted)
	}
	openBody, err := EncodeClosedOpen(ClosedOpen{NextNodeID: receiver.NodeID, NextDutyGeneration: receiver.DutyGeneration, Purpose: ClosedPurposeIssuer, Deadline: receiver.Deadline})
	if err != nil {
		t.Fatal(err)
	}
	lane, err := bridge.Accept(ClosedLaneFrame{Kind: closedFrameOpen, Lane: 1, Body: openBody})
	if err != nil || lane == nil {
		t.Fatalf("outer open = %v / %v", lane, err)
	}
	if _, err := bridge.Accept(ClosedLaneFrame{Kind: closedFrameBytes, Lane: 1, Body: []byte{1, 2, 3}}); err != nil {
		t.Fatal(err)
	}
	buffer := make([]byte, 3)
	if count, err := io.ReadFull(lane, buffer); err != nil || !bytes.Equal(buffer, []byte{1, 2, 3}) || count != 3 {
		t.Fatalf("pre-TLS bridge read = %d / %x / %v", count, buffer, err)
	}
	if err := lane.BeginInnerHello(); err != nil {
		t.Fatal(err)
	}
	inner := hello
	inner.Purpose, inner.ChannelNonce = ClosedPurposeIssuer, [32]byte{20}
	if err := lane.Activate(inner); err != nil {
		t.Fatal(err)
	}
	if _, err := bridge.Accept(ClosedLaneFrame{Kind: closedFrameBytes, Lane: 1, Body: []byte{4, 5}}); err != nil {
		t.Fatal(err)
	}
	buffer = make([]byte, 2)
	if count, err := io.ReadFull(lane, buffer); err != nil || !bytes.Equal(buffer, []byte{4, 5}) || count != 2 {
		t.Fatalf("post-TLS bridge read = %d / %x / %v", count, buffer, err)
	}
	if credit := <-written; credit.Kind != closedFrameCredit || credit.Lane != 1 || !bytes.Equal(credit.Body, []byte{0, 0, 0, 2}) {
		t.Fatalf("inner read credit = %+v", credit)
	}
	if count, err := lane.Write([]byte{6, 7}); err != nil || count != 2 {
		t.Fatalf("inner write = %d / %v", count, err)
	}
	if outbound := <-written; outbound.Kind != closedFrameBytes || outbound.Lane != 1 || !bytes.Equal(outbound.Body, []byte{6, 7}) {
		t.Fatalf("inner outbound bytes = %+v", outbound)
	}
	creditBody := make([]byte, 4)
	binary.BigEndian.PutUint32(creditBody, 2)
	if _, err := bridge.Accept(ClosedLaneFrame{Kind: closedFrameCredit, Lane: 1, Body: creditBody}); err != nil {
		t.Fatal(err)
	}
	if _, err := bridge.Accept(ClosedLaneFrame{Kind: closedFrameEOF, Lane: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := lane.Read(make([]byte, 1)); err != io.EOF {
		t.Fatalf("inner EOF = %v", err)
	}
	if err := lane.CloseWithStatus(0); err != nil {
		t.Fatal(err)
	}
	if closed := <-written; closed.Kind != closedFrameClose || closed.Lane != 1 || !bytes.Equal(closed.Body, []byte{0}) {
		t.Fatalf("local lane close = %+v", closed)
	}
	if replacement, err := bridge.Accept(ClosedLaneFrame{Kind: closedFrameOpen, Lane: 3, Body: openBody}); err != nil || replacement == nil {
		t.Fatalf("released child replacement = %v / %v", replacement, err)
	}
}

func TestClosedOuterBridgeRefusesForeignOpenBeforeCreatingLane(t *testing.T) {
	now := time.Unix(1_800_300_000, 0).UTC()
	receiver := closedOuterHandshakeReceiver(now)
	limits, _ := NewClosedDutyLimits(func() time.Time { return now })
	handshake, err := NewClosedOuterHandshake(receiver, limits, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	defer handshake.Close()
	bridge, err := NewClosedOuterBridge(handshake, func(ClosedLaneFrame) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	hello := ClosedHello{NetworkID: receiver.NetworkID, StateGeneration: receiver.StateGeneration, StateDigest: receiver.StateDigest, ProfileDigest: receiver.ProfileDigest,
		RecipientNodeID: receiver.NodeID, RecipientDutyGeneration: receiver.DutyGeneration, Purpose: ClosedPurposeForwarding, ChannelNonce: [32]byte{21}, Deadline: receiver.Deadline}
	body, err := EncodeClosedHello(hello)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bridge.Accept(ClosedLaneFrame{Kind: closedFrameHello, Body: body}); err != nil {
		t.Fatal(err)
	}
	foreign := receiver.NodeID
	foreign[0]++
	body, err = EncodeClosedOpen(ClosedOpen{NextNodeID: foreign, NextDutyGeneration: receiver.DutyGeneration, Purpose: ClosedPurposeIssuer, Deadline: receiver.Deadline})
	if err != nil {
		t.Fatal(err)
	}
	if lane, err := bridge.Accept(ClosedLaneFrame{Kind: closedFrameOpen, Lane: 1, Body: body}); err == nil || lane != nil || len(bridge.lanes) != 0 {
		t.Fatalf("foreign open created bridge work: %v / %v / %d", lane, err, len(bridge.lanes))
	}
}
