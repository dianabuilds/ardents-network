package route

import (
	"encoding/binary"
	"errors"
	"os"
	"testing"
	"time"
)

func TestClosedOuterWriterWaitsForReturnedCredit(t *testing.T) {
	lane, bridge, _, _ := closedOuterAdmissionFixture(t)
	frames := make(chan ClosedLaneFrame, 8)
	bridge.write = func(frame ClosedLaneFrame, _ func() time.Time) error { frames <- frame; return nil }
	if err := lane.SetWriteDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	// Model four ordinary TLS records consuming the initial advertised window.
	for range 4 {
		if n, err := lane.Write(make([]byte, 16<<10)); err != nil || n != 16<<10 {
			t.Fatalf("initial window: %d %v", n, err)
		}
		<-frames
	}
	done := make(chan error, 1)
	go func() {
		n, err := lane.Write([]byte{9})
		if err == nil && n != 1 {
			err = errors.New("short resumed write")
		}
		done <- err
	}()
	select {
	case err := <-done:
		t.Fatalf("credit backpressure closed a live writer: %v", err)
	case frame := <-frames:
		t.Fatalf("wrote without credit: %v", frame.Kind)
	case <-time.After(40 * time.Millisecond):
	}
	body := binary.BigEndian.AppendUint32(nil, 1)
	if _, err := bridge.Accept(ClosedLaneFrame{Kind: closedFrameCredit, Lane: 1, Body: body}); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("credit did not resume writer")
	}
	frame := <-frames
	if frame.Kind != closedFrameBytes || len(frame.Body) != 1 || frame.Body[0] != 9 {
		t.Fatal("resumed payload changed")
	}
}

func TestClosedOuterWriterCreditWaitEndsAtDeadlineOrClose(t *testing.T) {
	for _, reason := range []string{"deadline", "close"} {
		t.Run(reason, func(t *testing.T) {
			lane, bridge, _, _ := closedOuterAdmissionFixture(t)
			if err := lane.SetWriteDeadline(time.Now().Add(2 * time.Second)); err != nil {
				t.Fatal(err)
			}
			if n, err := lane.Write(make([]byte, 64<<10)); err != nil || n != 64<<10 {
				t.Fatalf("initial window: %d %v", n, err)
			}
			done := make(chan error, 1)
			go func() { _, err := lane.Write([]byte{1}); done <- err }()
			select {
			case err := <-done:
				t.Fatalf("writer did not wait for credit: %v", err)
			case <-time.After(40 * time.Millisecond):
			}
			if reason == "deadline" {
				if err := lane.SetWriteDeadline(time.Now()); err != nil {
					t.Fatal(err)
				}
			} else {
				bridge.Close()
			}
			select {
			case err := <-done:
				if err == nil {
					t.Fatal("expired writer succeeded")
				}
				if reason == "deadline" && !errors.Is(err, os.ErrDeadlineExceeded) {
					t.Fatalf("deadline classification lost: %v", err)
				}
			case <-time.After(time.Second):
				t.Fatal("credit waiter survived cancellation")
			}
		})
	}
}

func TestClosedOuterCreditWaitDoesNotBlockSiblingLane(t *testing.T) {
	lane, bridge, _, _ := closedOuterAdmissionFixture(t)
	if err := lane.SetWriteDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := lane.Write(make([]byte, 64<<10)); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { _, err := lane.Write([]byte{1}); done <- err }()
	select {
	case err := <-done:
		t.Fatalf("credit did not hold writer: %v", err)
	case <-time.After(40 * time.Millisecond):
	}
	receiver := bridge.handshake.receiver
	body, err := EncodeClosedNodeOpen(ClosedOpen{NextNodeID: receiver.NodeID, NextDutyGeneration: receiver.DutyGeneration, Purpose: ClosedPurposeForwarding, Deadline: receiver.Deadline}, ClosedChildOrdinary)
	if err != nil {
		t.Fatal(err)
	}
	sibling, err := bridge.Accept(ClosedLaneFrame{Kind: closedFrameOpen, Lane: 3, Body: body})
	if err != nil {
		t.Fatal(err)
	}
	if n, err := sibling.Write([]byte{2}); err != nil || n != 1 {
		t.Fatalf("sibling stalled behind credit waiter: %d %v", n, err)
	}
	bridge.Close()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("closed waiter succeeded")
		}
	case <-time.After(time.Second):
		t.Fatal("waiter did not join")
	}
}
