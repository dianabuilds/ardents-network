package route

import (
	"bytes"
	"testing"
	"time"
)

func TestClosedForwardingChannelBoundsAuthorizedOddChild(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	lease := ClosedAdmission{Class: 2, Bytes: 32 << 20, Deadline: now.Add(time.Minute)}
	allowed := ClosedOpen{NextNodeID: [32]byte{1}, NextDutyGeneration: 2, Purpose: ClosedPurposeForwarding, Deadline: now.Add(30 * time.Second)}
	channel, err := NewClosedForwardingChannel(lease, func(open ClosedOpen) error {
		if open != allowed {
			return errUnexpectedForwardOpen
		}
		return nil
	}, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	body, err := EncodeClosedOpen(allowed)
	if err != nil {
		t.Fatal(err)
	}
	if event, err := channel.Accept(ClosedLaneFrame{Kind: closedFrameOpen, Lane: 1, Body: body}); err != nil || event.Open != allowed {
		t.Fatalf("open = %+v / %v", event, err)
	}
	bytesFrame := ClosedLaneFrame{Kind: closedFrameBytes, Lane: 1, Body: bytes.Repeat([]byte{3}, int(closedLaneCredit))}
	if _, err := channel.Accept(bytesFrame); err != nil {
		t.Fatal(err)
	}
	if _, err := channel.Accept(ClosedLaneFrame{Kind: closedFrameBytes, Lane: 1, Body: []byte{1}}); err == nil {
		t.Fatal("accepted bytes beyond fixed credit")
	}
	credit, err := channel.Credit(1, 1)
	if err != nil || credit.Kind != closedFrameCredit {
		t.Fatalf("credit = %+v / %v", credit, err)
	}
	if _, err := channel.Accept(ClosedLaneFrame{Kind: closedFrameBytes, Lane: 1, Body: []byte{1}}); err != nil {
		t.Fatal(err)
	}
	if _, err := channel.Accept(ClosedLaneFrame{Kind: closedFrameOpen, Lane: 1, Body: body}); err == nil {
		t.Fatal("reused child lane")
	}
	if _, err := channel.Accept(ClosedLaneFrame{Kind: closedFrameOpen, Lane: 2, Body: body}); err == nil {
		t.Fatal("accepted endpoint even lane")
	}
	if _, err := channel.Accept(ClosedLaneFrame{Kind: closedFrameEOF, Lane: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := channel.Accept(ClosedLaneFrame{Kind: closedFrameBytes, Lane: 1, Body: []byte{1}}); err == nil {
		t.Fatal("accepted bytes after EOF")
	}
}

var errUnexpectedForwardOpen = &unexpectedForwardOpen{}

type unexpectedForwardOpen struct{}

func (*unexpectedForwardOpen) Error() string { return "unexpected forwarding OPEN" }
