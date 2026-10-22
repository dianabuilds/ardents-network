package infrastructure

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"
)

func TestSourceRetirementFrameCannotReplayStartupACKOrAnotherChallenge(t *testing.T) {
	var start [160]byte
	copy(start[:], bytes.Repeat([]byte{'a'}, len(start)))
	challenge := [32]byte{1, 2, 3}
	actual := sourceRetirementFrame(start, challenge)
	wanted := append([]byte("ardents-source-domain-joined-v1\n"), start[:]...)
	wanted = append(wanted, challenge[:]...)
	if !bytes.Equal(actual[:], wanted) || bytes.Equal(actual[:], start[:]) {
		t.Fatal("independent retirement frame differs or admits old ACK")
	}
	if actual == sourceRetirementFrame(start, [32]byte{4}) {
		t.Fatal("foreign stop challenge replayed")
	}
	start[0] = 'b'
	if actual == sourceRetirementFrame(start, challenge) {
		t.Fatal("foreign invocation frame replayed")
	}
}

func TestSourceRetirementReportRefusesBeforeOriginalCloseJoins(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	// Failure-only local control. No accepting Startup, State or native peer is
	// constructed; a held original Close must refuse before any native I/O.
	o := &sourceStartup{ready: true, closed: true, closeDone: make(chan struct{})}
	s := &SourceStartup{original: o}
	copy := *s
	if reported, err := copy.ReportRetirement(ctx, context.Canceled); reported || !errors.Is(err, ErrBinding) {
		t.Fatal("report escaped held original Close", reported, err)
	}
	if o.reportAttempted {
		t.Fatal("held Close consumed completion report")
	}
	close(o.closeDone)
	o.ready = false
	if reported, err := s.ReportRetirement(ctx, nil); reported || !errors.Is(err, ErrBinding) {
		t.Fatal("unadmitted Startup supplied completion", reported, err)
	}
}

func TestSourceJoinedCancellationRetainsEveryIndependentFailure(t *testing.T) {
	physical := errors.New("original input close failed")
	for _, err := range []error{nil, context.Canceled, errors.Join(context.Canceled, context.Canceled)} {
		if !sourceJoinedCancellation(err) {
			t.Fatal("ordinary joined cancellation refused", err)
		}
	}
	for _, err := range []error{physical, errors.Join(context.Canceled, physical), context.DeadlineExceeded, errors.Join(context.Canceled, context.DeadlineExceeded)} {
		if sourceJoinedCancellation(err) {
			t.Fatal("physical/timeout failure erased", err)
		}
	}
}
