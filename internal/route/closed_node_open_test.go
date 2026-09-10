package route

import (
	"bytes"
	"testing"
	"time"
)

func TestClosedNodeOpenRequiresAuthenticatedExactGrammar(t *testing.T) {
	now := time.Unix(1_800_300_000, 0).UTC()
	receiver := closedOuterHandshakeReceiver(now)
	open := ClosedOpen{NextNodeID: receiver.NodeID, NextDutyGeneration: receiver.DutyGeneration, Purpose: ClosedPurposeIssuer, Deadline: receiver.Deadline}
	old, err := EncodeClosedOpen(open)
	if err != nil {
		t.Fatal(err)
	}
	for _, restriction := range []ClosedChildRestriction{ClosedChildOrdinary, ClosedChildIssuerBootstrap} {
		raw, err := EncodeClosedNodeOpen(open, restriction)
		if err != nil || len(raw) != 50 || !bytes.Equal(raw[:49], old) || raw[49] != byte(restriction) {
			t.Fatalf("canonical NodeOPEN %x %v", raw, err)
		}
		decoded, bound, err := DecodeClosedNodeOpen(raw)
		if err != nil || decoded != open || bound != restriction {
			t.Fatalf("decode %v %v %v", decoded, bound, err)
		}
		if _, err := DecodeClosedOpen(raw); err == nil {
			t.Fatal("Endpoint role accepted injected restriction")
		}
		var wire bytes.Buffer
		if err := WriteClosedLaneFrame(&wire, ClosedLaneFrame{Kind: 4, Lane: 1, Body: raw}); err != nil {
			t.Fatal(err)
		}
		frame, err := ReadClosedLaneFrame(&wire)
		if err != nil || !bytes.Equal(frame.Body, raw) {
			t.Fatalf("framed NodeOPEN: %v", err)
		}
	}
	for _, raw := range [][]byte{old, append(append([]byte(nil), old...), 2), append(append([]byte(nil), old...), 0, 0)} {
		if _, _, err := DecodeClosedNodeOpen(raw); err == nil {
			t.Fatalf("accepted missing/unknown/trailing restriction %x", raw)
		}
	}
	limits, err := NewClosedDutyLimits(func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	outer, err := NewClosedOuterHandshake(receiver, limits, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	defer outer.Close()
	bridge, err := NewClosedOuterBridge(outer, func(uint32, time.Time) error { return nil }, func(ClosedLaneFrame, func() time.Time) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	hello := ClosedHello{NetworkID: receiver.NetworkID, StateGeneration: receiver.StateGeneration, StateDigest: receiver.StateDigest, ProfileDigest: receiver.ProfileDigest,
		RecipientNodeID: receiver.NodeID, RecipientDutyGeneration: receiver.DutyGeneration, Purpose: ClosedPurposeForwarding, ChannelNonce: [32]byte{20}, Deadline: receiver.Deadline}
	body, err := EncodeClosedHello(hello)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bridge.Accept(ClosedLaneFrame{Kind: 1, Body: body}); err != nil {
		t.Fatal(err)
	}
	for _, raw := range [][]byte{old, append(append([]byte(nil), old...), 2)} {
		if _, err := bridge.Accept(ClosedLaneFrame{Kind: 4, Lane: 1, Body: raw}); err == nil {
			t.Fatal("outer accepted incompatible allocation")
		}
		if limits.children != 0 || len(bridge.lanes) != 0 {
			t.Fatal("incompatible OPEN allocated before rejection")
		}
	}
	for index, restriction := range []ClosedChildRestriction{ClosedChildIssuerBootstrap, ClosedChildOrdinary} {
		raw, err := EncodeClosedNodeOpen(open, restriction)
		if err != nil {
			t.Fatal(err)
		}
		lane, err := bridge.Accept(ClosedLaneFrame{Kind: 4, Lane: uint32(index*2 + 1), Body: raw})
		if err != nil || lane.Restriction() != restriction {
			t.Fatalf("mixed child restriction %v %v", lane, err)
		}
		raw[49] ^= 1
		if lane.Restriction() != restriction {
			t.Fatal("caller rewrote retained child restriction")
		}
		if _, err := bridge.Accept(ClosedLaneFrame{Kind: 4, Lane: uint32(index*2 + 1), Body: raw}); err == nil {
			t.Fatal("same child was relabelled")
		}
	}
	if limits.children != 2 {
		t.Fatalf("mixed pool children=%d", limits.children)
	}
	if _, err := bridge.Accept(ClosedLaneFrame{Kind: 9, Lane: 1, Body: []byte{0}}); err != nil {
		t.Fatal(err)
	}
	raw, err := EncodeClosedNodeOpen(open, ClosedChildOrdinary)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bridge.Accept(ClosedLaneFrame{Kind: 4, Lane: 1, Body: raw}); err == nil {
		t.Fatal("closed restricted ID reused as ordinary child")
	}
	if limits.children != 1 {
		t.Fatal("reused ID changed child reservation")
	}
}
