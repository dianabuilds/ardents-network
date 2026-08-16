package bridge

import (
	"context"
	"errors"
	"net"
	"time"
)

// Acquire consumes one authenticated transition and owns the complete durable
// contact sequence. The caller supplies only the candidate-specific opener.
func (owner *owner) Acquire(ctx context.Context, frame []byte, manifest [32]byte,
	parentDeadline time.Time,
	openContact func(context.Context, [32]byte, []byte, time.Time) (net.Conn, func() error, bool, error),
) (net.Conn, func() error, error) {
	identity, envelope, ordinal, baseOffset, deadline, err := owner.BeginContact(frame, manifest, parentDeadline)
	if err != nil {
		return nil, nil, err
	}
	baseTime := owner.config.Clock()
	for {
		contactCtx, cancel, contactDeadline, contextErr := contactContext(ctx, deadline)
		if contextErr != nil {
			finishErr := owner.finishAt(ordinal, baseOffset, baseTime, false, true)
			_, _, _, terminalErr := owner.NextContact(ctx)
			return nil, nil, errors.Join(contextErr, finishErr, terminalErr)
		}
		channel, cleanupContact, clean, openErr := openContact(contactCtx, identity, envelope, contactDeadline)
		if openErr == nil && (channel == nil || cleanupContact == nil) {
			openErr = errors.New("bridge opener returned an incomplete result")
		}
		if openErr != nil {
			cancel()
			finishErr := owner.finishAt(ordinal, baseOffset, baseTime, false, clean)
			if !clean || finishErr != nil {
				return nil, nil, errors.Join(errors.New("bridge-local-denial"), openErr, finishErr)
			}
			identity, envelope, ordinal, err = owner.NextContact(ctx)
			if err != nil {
				return nil, nil, err
			}
			continue
		}
		cleanup := func() error {
			cleanupErr := cleanupContact()
			cancel()
			finishErr := owner.finishAt(ordinal, baseOffset, baseTime, true, cleanupErr == nil)
			if cleanupErr != nil {
				return errors.Join(errors.New("bridge-local-denial"), cleanupErr, finishErr)
			}
			return finishErr
		}
		return channel, cleanup, nil
	}
}

func contactContext(parent context.Context, attemptDeadline time.Time) (context.Context, context.CancelFunc, time.Time, error) {
	deadline := time.Now().Add(15 * time.Second)
	if attemptDeadline.Before(deadline) {
		deadline = attemptDeadline
	}
	if parentDeadline, ok := parent.Deadline(); ok && parentDeadline.Before(deadline) {
		deadline = parentDeadline
	}
	if !time.Now().Before(deadline) {
		return nil, nil, time.Time{}, errors.New("bridge-deadline-exceeded")
	}
	ctx, cancel := context.WithDeadline(parent, deadline)
	return ctx, cancel, deadline, nil
}

func (owner *owner) finishAt(ordinal byte, baseOffset uint64, baseTime time.Time, opened, cleanup bool) error {
	elapsed := owner.config.Clock().Sub(baseTime)
	if elapsed < 0 {
		elapsed = 0
	}
	return owner.FinishContact(ordinal, baseOffset+uint64(elapsed), opened, cleanup)
}
