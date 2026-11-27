package channel

import (
	"errors"
	"fmt"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/domain/route/transport"
)

func (s *Session) physicalClose() error {
	s.physicalOnce.Do(func() {
		s.closeOutcome = s.conn.Close()
		if lower := lowerFramingLane(s.conn); lower != nil && s.closeOutcome != nil {
			// The exact lower lane retains whether CLOSE actually failed in
			// physical output. Its authority/pre-output refusal is still this
			// borrower's close outcome, but not an invented socket failure.
			cleanup, joined := lower.CleanupResult()
			if !joined {
				cleanup = errors.Join(cleanup, errors.New("original lower lane cleanup did not join"))
			}
			s.physicalErr = cleanup
		} else if lower == nil && s.closeOutcome != nil {
			s.physicalErr = &physicalCloseFailure{owner: s, cause: s.closeOutcome}
		}
	})
	return s.closeOutcome
}

func (s *Session) Retire(cause error) {
	s.mu.Lock()
	if !s.stopped {
		s.stopped = true
		s.failure = cause
		for _, l := range s.lanes {
			l.stopLocked(cause)
		}
	}
	s.mu.Unlock()
	s.cancel()
	_ = s.physicalClose()
}

func (s *Session) Close() error {
	s.closeOnce.Do(func() {
		s.Retire(nil)
		s.Start()
		<-s.readerDone
		s.children.Wait()
		// Every selected physical write has relinquished serialization before
		// this result can authorize reservation release.
		s.writes.Wait()
		s.deadlines.Wait()
		s.mu.Lock()
		s.closeErr = errors.Join(s.failure, s.closeOutcome, s.physicalErr, s.writeErr)
		s.queues.release(s.queued)
		s.queued = 0
		var returns []func()
		for _, l := range s.lanes {
			if l.queueBound != nil {
				l.queueBound.release(uint64(len(l.buffer)))
				if l.queueTermination {
					l.queueBound.release(ardp.HeaderSize + 1)
					l.queueTermination = false
				}
			}
			l.buffer = nil
			s.finishChildLocked(l)
			if l.afterFinish != nil {
				returns = append(returns, l.afterFinish)
				l.afterFinish = nil
			}
		}
		s.mu.Unlock()
		for _, release := range returns {
			release()
		}
		// A borrower can have detached its empty lane just before parent
		// retirement. Its already started physical return must still join.
		// Every retained lane is now finished, so no later Finish can add work.
		s.finishes.Wait()
	})
	return s.closeErr
}

// PhysicalFailure is read only after Close has joined all physical
// writers. Peer protocol refusal, EOF and authority cancellation remain local
// session outcomes; an owned physical write/close failure survives retirement.
func (s *Session) PhysicalFailure() error {
	return errors.Join(s.physicalErr, s.writeErr)
}

// Live is the framing owner's admission fact. Borrowers do not inspect its
// mutex or retirement state to decide whether an existing channel is usable.
func (s *Session) Live() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return !s.stopped && !s.finishingRole
}

// WaitTerminal joins the sole dedicated reader within the original channel
// horizon. Its caller still joins the writer and physical owner through Close.
func (s *Session) WaitTerminal(bound time.Time) error {
	if err := s.conn.SetReadDeadline(MinDeadline(s.end, bound)); err != nil {
		return err
	}
	<-s.readerDone
	return nil
}

// Done signals sole-reader retirement. Close still joins every writer and child.
func (s *Session) Done() <-chan struct{} { return s.readerDone }

// physicalWriteFailure is created only after a session selects and starts its
// original physical frame. Queue refusal and ordinary EOF cannot mint it.
type physicalWriteFailure struct {
	owner *Session
	kind  uint8
	cause error
}

func (e *physicalWriteFailure) Error() string {
	return fmt.Sprintf("route frame %d physical write: %v", e.kind, e.cause)
}
func (e *physicalWriteFailure) Unwrap() error { return e.cause }

// physicalCloseFailure is minted only by the exact session's owned Conn.Close.
// An equal native error elsewhere is not evidence of this physical operation.
type physicalCloseFailure struct {
	owner *Session
	cause error
}

func (e *physicalCloseFailure) Error() string {
	return fmt.Sprintf("route physical close: %v", e.cause)
}
func (e *physicalCloseFailure) Unwrap() error { return e.cause }

// TerminalFailureStage describes the complete retained error tree for command
// reporting. It does not turn any failure into success or change its outcome.
func TerminalFailureStage(err error) string {
	if err == nil {
		return "completed"
	}
	physical, peer := physicalFailureTree(err)
	var write *physicalWriteFailure
	hasWrite := errors.As(err, &write) && write.cause != nil
	physical = physical && closeOwnersHaveWrites(err)
	if physical && peer && hasWrite {
		return "peer-retired-write"
	}
	if physical && hasWrite {
		return "physical-write"
	}
	return "terminal"
}

func physicalFailureTree(err error) (physical, peer bool) {
	if err == nil {
		return false, false
	}
	if frame, ok := err.(*physicalWriteFailure); ok {
		return frame.cause != nil, transport.IsPeerRetirementCause(frame.cause)
	}
	if close, ok := err.(*physicalCloseFailure); ok {
		peer := close.owner != nil && transport.IsPeerRetirementCause(close.cause)
		return peer, peer
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		children := joined.Unwrap()
		if len(children) == 0 {
			return false, false
		}
		physical, peer = true, true
		for _, child := range children {
			p, c := physicalFailureTree(child)
			physical = physical && p
			peer = peer && c
		}
		return physical, peer
	}
	if wrapped, ok := err.(interface{ Unwrap() error }); ok {
		return physicalFailureTree(wrapped.Unwrap())
	}
	return false, false
}

func closeOwnersHaveWrites(err error) bool {
	writes := make(map[*Session]bool)
	var closes []*Session
	var visit func(error)
	visit = func(err error) {
		switch failure := err.(type) {
		case *physicalWriteFailure:
			if failure.owner != nil && failure.cause != nil {
				writes[failure.owner] = true
			}
		case *physicalCloseFailure:
			closes = append(closes, failure.owner)
		case interface{ Unwrap() []error }:
			for _, child := range failure.Unwrap() {
				visit(child)
			}
		case interface{ Unwrap() error }:
			visit(failure.Unwrap())
		}
	}
	visit(err)
	for _, owner := range closes {
		if owner == nil || !writes[owner] {
			return false
		}
	}
	return true
}
