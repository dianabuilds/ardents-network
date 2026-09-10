package endpoint

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRecordingAttachmentQueueHonorsCancellationWhileObservationBlocked(t *testing.T) {
	requests := make(chan observedAttachmentRequest)
	open := recordingAttachmentQueue(requests)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	outcome := make(chan error, 1)
	go func() { _, err := open(ctx, routeRecovery{}); outcome <- err }()
	select {
	case err := <-outcome:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancelled observation: %v", err)
		}
	case <-time.After(100 * time.Millisecond):
		// Join the old blocking implementation before reporting the regression.
		select {
		case <-requests:
		case <-time.After(time.Second):
			t.Fatal("observer did not reach recording")
		}
		<-outcome
		t.Fatal("cancelled opener waited for its observation consumer")
	}
}
