package forwarding

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestForwardingRetirementFrameIsDistinctExactOriginalChallenge(t *testing.T) {
	var start [160]byte
	for i := range start {
		start[i] = byte(i)
	}
	challenge := [32]byte{7}
	frame := retirementFrame(start, challenge)
	if string(frame[:len(retirementPrefix)]) != "ardents-forwarding-domain-joined-v1\n" || !bytes.Equal(frame[len(retirementPrefix):len(retirementPrefix)+160], start[:]) || !bytes.Equal(frame[len(frame)-32:], challenge[:]) {
		t.Fatal("independent retirement frame layout differs")
	}
	other := challenge
	other[31] = 1
	if retirementFrame(start, other) == frame {
		t.Fatal("foreign challenge reused an old completion")
	}
	start[0] ^= 1
	if retirementFrame(start, challenge) == frame {
		t.Fatal("foreign original start reused old completion")
	}
}

func TestForwardingRetirementCancellationCannotErasePhysicalWrappers(t *testing.T) {
	for _, err := range []error{nil, context.Canceled, errors.Join(context.Canceled, errors.Join(context.Canceled, context.Canceled))} {
		if !joinedCancellation(err) {
			t.Fatal("exact original cancellation grammar refused", err)
		}
	}
	for _, err := range []error{context.DeadlineExceeded, errors.New("late native Close failed"), fmt.Errorf("physical Close: %w", context.Canceled), errors.Join(context.Canceled, fmt.Errorf("physical Close: %w", context.Canceled)), errors.Join(context.Canceled, errors.New("output failed"))} {
		if joinedCancellation(err) {
			t.Fatal("independent physical/late failure classified as ordinary cancellation", err)
		}
	}
}

func TestForwardingRetirementCannotReportBeforeOriginalStartupClose(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	for _, h := range []*Startup{nil, {}, {original: &startup{closeDone: make(chan struct{})}}} {
		if reported, err := h.ReportRetirement(ctx, nil); reported || err == nil {
			t.Fatal("empty or unjoined startup reported completion", err)
		}
	}
}
