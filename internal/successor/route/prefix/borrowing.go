package prefix

import (
	"context"
	"errors"
	"net"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network"
)

// Borrow is one published physical borrower of this exact generation.
// The borrower owns its operation and terminal result; Prefix only interrupts
// and joins it before retiring framing or returning generation resources.
// The callbacks are captured once before publication, never rebound to a
// replacement. They run under the same ordering as the original owner locks.
type Borrow struct {
	owner     *Prefix
	interrupt func()
	join      func() error
	active    func() bool
}

// BorrowLifetime retains this already opened physical generation for one
// bounded live operation, including quiet intervals between its exchanges.
// It supplies no token, recipient or JOIN authority. Interrupt must only seal;
// join runs outside the generation lock and must finish the borrower's users,
// not recursively Close this Prefix. ReturnJoined follows that actual join.
func (p *Prefix) BorrowLifetime(ctx context.Context, interrupt func(), join func() error) (*Borrow, error) {
	if p == nil || ctx == nil || interrupt == nil || join == nil {
		return nil, errors.New("route retained physical lifetime absent")
	}
	end, bounded := ctx.Deadline()
	if !bounded || p.config.Deadline.IsZero() || end.After(p.config.Deadline) || !time.Now().Before(end) {
		return nil, errors.New("route retained physical lifetime exceeds original bound")
	}
	p.lifetimeMu.Lock()
	defer p.lifetimeMu.Unlock()
	if err := errors.Join(ctx.Err(), p.localCurrent()); err != nil {
		return nil, err
	}
	borrow := &Borrow{owner: p, interrupt: interrupt, join: join, active: func() bool { return ctx.Err() == nil }}
	p.retainBorrowLocked(borrow)
	return borrow, nil
}

// retainBorrowLocked is part of the existing synchronous handoff: callers
// hold the original generation lock and have checked its seal and caller.
func (p *Prefix) retainBorrowLocked(borrow *Borrow) {
	if p.borrows == nil {
		p.borrows = make(map[*Borrow]struct{})
	}
	p.borrows[borrow] = struct{}{}
	p.changedActivity()
}

// ReturnJoined runs only after the owning operation joins physical work.
// Removing this exact claim cannot release a sibling or successor generation.
func (b *Borrow) ReturnJoined() {
	if b == nil {
		return
	}
	b.owner.lifetimeMu.Lock()
	delete(b.owner.borrows, b)
	b.owner.lifetimeMu.Unlock()
	b.owner.changedActivity()
}

func (b *Borrow) ChangedActivity() { b.owner.changedActivity() }

func (p *Prefix) changedActivity() {
	select {
	case p.activity <- struct{}{}:
	default:
	}
}

// JoinBorrow owns the atomic claims on one exact Source and its optional
// original Responder. Prefix owns admission and publication ordering; JOIN owns
// the borrower state, physical attempt and joined terminal result.
type JoinBorrow struct {
	source, responder *Prefix
	claims            [2]*Borrow
}

func (p *Prefix) BorrowResponderJoin(ctx context.Context, interrupt func(), join func() error) (*JoinBorrow, error) {
	if p == nil || p.source == nil {
		return nil, errors.New("route Responder opening absent")
	}
	return p.source.borrowJoin(ctx, p, interrupt, join)
}

func (b *JoinBorrow) CheckLocal() error {
	for _, original := range []*Prefix{b.source, b.responder} {
		if original != nil {
			if err := original.localCurrent(); err != nil {
				return err
			}
		}
	}
	return nil
}

func (b *JoinBorrow) CheckOriginal() error {
	for _, original := range []*Prefix{b.source, b.responder} {
		if original != nil {
			if err := original.originalCurrent(); err != nil {
				return err
			}
		}
	}
	return nil
}

func (b *JoinBorrow) WaitRetirement(ctx context.Context) {
	var responderDone <-chan struct{}
	if b.responder != nil {
		responderDone = b.responder.ctx.Done()
	}
	select {
	case <-b.source.ctx.Done():
	case <-responderDone:
	case <-ctx.Done():
	}
}

func (b *JoinBorrow) CheckRecipient(duty network.RetainedDuty, end time.Time) error {
	for _, original := range []*Prefix{b.source, b.responder} {
		if original != nil {
			if err := original.joinRecipient(duty, end); err != nil {
				return err
			}
		}
	}
	return nil
}

func (b *JoinBorrow) OpenChannel(ctx, caller context.Context, duty network.RetainedDuty, end, setup time.Time) (*JoinChannel, uint8, error) {
	parent, side := b.source, uint8(1)
	if b.responder != nil {
		parent, side = b.responder, 2
	}
	terminal, err := parent.openJoinChannel(ctx, caller, duty, end, setup)
	return terminal, side, err
}

