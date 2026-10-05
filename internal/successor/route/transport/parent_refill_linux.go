//go:build linux

package transport

import (
	"context"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
)

// refillCapacityDelta means no output started: release serialization before
// requesting the additional local capacity exposed by intervening traffic.
type refillCapacityDelta struct{ additional uint64 }

func (e *refillCapacityDelta) Error() string { return "route refill capacity changed before output" }

// replenish keeps one requested control exchange on the original parent. Its
// sole reader delivers ACCEPT; presentation cannot start a second reader.
func (s *session) replenish(ctx, caller context.Context, hello ardp.Hello, present Present, holds ...func(context.Context, uint64) error) error {
	if ctx == nil || caller == nil || present == nil || hello.Purpose != ardp.PurposeForwarding {
		return errors.New("route replenishment composition unavailable")
	}
	s.mu.Lock()
	if s.stopped || s.dedicated || s.parentBusy || s.parentControl != nil || s.used >= s.limit || hello.Deadline != s.end {
		s.mu.Unlock()
		return errors.New("route replenishment parent unavailable")
	}
	s.parentBusy = true
	s.parentAdmitCharged = false
	s.mu.Unlock()
	defer func() { s.mu.Lock(); s.parentBusy = false; s.parentReply = nil; s.parentCaller = nil; s.mu.Unlock() }()
	check := func() error {
		if err := errors.Join(ctx.Err(), caller.Err(), s.ctx.Err()); err != nil {
			return err
		}
		if s.check != nil {
			return errors.Join(s.check(), ctx.Err(), caller.Err(), s.ctx.Err())
		}
		return nil
	}
	if err := check(); err != nil {
		return err
	}
	raw, err := present(ctx, hello)
	if err != nil {
		return err
	}
	defer clear(raw)
	if len(raw) != 354 {
		return errors.New("route refill token length invalid")
	}
	if err := check(); err != nil {
		return err
	}
	op, cancel := context.WithCancelCause(s.ctx)
	defer cancel(nil)
	control := &lane{s: s, end: s.end, hardEnd: s.end, writeEnd: s.end, changed: make(chan struct{}), ctx: op, caller: ctx, cancel: cancel}
	if end, ok := ctx.Deadline(); ok && end.Before(control.writeEnd) {
		control.writeEnd = end
	}
	reply := make(chan ardp.Frame, 1)
	s.mu.Lock()
	s.parentReply = reply
	s.parentCaller = ctx
	s.mu.Unlock()
	interrupted := make(chan struct{})
	stop := context.AfterFunc(ctx, func() {
		defer close(interrupted)
		s.mu.Lock()
		control.stopLocked(ctx.Err())
		err := control.interruptOutputLocked()
		s.mu.Unlock()
		if err != nil {
			s.retire(err)
		}
	})
	defer func() {
		if !stop() {
			<-interrupted
		}
	}()
	body := append([]byte{2}, raw...)
	defer clear(body)
	control.refillRequiresHold = len(holds) != 0 && holds[0] != nil
	for {
		if err := check(); err != nil {
			return err
		}
		err := s.write(control, ardp.Frame{Kind: ardp.KindAdmit, Body: body}, false)
		var changed *refillCapacityDelta
		if errors.As(err, &changed) {
			// The selected turn has returned without charging/emitting ADMIT.
			// Child frames continue while Hosting performs durable reservation.
			if err := holds[0](ctx, changed.additional); err != nil {
				return err
			}
			control.refillHeld += changed.additional
			continue
		}
		if err != nil {
			return errors.Join(err, ctx.Err(), caller.Err())
		}
		break
	}
	// Once token bytes were emitted, lost acknowledgement cannot be retried on
	// this parent: retire uncertainty and preserve irreversible presentation.
	var accepted ardp.Frame
	select {
	case accepted = <-reply:
	case <-ctx.Done():
		s.retire(ctx.Err())
		return ctx.Err()
	case <-caller.Done():
		s.retire(caller.Err())
		return caller.Err()
	case <-s.ctx.Done():
		return s.ctx.Err()
	}
	status, credit, err := ardp.DecodeAcceptFrame(accepted)
	if err = errors.Join(err, check()); err != nil || status != 0 || credit != window || !time.Now().Before(s.end) {
		err = errors.Join(errors.New("route refill acknowledgement refused"), err)
		s.retire(err)
		return err
	}
	if err := s.replaceRemaining(control.refillWitness, admission.ForwardClass.ByteLimit()); err != nil {
		s.retire(err)
		return err
	}
	return check()
}
