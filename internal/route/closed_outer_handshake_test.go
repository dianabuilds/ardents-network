//go:build linux

package route

import (
	"testing"
	"time"
)

func TestClosedOuterHandshakeRefusesBootstrapAndSubstitutedRecipient(t *testing.T) {
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
	if _, err := handshake.Accept(ClosedLaneFrame{Kind: closedFrameHello, Body: helloBody}); err != nil {
		t.Fatal(err)
	}
	if _, err := handshake.Accept(ClosedLaneFrame{Kind: closedFrameBootstrap, Body: EncodeClosedBootstrap(true)}); err == nil {
		t.Fatal("outer carrier accepted bootstrap")
	}
	otherNode := receiver.NodeID
	otherNode[0]++
	openBody, err := EncodeClosedNodeOpen(ClosedOpen{NextNodeID: otherNode, NextDutyGeneration: receiver.DutyGeneration,
		Purpose: ClosedPurposeIssuer, Deadline: receiver.Deadline}, ClosedChildOrdinary)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := handshake.Accept(ClosedLaneFrame{Kind: closedFrameOpen, Lane: 1, Body: openBody}); err == nil {
		t.Fatal("outer carrier accepted substituted recipient")
	}
	incompatible, err := EncodeClosedNodeOpen(ClosedOpen{NextNodeID: receiver.NodeID, NextDutyGeneration: receiver.DutyGeneration,
		Purpose: ClosedPurposeDataJoin, Deadline: receiver.Deadline}, ClosedChildOrdinary)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := handshake.Accept(ClosedLaneFrame{Kind: closedFrameOpen, Lane: 3, Body: incompatible}); err == nil {
		t.Fatal("outer carrier accepted an incompatible issuer assignment")
	}
}
