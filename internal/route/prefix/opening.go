package prefix

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"
)

// An original Prefix keeps setup cancellation and completion until the result
// joins or transfers to a published borrower. The operation owns its rules;
// this claim grants no authority and cannot publish into a replacement.
type prefixOpening struct {
	owner     *Prefix
	cancel    context.CancelFunc
	once      sync.Once
	published bool // guarded by the original owner's lifetimeMu
}

// beginTerminalOpening retains setup under this original generation's lock.
// Its derived context interrupts I/O; the caller still checks its original
// request around effects and handoff. Setup itself grants no role authority.
func (p *Prefix) beginTerminalOpening(caller context.Context, end time.Time) (context.Context, context.CancelFunc, *prefixOpening, error) {
	p.lifetimeMu.Lock()
	defer p.lifetimeMu.Unlock()
	select {
	case <-p.closing:
		return nil, nil, nil, net.ErrClosed
	default:
	}
	if err := p.localCurrent(); err != nil {
		return nil, nil, nil, err
	}
	if caller == nil {
		return nil, nil, nil, errors.New("route terminal opening caller unavailable")
	}
	if err := caller.Err(); err != nil {
		return nil, nil, nil, err
	}
	child, cancel := context.WithDeadline(p.ctx, end)
	return child, cancel, p.retainOpeningLocked(cancel), nil
}

func (p *Prefix) beginOpening(cancel context.CancelFunc) (*prefixOpening, error) {
	p.lifetimeMu.Lock()
	defer p.lifetimeMu.Unlock()
	if err := p.localCurrent(); err != nil {
		return nil, err
	}
	return p.retainOpeningLocked(cancel), nil
}

func (p *Prefix) retainOpeningLocked(cancel context.CancelFunc) *prefixOpening {
	opening := &prefixOpening{owner: p, cancel: cancel}
	if p.setups == nil {
		p.setups = make(map[*prefixOpening]struct{})
	}
	p.setups[opening] = struct{}{}
	p.openings.Add(1)
	return opening
}

func (o *prefixOpening) finish() {
	o.once.Do(func() {
		o.owner.lifetimeMu.Lock()
		delete(o.owner.setups, o)
		o.owner.lifetimeMu.Unlock()
		o.owner.openings.Done()
	})
}

// publishBorrow transfers a still-retained setup to exactly one borrower of
// this generation. Seal and publication serialize; cleanup and join stay with
// the borrower outside this lock, including a failed publication after ACK.
func (o *prefixOpening) publishBorrow(caller, child context.Context, borrow *Borrow) error {
	p := o.owner
	p.lifetimeMu.Lock()
	defer p.lifetimeMu.Unlock()
	if _, held := p.setups[o]; !held || o.published || caller == nil || child == nil || borrow == nil || borrow.owner != p || borrow.interrupt == nil || borrow.join == nil || borrow.active == nil {
		return errors.New("route terminal opening handoff unavailable")
	}
	if err := p.localCurrent(); err != nil {
		return err
	}
	if err := errors.Join(caller.Err(), child.Err()); err != nil {
		return err
	}
	o.published = true
	p.retainBorrowLocked(borrow)
	return nil
}

// minDeadline bounds operation setup by both original owners. The channel
// independently enforces its own original deadline at each physical effect.
func minDeadline(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}
