package channel

import (
	"context"
	"errors"
	"net"
	"os"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/route/ardp"
)

// Open claims one child and emits its exact OPEN before giving it to the caller.
func (s *Session) Open(ctx, caller context.Context, body []byte) (*Lane, error) {
	if ctx == nil || caller == nil || len(body) != 49 && len(body) != 50 {
		return nil, errors.New("route OPEN shape invalid")
	}
	var opened ardp.Open
	var err error
	if len(body) == 50 {
		var envelope ardp.NodeOpen
		envelope, err = ardp.DecodeNodeOpen(body)
		opened = envelope.Recipient
	} else {
		opened, err = ardp.DecodeOpen(body)
	}
	if err != nil {
		return nil, err
	}
	if !time.Now().Before(opened.Deadline) {
		return nil, errors.New("route OPEN facts invalid")
	}
	// Allocation and complete OPEN emission share one bounded operation. The
	// peer enforces monotonically increasing IDs, so a later allocation must
	// not enter physical scheduling ahead of an earlier caller.
	openEnd := MinDeadline(MinDeadline(opened.Deadline, s.end), time.Now().Add(10*time.Second))
	timer := time.NewTimer(time.Until(openEnd))
	defer timer.Stop()
	select {
	case s.opening <- struct{}{}:
		defer func() { <-s.opening }()
	case <-s.ctx.Done():
		return nil, s.ctx.Err()
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-caller.Done():
		return nil, caller.Err()
	case <-timer.C:
		return nil, os.ErrDeadlineExceeded
	}
	if err := s.ctx.Err(); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := caller.Err(); err != nil {
		return nil, err
	}
	if !time.Now().Before(openEnd) {
		return nil, os.ErrDeadlineExceeded
	}
	s.mu.Lock()
	if s.stopped || s.finishingRole || s.next == 0 {
		s.mu.Unlock()
		return nil, errors.New("route child capacity unavailable")
	}
	id := s.next
	l := s.newChildLocked(id, body)
	if l == nil {
		s.mu.Unlock()
		return nil, errors.New("route child capacity unavailable")
	}
	s.next += 2
	// Cancellation of the physical child may lose the original deadline reason.
	// Retain the actual caller so retirement cannot resurrect expired output.
	l.caller = caller
	l.openEnd = openEnd
	s.mu.Unlock()
	interrupted := make(chan struct{})
	stop := context.AfterFunc(ctx, func() { defer close(interrupted); _ = l.Close() })
	defer func() {
		if !stop() {
			<-interrupted
		}
	}()
	if err := l.Bound(opened.Deadline); err != nil {
		_ = l.Close()
		l.Finish()
		return nil, err
	}
	l.writeMu.Lock()
	err = s.write(l, ardp.Frame{Kind: ardp.KindOpen, Lane: id, Body: body}, false)
	l.writeMu.Unlock()
	if err != nil {
		_ = l.Close()
		l.Finish()
		return nil, errors.Join(ctx.Err(), caller.Err(), err)
	}
	if err := errors.Join(ctx.Err(), caller.Err()); err != nil {
		closeErr := l.Close()
		l.Finish()
		return nil, errors.Join(err, closeErr)
	}
	return l, nil
}

// PrepareJoined reserves and installs the sole lane before an accepted
// RESULT can be emitted. Its owner starts reading only after the actual paired
// result barrier, or closes this preparation on any refusal.
func PrepareJoined(ctx context.Context, conn net.Conn, end time.Time, remaining uint64, check func() error, queues *Budget) (*Session, *Lane, error) {
	if ctx == nil || conn == nil || queues == nil || !time.Now().Before(end) || remaining < ardp.HeaderSize+1 {
		return nil, nil, errors.New("route joined framing bounds invalid")
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	if !queues.child() {
		return nil, nil, errors.New("route joined lane capacity unavailable")
	}
	s := Prepare(ctx, conn, end, remaining, check, false, queues, Handlers{})
	s.dedicated = true
	l := s.newLaneLocked(1)
	s.next, s.last = 3, 1
	return s, l, nil
}

// PrepareJoined keeps the shared physical-principal budget inside its
// session owner. The JOIN caller supplies its exact connection and bounds;
// preparing a lane reserves its bounded slot but starts no reader and grants
// no Network or Admission authority.
func (s *Session) PrepareJoined(ctx context.Context, conn net.Conn, end time.Time, remaining uint64, check func() error) (*Session, *Lane, error) {
	return PrepareJoined(ctx, conn, end, remaining, check, s.queues)
}
