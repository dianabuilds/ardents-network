package connection

import (
	"context"
	"errors"
	"io"
	"time"

	application "github.com/dianabuilds/ardents-network/internal/application/connection"
)

func (stream *Stream) Done() <-chan application.Outcome {
	if stream == nil || stream.self != stream {
		return nil
	}
	return stream.outcome
}

// Context retains the original logical cancellation and immutable safety bound.
// It exposes no cancel function or physical/protocol authority.
func (stream *Stream) Context() context.Context {
	if stream == nil || stream.self != stream {
		return nil
	}
	return stream.ctx
}

func (stream *Stream) failLocked(err error) {
	if err != nil && stream.failure == nil {
		stream.failure = err
	}
	stream.cancel()
	stream.signalLocked()
}

func (stream *Stream) settleLocked() {
	if stream.settled || !stream.transfer.DeliveryComplete() {
		return
	}
	if err := stream.checkLocked(); err != nil {
		stream.failLocked(err)
		return
	}
	stream.settled = true
	stream.outcome <- application.Outcome{Class: application.CleanClose}
	close(stream.outcome)
	close(stream.settledDone)
	// Original authority/Instance remain retained during the control tail.
	// Explicit Close or the original lifetime interrupts and joins it.
	stream.signalLocked()
}

func (stream *Stream) watchLifetime() {
	defer close(stream.watcherDone)
	select {
	case <-stream.ctx.Done():
	case <-stream.settledDone:
		remaining := time.Until(time.Unix(stream.owner.identity.NoNewRecoveryAfter, 0))
		if remaining > 0 {
			timer := time.NewTimer(remaining)
			select {
			case <-stream.ctx.Done():
			case <-timer.C:
			}
			timer.Stop()
		}
		stream.sealRetirement()
	}
	stream.retire()
}

// sealRetirement denies late record effects before deliberate interruption.
// Cancellation alone leaves a window in which an interrupted record actor can
// replace an already settled outcome with context.Canceled. Physical failures
// remain with the exact joined Attachment/Route cleanup owners.
func (stream *Stream) sealRetirement() {
	stream.mu.Lock()
	defer stream.mu.Unlock()
	stream.closing = true
	if stream.failure == nil && !stream.settled {
		stream.failure = context.Cause(stream.ctx)
		if stream.failure == nil {
			stream.failure = context.Canceled
		}
	}
	stream.cancel()
	stream.signalLocked()
}

func (stream *Stream) retire() {
	stream.stop.Do(func() {
		stream.mu.Lock()
		if stream.failure == nil && !stream.settled {
			stream.failure = context.Cause(stream.ctx)
		}
		stream.closing = true
		if stream.recovery != nil {
			stream.recovery.stop()
		}
		proposal := stream.publisherProposal
		initiatorProposal := stream.initiatorProposal
		stream.cancel()
		stream.signalLocked()
		stream.mu.Unlock()
		if proposal != nil {
			_ = proposal.Close()
		}
		if initiatorProposal != nil {
			_ = initiatorProposal.close()
		}
		_ = stream.owner.Close()
		physical := stream.owner.cleanupErr
		if stream.records.physical.initiator != nil && stream.records.physical.initiator != stream.owner.initiator {
			_ = stream.records.physical.initiator.Close()
			physical = errors.Join(physical, stream.records.physical.initiator.cleanupErr)
		}
		stream.prepareUsers.Wait()
		stream.recoveryUsers.Wait()
		// The original records retain their raw work results after this join.
		// Intentional interruption may return a socket error as well as caller
		// cancellation; it is not a second logical or physical cleanup failure.
		// Physical failures are retained by the exact lower cleanup owners.
		_ = stream.records.join()
		stream.mu.Lock()
		stream.transfer.RetireObservation()
		stream.cleanupErr = errors.Join(physical, stream.recoveryCleanup)
		stream.closeErr = errors.Join(stream.failure, stream.cleanupErr)
		clear(stream.key[:])
		if !stream.settled {
			stream.outcome <- application.Outcome{Class: application.IndeterminateFailure, Reason: "Service Connection did not complete authenticated Terminal exchange"}
			close(stream.outcome)
		}
		stream.mu.Unlock()
		close(stream.joined)
	})
}

func (stream *Stream) Close() error {
	if stream == nil {
		return nil
	}
	if stream.self != stream {
		return errIntegrity
	}
	stream.sealRetirement()
	stream.retire()
	<-stream.joined
	<-stream.watcherDone
	return stream.closeErr
}

// Joined observes original physical completion, distinct from Application Done.
func (stream *Stream) Joined() <-chan struct{} {
	if stream == nil || stream.self != stream {
		return nil
	}
	return stream.joined
}

// JoinedCleanup returns the retained physical cleanup result only after this
// original stream has joined. A logical refusal is not a cleanup failure.
func (stream *Stream) JoinedCleanup() error {
	if stream == nil || stream.self != stream {
		return errIntegrity
	}
	select {
	case <-stream.joined:
		return stream.cleanupErr
	default:
		return errors.New("service Connection physical cleanup has not joined")
	}
}

// Stop synchronously seals local effects without returning physical borrows.
// Close still joins the original actors, callbacks and retained resources.
func (stream *Stream) Stop() {
	if stream == nil || stream.self != stream {
		return
	}
	stream.mu.Lock()
	stream.closing = true
	if stream.recovery != nil {
		stream.recovery.stop()
	}
	stream.cancel()
	stream.signalLocked()
	stream.mu.Unlock()
}

// Application settlement survives replacement, but the new Attachment must
// renew every Terminal proof before its clean Route close can retire the tail.
// The caller separately checks live authority and that exact child's witness.
func (stream *Stream) terminalTailCompleteLocked() bool {
	return stream.settled && stream.transfer.TerminalComplete()
}

// A clean Route terminal permits only TLS truncation after the authenticated
// exchange is complete. Joined authority, integrity and cleanup errors cannot
// be hidden merely because one of their causes is EOF.
func terminalTruncation(err error, received int) bool {
	if received != 0 {
		return false
	}
	if err == io.EOF || err == io.ErrUnexpectedEOF {
		return true
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		causes := joined.Unwrap()
		if len(causes) == 0 {
			return false
		}
		for _, cause := range causes {
			if !terminalTruncation(cause, 0) {
				return false
			}
		}
		return true
	}
	return false
}
