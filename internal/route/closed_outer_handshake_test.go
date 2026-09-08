package route

import (
	"bytes"
	"testing"
	"time"
)

func TestClosedOuterHandshakeOnlyAllocatesBoundedInnerTLS(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	receiver := closedOuterHandshakeReceiver(now)
	handshake, err := NewClosedOuterHandshake(receiver, func() time.Time { return now })
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
	openBody, err := EncodeClosedOpen(ClosedOpen{NextNodeID: receiver.NodeID, NextDutyGeneration: receiver.DutyGeneration,
		Purpose: ClosedPurposeIssuer, Deadline: receiver.Deadline})
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
		t.Fatal("accepted TLS handshake bytes beyond 4096")
	}
}

func TestClosedOuterHandshakeRefusesBootstrapAndSubstitutedRecipient(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	receiver := closedOuterHandshakeReceiver(now)
	handshake, err := NewClosedOuterHandshake(receiver, func() time.Time { return now })
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
	if _, err := handshake.Accept(ClosedLaneFrame{Kind: closedFrameHello, Body: helloBody}); err != nil {
		t.Fatal(err)
	}
	if _, err := handshake.Accept(ClosedLaneFrame{Kind: closedFrameBootstrap, Body: EncodeClosedBootstrap(true)}); err == nil {
		t.Fatal("outer carrier accepted bootstrap")
	}
	otherNode := receiver.NodeID
	otherNode[0]++
	openBody, err := EncodeClosedOpen(ClosedOpen{NextNodeID: otherNode, NextDutyGeneration: receiver.DutyGeneration,
		Purpose: ClosedPurposeIssuer, Deadline: receiver.Deadline})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := handshake.Accept(ClosedLaneFrame{Kind: closedFrameOpen, Lane: 1, Body: openBody}); err == nil {
		t.Fatal("outer carrier accepted substituted recipient")
	}
}

func closedOuterHandshakeReceiver(now time.Time) ClosedOuterReceiver {
	receiver := ClosedOuterReceiver{NetworkID: [32]byte{1}, StateGeneration: [32]byte{2}, StateDigest: [32]byte{3},
		ProfileDigest: [32]byte{4}, NodeID: [32]byte{5}, DutyGeneration: 6, Deadline: now.Add(10 * time.Second)}
	receiver.AllowedPurposes[ClosedPurposeIssuer] = true
	return receiver
}