func (source *Prefix) borrowJoin(ctx context.Context, responder *Prefix, interrupt func(), join func() error) (*JoinBorrow, error) {
	if ctx == nil || ctx.Err() != nil || source == nil || source.config.Leg.EntryMember.RoleDomain != 1 || responder != nil && (responder.config.Leg.EntryMember.RoleDomain != 3 || responder.source != source) {
		return nil, errors.New("route JOIN original roles unavailable")
	}
	// All pair transitions lock Source before Responder. Neither observation,
	// physical I/O nor join runs while these generation locks are held.
	source.lifetimeMu.Lock()
	defer source.lifetimeMu.Unlock()
	if responder != nil {
		responder.lifetimeMu.Lock()
		defer responder.lifetimeMu.Unlock()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	for _, original := range []*Prefix{source, responder} {
		if original != nil && original.localCurrent() != nil {
			return nil, errors.New("route JOIN original prefix retired")
		}
	}
	borrow := &JoinBorrow{source: source, responder: responder}
	for i, original := range []*Prefix{source, responder} {
		if original != nil {
			borrow.claims[i] = &Borrow{owner: original, interrupt: interrupt, join: join, active: func() bool { return true }}
			original.retainBorrowLocked(borrow.claims[i])
		}
	}
	return borrow, nil
}

// publish serializes the caller's local handoff with both original generations'
// synchronous seals. The handoff only updates borrower state; no I/O or join.
func (b *JoinBorrow) Publish(ctx, caller, operation context.Context, handoff func(checkOriginal func() error) error) error {
	return b.source.CommitPair(b.responder, func(checkOriginal func() error) error {
		return handoff(func() error {
			if ctx.Err() != nil || caller.Err() != nil || operation.Err() != nil {
				return errors.Join(net.ErrClosed, ctx.Err(), caller.Err(), operation.Err())
			}
			return checkOriginal()
		})
	})
}

func (b *JoinBorrow) ReturnJoined() {
	for _, claim := range b.claims {
		claim.ReturnJoined()
	}
}

func (p *Prefix) localCurrent() error {
	if p.ctx == nil || p.caller == nil {
		return errors.New("route opening not published")
	}
	select {
	case <-p.closing:
		return net.ErrClosed
	default:
	}
	return errors.Join(p.caller.Err(), p.ctx.Err())
}

func (p *Prefix) Current() error {
	if err := p.localCurrent(); err != nil {
		return err
	}
	return errors.Join(p.originalCurrent(), p.localCurrent())
}

// A local seal denies new work; terminal traffic still checks this original
// physical generation, caller and fresh authority rather than a replacement.
func (p *Prefix) originalCurrent() error {
	if p.ctx == nil || p.caller == nil {
		return errors.New("route opening not published")
	}
	if err := errors.Join(p.caller.Err(), p.ctx.Err()); err != nil {
		return err
	}
	view, err := p.observeOriginal()
	if err != nil {
		return err
	}
	return errors.Join(p.config.Leg.Check(view, view.ObservedAt()), p.caller.Err(), p.ctx.Err())
}

// observeOriginal is the physical generation's observation seam. Every terminal
// role/recipient observation retains the original Prefix caller, including
// cancellation during a failed read. Local seal still permits bounded terminal
// cleanup; it cannot replace the caller or renew the physical lifetime.
func (p *Prefix) observeOriginal() (network.RuntimeView, error) {
	if p.ctx == nil || p.caller == nil {
		return network.RuntimeView{}, errors.New("route opening not published")
	}
	if err := errors.Join(p.caller.Err(), p.ctx.Err()); err != nil {
		return network.RuntimeView{}, err
	}
	if p.config.Current == nil {
		return network.RuntimeView{}, errors.New("route prefix Network unavailable")
	}
	view, err := p.config.Current()
	return view, errors.Join(err, p.caller.Err(), p.ctx.Err())
}

// CommitPair serializes a local transition with both original physical seals.
// Prefix owns these generation locks and their currentness checks; its consumer
// supplies only its own bounded state handoff, never observation, I/O or join.
// Every pair transition acquires Source before its original Responder.
func (source *Prefix) CommitPair(responder *Prefix, handoff func(checkOriginal func() error) error) error {
	source.lifetimeMu.Lock()
	defer source.lifetimeMu.Unlock()
	if responder != nil {
		responder.lifetimeMu.Lock()
		defer responder.lifetimeMu.Unlock()
	}
	return handoff(func() error {
		var result error
		for _, original := range []*Prefix{source, responder} {
			if original != nil {
				result = errors.Join(result, original.localCurrent())
			}
		}
		return result
	})
}

// BorrowSourceJoin retains this exact Source until its operation joins.
// The callbacks are immutable; they must seal without I/O and join outside locks.
func (p *Prefix) BorrowSourceJoin(ctx context.Context, interrupt func(), join func() error) (*JoinBorrow, error) {
	return p.borrowJoin(ctx, nil, interrupt, join)
}
