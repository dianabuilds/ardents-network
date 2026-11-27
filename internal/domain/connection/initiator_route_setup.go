package connection

import (
	"context"
	"errors"
	"time"
)

// attachmentSetupDeadline selects the original absolute setup bound once.
// Rounding cannot extend a caller's earlier deadline (ADR-0121).
func attachmentSetupDeadline(ctx context.Context, now time.Time, bounds ...time.Time) time.Time {
	end := now.Add(30 * time.Second)
	if callerEnd, ok := ctx.Deadline(); ok && callerEnd.Before(end) {
		end = callerEnd
	}
	for _, bound := range bounds {
		if bound.Before(end) {
			end = bound
		}
	}
	return end.UTC().Truncate(time.Second)
}

// runInitiatorSetupFlights joins the two independent setup exchanges. The
// callers above alone construct genuine Submission/JOIN operations and retain
// their authority checks and durable presentations before operation bytes.
// JOIN keeps its original physical caller: canceling this setup must interrupt
// the exact acquisition, not shorten a successfully transferred stream.
func runInitiatorSetupFlights(setup context.Context, interrupt func() error, submit func(context.Context) error, openJoin func() error) error {
	ctx, cancel := context.WithCancel(setup)
	defer cancel()
	interrupted := make(chan struct{})
	var interruption error
	stop := context.AfterFunc(ctx, func() {
		defer close(interrupted)
		interruption = interrupt()
	})
	submitted := make(chan error, 1)
	go func() {
		err := submit(ctx)
		if err != nil {
			cancel()
		}
		submitted <- err
	}()
	joined := openJoin()
	if joined != nil {
		cancel()
	}
	submission := <-submitted
	// Disarm and join before canceling our private setup context on return.
	// Any already-started physical retirement and its error remain retained.
	if !stop() {
		<-interrupted
	}
	return errors.Join(submission, joined, interruption, ctx.Err())
}
