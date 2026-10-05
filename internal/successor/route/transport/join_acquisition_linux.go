//go:build linux

package transport

import (
	"context"
	"errors"
	"net"
	"sync"
)

// JoinAcquisition retains the original physical Source opening, and for a
// Publisher also the exact Responder. It cannot resolve a replacement. Local
// acquisition is not Network or Service authority; Join rechecks real owners
// before and after every admission and handoff boundary.
type JoinAcquisition struct {
	source, responder *Prefix
	caller, ctx       context.Context
	cancel            context.CancelFunc
	mu                sync.Mutex
	closed, attempted bool
	opening           chan struct{}
	watchDone         chan struct{}
	stream            *Joined
	context           *JoinContext
	generation        *joinContextOpening
	once              sync.Once
	setupRetirement   error // stored before opening closes; guarded by mu
	result            error
}

func (p *Prefix) AcquireSourceJoin(ctx context.Context) (*JoinAcquisition, error) {
	return acquireJoin(ctx, p, nil)
}

func (p *Prefix) AcquireResponderJoin(ctx context.Context) (*JoinAcquisition, error) {
	if p == nil || p.source == nil {
		return nil, errors.New("route Responder opening absent")
	}
	return acquireJoin(ctx, p.source, p)
}

func acquireJoin(ctx context.Context, source, responder *Prefix) (*JoinAcquisition, error) {
	if ctx == nil || ctx.Err() != nil || source == nil || source.config.Leg.EntryMember.RoleDomain != 1 || responder != nil && responder.config.Leg.EntryMember.RoleDomain != 3 {
		return nil, errors.New("route JOIN original roles unavailable")
	}
	// Source precedes Responder in every joint lock operation. No I/O or join
	// occurs under either lock, and a closing role cannot acquire a borrower.
	source.registrationMu.Lock()
	defer source.registrationMu.Unlock()
	if responder != nil {
		responder.registrationMu.Lock()
		defer responder.registrationMu.Unlock()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	for _, p := range []*Prefix{source, responder} {
		if p != nil && p.localCurrent() != nil {
			return nil, errors.New("route JOIN original prefix retired")
		}
	}
	child, cancel := context.WithCancel(ctx)
	if err := errors.Join(ctx.Err(), child.Err()); err != nil {
		cancel()
		return nil, err
	}
	a := &JoinAcquisition{source: source, responder: responder, caller: ctx, ctx: child, cancel: cancel, watchDone: make(chan struct{})}
	for _, p := range []*Prefix{source, responder} {
		if p != nil {
			if p.joins == nil {
				p.joins = make(map[*JoinAcquisition]struct{})
			}
			p.joins[a] = struct{}{}
			p.changedActivity()
		}
	}
	go func() {
		defer close(a.watchDone)
		var responderDone <-chan struct{}
		if responder != nil {
			responderDone = responder.ctx.Done()
		}
		select {
		case <-source.ctx.Done():
		case <-responderDone:
		case <-child.Done():
		}
		cancel()
	}()
	return a, nil
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

func (p *Prefix) current() error {
	if err := p.localCurrent(); err != nil {
		return err
	}
	return errors.Join(p.originalCurrent(), p.localCurrent())
}

// A synchronous local seal denies new work but still permits bounded terminal
// traffic while this original physical generation and its authority are live.
func (p *Prefix) originalCurrent() error {
	if p.ctx == nil || p.caller == nil {
		return errors.New("route opening not published")
	}
	if err := errors.Join(p.caller.Err(), p.ctx.Err()); err != nil {
		return err
	}
	if p.config.Current == nil {
		return errors.New("route prefix Network unavailable")
	}
	view, err := p.config.Current()
	if err != nil {
		return err
	}
	return errors.Join(p.config.Leg.Check(view, view.ObservedAt()), p.caller.Err(), p.ctx.Err())
}

func (a *JoinAcquisition) current() error {
	if a == nil {
		return errors.New("route JOIN acquisition absent")
	}
	if err := errors.Join(a.caller.Err(), a.ctx.Err()); err != nil {
		return err
	}
	a.mu.Lock()
	closed := a.closed
	a.mu.Unlock()
	if closed {
		return net.ErrClosed
	}
	for _, p := range []*Prefix{a.source, a.responder} {
		if p != nil {
			if err := p.localCurrent(); err != nil {
				return err
			}
		}
	}
	if err := a.originalCurrent(); err != nil {
		return err
	}
	a.mu.Lock()
	closed = a.closed
	a.mu.Unlock()
	if closed {
		return net.ErrClosed
	}
	for _, p := range []*Prefix{a.source, a.responder} {
		if p != nil {
			if err := p.localCurrent(); err != nil {
				return err
			}
		}
	}
	return errors.Join(a.caller.Err(), a.ctx.Err())
}

// Terminal cleanup still obeys the original caller and physical generations
// after local admission seals.
func (a *JoinAcquisition) originalCurrent() error {
	for _, p := range []*Prefix{a.source, a.responder} {
		if p == nil {
			continue
		}
		if err := p.originalCurrent(); err != nil {
			return err
		}
	}
	return a.caller.Err()
}

func (a *JoinAcquisition) stop() {
	a.mu.Lock()
	a.closed = true
	stream := a.stream
	a.mu.Unlock()
	if stream != nil {
		stream.seal()
	}
	a.cancel()
}

// Publication and synchronous Prefix.Seal share the original role locks. An
// acquisition lock alone cannot prevent a prefix from sealing before a delayed
// stream is assigned. No observation or physical I/O runs inside this commit.
func (a *JoinAcquisition) publish(ctx context.Context, stream *Joined) error {
	a.source.registrationMu.Lock()
	defer a.source.registrationMu.Unlock()
	if a.responder != nil {
		a.responder.registrationMu.Lock()
		defer a.responder.registrationMu.Unlock()
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed || ctx.Err() != nil || a.caller.Err() != nil || a.ctx.Err() != nil {
		return errors.Join(net.ErrClosed, ctx.Err(), a.caller.Err(), a.ctx.Err())
	}
	for _, original := range []*Prefix{a.source, a.responder} {
		if original != nil {
			if err := original.localCurrent(); err != nil {
				return err
			}
		}
	}
	a.stream = stream
	return nil
}

// Close seals the acquisition synchronously, interrupts opening/stream I/O,
// joins them, then returns both original borrows. Repeated calls retain one
// terminal result. Prefix retirement uses this same join, never a timeout.
func (a *JoinAcquisition) Close() error {
	if a == nil {
		return nil
	}
	a.once.Do(func() {
		a.stop()
		a.mu.Lock()
		opening := a.opening
		a.mu.Unlock()
		if opening != nil {
			<-opening
		}
		<-a.watchDone
		a.mu.Lock()
		a.result = a.setupRetirement
		a.mu.Unlock()
		if a.stream != nil {
			a.result = errors.Join(a.result, a.stream.closePhysical())
		}
		for _, p := range []*Prefix{a.source, a.responder} {
			if p != nil {
				p.registrationMu.Lock()
				delete(p.joins, a)
				p.registrationMu.Unlock()
				p.changedActivity()
			}
		}
		a.mu.Lock()
		owner := a.context
		a.mu.Unlock()
		if owner != nil {
			owner.completed(a, a.result)
		}
	})
	return a.result
}
