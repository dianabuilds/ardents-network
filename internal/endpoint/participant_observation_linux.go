//go:build linux

package endpoint

import (
	"context"
	"sync"
	"time"
)

// participantObservation serializes the local event output and retains the
// first background delivery failure until the participant can stop and report it.
type participantObservation struct {
	observe func(context.Context, ClosedParticipantEvent) error
	now     func() time.Time
	mu      sync.Mutex
	failed  chan error
}

func newParticipantObservation(observe func(context.Context, ClosedParticipantEvent) error, now func() time.Time) *participantObservation {
	return &participantObservation{observe: observe, now: now, failed: make(chan error, 1)}
}

func (output *participantObservation) emit(ctx context.Context, event ClosedParticipantEvent) error {
	if event.At.IsZero() {
		event.At = output.now().UTC()
	}
	output.mu.Lock()
	defer output.mu.Unlock()
	return output.observe(ctx, event)
}

func (output *participantObservation) background(event ClosedParticipantEvent) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := output.emit(ctx, event); err != nil {
		select {
		case output.failed <- err:
		default:
		}
	}
}

func (output *participantObservation) pendingFailure() error {
	select {
	case err := <-output.failed:
		return err
	default:
		return nil
	}
}

func (output *participantObservation) wait(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return nil
	case err := <-output.failed:
		return err
	}
}
