package route

import (
	"testing"
	"time"
)

func TestClosedForwardingByteAllowanceIncludesAdmissionHeadersAndControl(t *testing.T) {
	_, _, lease, now := closedOuterAdmissionFixture(t)
	channel, err := NewClosedForwardingChannel(lease, func(ClosedOpen) error { return nil }, func() time.Time { return *now })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(channel.Cancel)
	body, err := EncodeClosedOpen(ClosedOpen{NextNodeID: [32]byte{1}, NextDutyGeneration: 1, Purpose: ClosedPurposeForwarding, Deadline: now.Add(time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := channel.Accept(ClosedLaneFrame{Kind: closedFrameOpen, Lane: 1, Body: body}); err != nil {
		t.Fatal(err)
	}
	if _, ok := channel.Next(); !ok {
		t.Fatal("missing OPEN")
	}
	accepted, err := ClosedAcceptFrame(0, 64<<10)
	if err != nil {
		t.Fatal(err)
	}
	if err := channel.AccountOutput(accepted); err != nil {
		t.Fatal(err)
	}
	oneByte := ClosedLaneFrame{Kind: closedFrameBytes, Lane: 1, Body: []byte{1}}
	if err := channel.QueueReverse(oneByte); err != nil {
		t.Fatal(err)
	}
	if err := channel.AccountOutput(oneByte); err != nil {
		t.Fatal(err)
	}
	channel.ReleaseReverse(oneByte)
	// Actual admitted channel frames: HELLO 225 + ADMIT 371 + OPEN 65 +
	// ACCEPT 21 + BYTES 17 = 699. Keep precisely one 20-byte CREDIT.
	// This isolates accounting; it does not claim a network workload.
	remaining := (32 << 20) - 699
	data := make([]byte, 16<<10)
	for remaining > 20 {
		size := min(len(data), remaining-20-16)
		if size <= 0 {
			t.Fatal("invalid predeclared accounting schedule")
		}
		if err := channel.AccountOutput(ClosedLaneFrame{Kind: closedFrameBytes, Lane: 1, Body: data[:size]}); err != nil {
			t.Fatalf("refused output before its complete-byte allowance: %v", err)
		}
		remaining -= 16 + size
	}
	credit := ClosedLaneFrame{Kind: closedFrameCredit, Lane: 1, Body: []byte{0, 0, 0, 1}}
	if _, err := channel.Accept(credit); err != nil {
		t.Fatal("final receiver-accounted CREDIT was refused")
	}
	if err := channel.AccountOutput(ClosedLaneFrame{Kind: closedFrameEOF, Lane: 1}); err == nil {
		t.Fatal("empty control bypassed exhausted complete-byte allowance")
	}
	if _, err := channel.Accept(ClosedLaneFrame{Kind: closedFrameEOF, Lane: 1}); err == nil {
		t.Fatal("incoming control bypassed exhausted complete-byte allowance")
	}
}

func TestClosedForwardingSemanticRefusalDoesNotRefundReceivedFrame(t *testing.T) {
	_, _, lease, now := closedOuterAdmissionFixture(t)
	channel, err := NewClosedForwardingChannel(lease, func(ClosedOpen) error { return nil }, func() time.Time { return *now })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(channel.Cancel)
	// This well-framed CREDIT names no live child. Its receipt still consumes
	// the remaining allowance even though no child effect may be admitted.
	channel.usedBytes = channel.byteLimit - 20
	if _, err := channel.Accept(ClosedLaneFrame{Kind: closedFrameCredit, Lane: 1, Body: []byte{0, 0, 0, 1}}); err == nil {
		t.Fatal("unallocated child CREDIT accepted")
	}
	accepted, err := ClosedAcceptFrame(0, 64<<10)
	if err != nil {
		t.Fatal(err)
	}
	// Use a 16-byte otherwise permitted output to distinguish the consumed
	// 20-byte input from a merely rejected semantic operation.
	if err := channel.AccountOutput(ClosedLaneFrame{Kind: closedFrameEOF}); err == nil {
		t.Fatal("refused input restored allowance for output")
	}
	if err := channel.AccountOutput(accepted); err == nil {
		t.Fatal("exhausted channel accepted another response")
	}
}
