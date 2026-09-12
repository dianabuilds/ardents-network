//go:build linux

package route

import (
	"errors"
	"io"
	"os"
	"testing"
)

func TestClosedIntroductionRegistrationEndReasonReportsOnlyFixedCategories(t *testing.T) {
	localCancel := make(chan struct{})
	close(localCancel)
	for _, test := range []struct {
		name    string
		outcome error
		stop    chan struct{}
		want    ClosedIntroductionEndReason
	}{
		{name: "nil", want: ClosedIntroductionEndUnknown},
		{name: "local cancellation", stop: localCancel, want: ClosedIntroductionEndLocalCancel},
		{name: "peer EOF", outcome: io.EOF, want: ClosedIntroductionEndPeerEOF},
		{name: "deadline", outcome: os.ErrDeadlineExceeded, want: ClosedIntroductionEndDeadline},
		{name: "source cleanup", outcome: errors.Join(ErrClosedSourceCleanup, errors.New("private detail")), want: ClosedIntroductionEndSourceCleanup},
		{name: "other", outcome: errors.New("private detail"), want: ClosedIntroductionEndProtocol},
	} {
		t.Run(test.name, func(t *testing.T) {
			registration := &ClosedIntroductionRegistration{outcome: test.outcome, interrupted: make(chan struct{})}
			if test.stop != nil {
				registration.interrupted = test.stop
			}
			if got := registration.EndReason(); got != test.want {
				t.Fatalf("EndReason() = %q, want %q", got, test.want)
			}
		})
	}
}
