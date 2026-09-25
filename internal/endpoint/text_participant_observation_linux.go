//go:build linux

package endpoint

import (
	"context"
	"sync"
	"time"
)

// textParticipantObservation serializes the local event output and retains the
// first background delivery failure until the participant can stop and report it.
type textParticipantObservation struct {
	observe func(context.Context, TextParticipantEvent) error
	now     func() time.Time
	mu      sync.Mutex
	failed  chan error
}

func newTextParticipantObservation(observe func(context.Context, TextParticipantEvent) error, now func() time.Time) *textParticipantObservation {
	return &textParticipantObservation{observe: observe, now: now, failed: make(chan error, 1)}
}

func (output *textParticipantObservation) emit(ctx context.Context, event TextParticipantEvent) error {
	if event.At.IsZero() {
		event.At = output.now().UTC()
	}
	output.mu.Lock()
	defer output.mu.Unlock()
	return output.observe(ctx, event)
}

func (output *textParticipantObservation) background(event TextParticipantEvent) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := output.emit(ctx, event); err != nil {
		select {
		case output.failed <- err:
		default:
		}
	}
}

func (output *textParticipantObservation) pendingFailure() error {
	select {
	case err := <-output.failed:
		return err
	default:
		return nil
	}
}

func (output *textParticipantObservation) wait(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return nil
	case err := <-output.failed:
		return err
	}
}
