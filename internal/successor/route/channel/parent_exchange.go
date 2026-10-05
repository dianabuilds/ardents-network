package channel

import (
	"context"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
	"time"
)

// refillCapacityDelta means no output started: release serialization before
// requesting the additional local capacity exposed by intervening traffic.
type refillCapacityDelta struct{ additional uint64 }

// parentExchange is the one in-flight lane-zero exchange. Its fields belong
// to the session lock; the control lane borrows this same state rather than
// keeping another accounting witness or capacity counter.
type parentExchange struct {
	caller       context.Context
	reply        chan ardp.Frame
	charged      bool
	witness      uint64
	remaining    uint64
	held         uint64
	requiresHold bool
}

func (e *refillCapacityDelta) Error() string { return "route refill capacity changed before output" }

// replenish keeps one requested control exchange on the original parent. Its
// sole reader delivers ACCEPT; presentation cannot start a second reader.
func (s *Session) ExchangeParent(ctx, caller context.Context, end time.Time, remaining uint64, prepare func(context.Context) (ardp.Frame, error), holds ...func(context.Context, uint64) error) error {
	if ctx == nil || caller == nil || prepare == nil || remaining == 0 {
		return errors.New("route replenishment composition unavailable")
	}
	s.mu.Lock()
	if s.stopped || s.dedicated || s.exchange != nil || s.parentControl != nil || s.used >= s.limit || end != s.end {
		s.mu.Unlock()
		return errors.New("route replenishment parent unavailable")
	}
	exchange := &parentExchange{remaining: remaining}
	s.exchange = exchange
	s.mu.Unlock()
	defer func() { s.mu.Lock(); s.exchange = nil; s.mu.Unlock() }()
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
	frame, err := prepare(ctx)
	if err != nil {
		return err
	}
	defer clear(frame.Body)
	if err := check(); err != nil {
		return err
	}
	op, cancel := context.WithCancelCause(s.ctx)
	defer cancel(nil)
	request := requestContext{Context: ctx, caller: caller}
	control := &Lane{s: s, end: s.end, hardEnd: s.end, writeEnd: s.end, changed: make(chan struct{}), ctx: op, caller: request, cancel: cancel}
	if end, ok := ctx.Deadline(); ok && end.Before(control.writeEnd) {
		control.writeEnd = end
	}
	reply := make(chan ardp.Frame, 1)
	s.mu.Lock()
	exchange.reply = reply
	exchange.caller = request
	control.refill = exchange
	s.mu.Unlock()
	interrupted := make(chan struct{})
	stop := context.AfterFunc(ctx, func() {
		defer close(interrupted)
		s.mu.Lock()
		control.stopLocked(ctx.Err())
		err := control.interruptOutputLocked()
		s.mu.Unlock()
		if err != nil {
			s.Retire(err)
		}
	})
	defer func() {
		if !stop() {
			<-interrupted
		}
	}()
	exchange.requiresHold = len(holds) != 0 && holds[0] != nil
	for {
		if err := check(); err != nil {
			return err
		}
		err := s.write(control, frame, false)
		var changed *refillCapacityDelta
		if errors.As(err, &changed) {
			// The selected turn has returned without charging/emitting ADMIT.
			// Child frames continue while Hosting performs durable reservation.
			if err := check(); err != nil {
				return err
			}
			if err := holds[0](ctx, changed.additional); err != nil {
				return err
			}
			s.mu.Lock()
			exchange.held += changed.additional
			s.mu.Unlock()
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
		s.Retire(ctx.Err())
		return ctx.Err()
	case <-caller.Done():
		s.Retire(caller.Err())
		return caller.Err()
	case <-s.ctx.Done():
		return s.ctx.Err()
	}
	status, credit, err := ardp.DecodeAcceptFrame(accepted)
	if err = errors.Join(err, check()); err != nil || status != 0 || credit != Window || !time.Now().Before(s.end) {
		err = errors.Join(errors.New("route refill acknowledgement refused"), err)
		s.Retire(err)
		return err
	}
	// The sole reader committed the replacement before debiting ACCEPT.
	// Completion observes that transition; it never replaces allowance again.
	if err := check(); err != nil {
		s.Retire(err)
		return err
	}
	return nil
}

// acceptParent owns the receiving framing commit and its separately accounted
// ACK. The operation supplies bytes only after genuine verification, capacity
// and durable spend; the framing owner alone changes the usage limit and emits
// the canonical control frame within its immutable physical horizon.
func (s *Session) AcceptParent(ctx context.Context, witness, remaining uint64) error {
	if err := s.replaceRemaining(witness, remaining); err != nil {
		return err
	}
	control := &Lane{s: s, end: s.end, writeEnd: s.end, changed: make(chan struct{}), ctx: ctx}
	accepted, err := ardp.AcceptFrame(0, Window)
	if err != nil {
		return err
	}
	return s.write(control, accepted, false)
}
