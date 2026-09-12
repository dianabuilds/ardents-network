package route

import (
	"bytes"
	"testing"
	"time"
)

func TestClosedOuterHandshakeOnlyAllocatesBoundedInnerTLS(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	receiver := closedOuterHandshakeReceiver(now)
	limits, err := NewClosedDutyLimits(func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	handshake, err := NewClosedOuterHandshake(receiver, limits, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	hello := ClosedHello{NetworkID: receiver.NetworkID, StateGeneration: receiver.StateGeneration, StateDigest: receiver.StateDigest,
		ProfileDigest: receiver.ProfileDigest, RecipientNodeID: receiver.NodeID, RecipientDutyGeneration: receiver.DutyGeneration,
		Purpose: ClosedPurposeForwarding, ChannelNonce: [32]byte{8}, Deadline: receiver.Deadline}
	helloBody, err := EncodeClosedHello(hello)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := handshake.Accept(ClosedLaneFrame{Kind: closedFrameHello, Body: helloBody}); err != nil || got != nil {
		t.Fatalf("outer HELLO = %x / %v", got, err)
	}
	openBody, err := EncodeClosedNodeOpen(ClosedOpen{NextNodeID: receiver.NodeID, NextDutyGeneration: receiver.DutyGeneration,
		Purpose: ClosedPurposeIssuer, Deadline: receiver.Deadline}, ClosedChildOrdinary)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := handshake.Accept(ClosedLaneFrame{Kind: closedFrameOpen, Lane: 1, Body: openBody}); err != nil || got != nil {
		t.Fatalf("outer OPEN = %x / %v", got, err)
	}
	innerTLS := bytes.Repeat([]byte{9}, closedOuterHandshakeBytes)
	if got, err := handshake.Accept(ClosedLaneFrame{Kind: closedFrameBytes, Lane: 1, Body: innerTLS}); err != nil || !bytes.Equal(got, innerTLS) {
		t.Fatalf("outer TLS bytes = %x / %v", got, err)
	}
	if _, err := handshake.Accept(ClosedLaneFrame{Kind: closedFrameBytes, Lane: 1, Body: []byte{1}}); err == nil {
		t.Fatal("accepted TLS handshake bytes beyond 4096 before inner HELLO")
	}
	if err := handshake.BeginInnerHello(1); err != nil {
		t.Fatalf("begin inner HELLO = %v", err)
	}
	inner := ClosedHello{NetworkID: receiver.NetworkID, StateGeneration: receiver.StateGeneration, StateDigest: receiver.StateDigest,
		ProfileDigest: receiver.ProfileDigest, RecipientNodeID: receiver.NodeID, RecipientDutyGeneration: receiver.DutyGeneration,
		Purpose: ClosedPurposeIssuer, ChannelNonce: [32]byte{9}, Deadline: receiver.Deadline}
	if err := handshake.VerifyInnerHello(1, inner); err != nil {
		t.Fatalf("matching inner HELLO = %v", err)
	}
	inner.Purpose = ClosedPurposeDataJoin
	if err := handshake.VerifyInnerHello(1, inner); err == nil {
		t.Fatal("outer child accepted an inner HELLO for a different purpose")
	}
	operation := bytes.Repeat([]byte{7}, 16<<10)
	if got, err := handshake.Accept(ClosedLaneFrame{Kind: closedFrameBytes, Lane: 1, Body: operation}); err != nil || !bytes.Equal(got, operation) {
		t.Fatalf("post-TLS inner bytes = %x / %v", got, err)
	}
	credit, err := handshake.ConsumeInnerBytes(1, uint32(len(operation)))
	if err != nil || credit.Kind != closedFrameCredit || credit.Lane != 1 || !bytes.Equal(credit.Body, []byte{0, 0, 64, 0}) {
		t.Fatalf("post-TLS credit = %+v / %v", credit, err)
	}
	if _, err := handshake.Accept(ClosedLaneFrame{Kind: closedFrameEOF, Lane: 1}); err != nil {
		t.Fatalf("post-TLS EOF = %v", err)
	}
	if _, err := handshake.Accept(ClosedLaneFrame{Kind: closedFrameBytes, Lane: 1, Body: []byte{7}}); err == nil {
		t.Fatal("accepted bytes after lane EOF")
	}
}

func closedOuterHandshakeReceiver(now time.Time) ClosedOuterReceiver {
	return ClosedOuterReceiver{NetworkID: [32]byte{1}, StateGeneration: [32]byte{2}, StateDigest: [32]byte{3},
		ProfileDigest: [32]byte{4}, NodeID: [32]byte{5}, RecordDigest: [32]byte{6}, DutyGeneration: 6,
		RoleDomain: closedRoleDomainRendezvous, Subrole: closedDutyIssuance, Deadline: now.Add(10 * time.Second)}
}
