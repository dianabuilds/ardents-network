package prefix

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network"
	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
	framing "github.com/dianabuilds/ardents-network/internal/successor/route/channel"
	"github.com/dianabuilds/ardents-network/internal/successor/route/role"
	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
	roletls "github.com/dianabuilds/ardents-network/internal/successor/route/transport/tls"
)

// IntroductionChannel owns the original setup, retained leg/recipient
// checks and authenticated child admission. Registration owns operation bytes,
// ACK interpretation and its published reader/writer lifetime. No slot or
// Publication authority follows merely from opening this channel.
type IntroductionChannel struct {
	ctx          context.Context
	cancel       context.CancelFunc
	caller       context.Context
	opening      *prefixOpening
	check        func() error
	conn         net.Conn
	lane         *framing.Lane
	release      func()
	joinCaller   func()
	interruption prefixSetupInterruption
}

func (p *Prefix) OpenIntroductionChannel(ctx context.Context, duty network.RetainedDuty, end time.Time) (_ *IntroductionChannel, result error) {
	child, cancel, opening, err := p.beginTerminalOpening(ctx, end)
	if err != nil {
		return nil, err
	}
	t := &IntroductionChannel{ctx: child, caller: ctx, cancel: cancel, opening: opening}
	defer func() {
		if result != nil {
			result = errors.Join(result, t.CloseSetup())
			opening.finish()
		}
	}()
	a := role.Authority{Current: p.observeOriginal, Duty: duty, Profile: p.config.Leg.Profile}
	t.check = func() error {
		// Physical interruption cannot stand in for original caller checks at
		// effects and after Network's durable observation.
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := p.ctx.Err(); err != nil {
			return err
		}
		if !time.Now().Before(end) {
			return errors.New("route registration expired")
		}
		view, err := p.observeOriginal()
		if err != nil {
			return err
		}
		if err := p.config.Leg.Check(view, time.Now()); err != nil {
			return err
		}
		member, err := a.Member()
		if err != nil || member.RoleDomain != 4 || member.Subrole != 3 || p.config.Leg.EntryMember.RoleDomain != 4 || end.After(p.config.Deadline) || end.After(member.NotAfter()) || end.After(a.Profile.NotAfter) || role.Conflicting(member, p.config.Leg.EntryMember) || role.Conflicting(member, p.config.Leg.InteriorMember) {
			return errors.Join(errors.New("route registration duty differs"), err)
		}
		if !time.Now().Before(end) {
			return errors.New("route registration expired")
		}
		return errors.Join(ctx.Err(), p.ctx.Err(), child.Err())
	}
	if err := t.check(); err != nil {
		return nil, err
	}
	t.release, err = p.interior.HoldControl()
	if err != nil {
		return nil, err
	}
	callerDone := make(chan struct{})
	stopCaller := context.AfterFunc(ctx, func() { defer close(callerDone); cancel() })
	var callerJoin sync.Once
	t.joinCaller = func() {
		callerJoin.Do(func() {
			if !stopCaller() {
				<-callerDone
			}
		})
	}
	member, err := a.Member()
	if err != nil {
		return nil, err
	}
	t.lane, err = p.interior.Open(child, ctx, ardp.EncodeOpen(ardp.Open{RecipientNodeID: member.NodeID, RecipientDutyGeneration: member.DutyGeneration, Purpose: uint8(ardp.PurposeIntroduction), Deadline: end}, false))
	if err != nil {
		return nil, err
	}
	done := make(chan struct{})
	t.interruption.done = done
	t.interruption.stop = context.AfterFunc(child, func() { defer close(done); _ = t.lane.Close() })
	secured, err := roletls.OpenRole(child, t.lane, member.PublicKey, minDeadline(end, time.Now().Add(10*time.Second)))
	if err != nil {
		return nil, err
	}
	t.conn = transport.Retain(secured)
	if err := t.conn.SetDeadline(minDeadline(end, time.Now().Add(10*time.Second))); err != nil {
		return nil, err
	}
	hello, err := a.FreshHello(end, ardp.PurposeIntroduction, false)
	if err == nil {
		err = role.Present(child, ctx, t.conn, a, hello, p.config.Present)
	}
	if err != nil {
		return nil, err
	}
	if err := t.conn.SetDeadline(end); err != nil {
		return nil, err
	}
	return t, nil
}

// A failed operation still joins this original physical setup before its
// pending claim completes. Stock presentation and receiving spend are never
// refunded here; the owning domains retain those irreversible transitions.
func (t *IntroductionChannel) CloseSetup() error {
	t.cancel()
	t.interruption.join()
	var result error
	if t.conn != nil {
		result = errors.Join(result, t.conn.Close())
	}
	if t.lane != nil {
		result = errors.Join(result, t.lane.Close())
	}
	t.FinishParent()
	t.JoinCaller()
	return result
}

// The original terminal keeps cancellation callback ownership through handoff.
// Registration joins it before waiting for its own readers and withdrawal.
func (t *IntroductionChannel) JoinCaller() {
	if t.joinCaller != nil {
		t.joinCaller()
	}
}

// FinishParent returns the original lower lane/control hold only after the
// Registration owner joins its physical I/O and complete withdrawal attempt.
// Failed setup follows the same return after joining its interruption callback.
func (t *IntroductionChannel) FinishParent() {
	if t.lane != nil {
		t.lane.Finish()
	}
	if t.release != nil {
		t.release()
	}
}

func (t *IntroductionChannel) StopOpening() error {
	if t.interruption.join() {
		return t.ctx.Err()
	}
	return nil
}

func (t *IntroductionChannel) Borrow(interrupt func(), join func() error, active func() bool) *Borrow {
	return &Borrow{owner: t.opening.owner, interrupt: interrupt, join: join, active: active}
}

func (t *IntroductionChannel) Publish(borrow *Borrow) error {
	return t.opening.publishBorrow(t.caller, t.ctx, borrow)
}

func (t *IntroductionChannel) FinishSetup() { t.opening.finish() }

// Context is the retained terminal lifetime, distinct from its original caller.
func (t *IntroductionChannel) Context() context.Context { return t.ctx }

// Cancel interrupts this terminal without canceling its original parent.
func (t *IntroductionChannel) Cancel() { t.cancel() }

// Check reobserves the original physical Prefix and exact recipient bounds.
func (t *IntroductionChannel) Check() error { return t.check() }

// Stream supplies ordered role bytes without exposing the lower parent/hold.
func (t *IntroductionChannel) Stream() net.Conn { return t.conn }
