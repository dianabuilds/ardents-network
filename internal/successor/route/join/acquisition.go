package join

import (
	"context"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/successor/network"
	"github.com/dianabuilds/ardents-network/internal/successor/route/prefix"
	"net"
	"sync"
	"time"
)

// joinClaim is the acquisition's exact borrowed-generation contract. Production
// claims are issued only by Prefix; tests may supply refusal/retirement controls,
// never a successful role opening or authority observation.
type joinClaim interface {
	CheckLocal() error
	CheckOriginal() error
	WaitRetirement(context.Context)
	CheckRecipient(network.RetainedDuty, time.Time) error
	OpenChannel(context.Context, context.Context, network.RetainedDuty, time.Time, time.Time) (*prefix.JoinChannel, uint8, error)
	Publish(context.Context, context.Context, context.Context, func(func() error) error) error
	ReturnJoined()
}

// JoinAcquisition retains the original physical Source opening, and for a
// Publisher also the exact Responder. It cannot resolve a replacement. Local
// acquisition is not Network or Service authority; Join rechecks real owners
// before and after every admission and handoff boundary.
type JoinAcquisition struct {
	borrows           joinClaim
	caller, ctx       context.Context
	cancel            context.CancelFunc
	mu                sync.Mutex
	closed, attempted bool
	opening           chan struct{}
	watchDone         chan struct{}
	stream            *Joined
	completion        func(error) // immutable original owner's joined notification
	once              sync.Once
	setupRetirement   error // stored before opening closes; guarded by mu
	result            error
}

// AcquireSourceJoin retains one exact Source for a single JOIN attempt.
func AcquireSourceJoin(ctx context.Context, p *prefix.Prefix) (*JoinAcquisition, error) {
	return acquireJoin(ctx, p, false)
}

// AcquireResponderJoin retains the Responder and its immutable original Source.
// prefix.Prefix resolves that binding; the caller cannot supply a replacement Source.
func AcquireResponderJoin(ctx context.Context, p *prefix.Prefix) (*JoinAcquisition, error) {
	return acquireJoin(ctx, p, true)
}

func acquireJoin(ctx context.Context, p *prefix.Prefix, responder bool) (*JoinAcquisition, error) {
	if p == nil {
		return nil, errors.New("route JOIN original roles unavailable")
	}
	return startAcquisition(ctx, func(stop func(), join func() error) (joinClaim, error) {
		if responder {
			return p.BorrowResponderJoin(ctx, stop, join)
		}
		return p.BorrowSourceJoin(ctx, stop, join)
	})
}

// startAcquisition binds the original caller before obtaining its exact claim.
// Source and Responder use the same acquisition lifetime and physical join.
func startAcquisition(ctx context.Context, borrow func(func(), func() error) (joinClaim, error)) (*JoinAcquisition, error) {
	if ctx == nil || ctx.Err() != nil || borrow == nil {
		return nil, errors.New("route JOIN original roles unavailable")
	}
	child, cancel := context.WithCancel(ctx)
	if err := errors.Join(ctx.Err(), child.Err()); err != nil {
		cancel()
		return nil, err
	}
	a := &JoinAcquisition{caller: ctx, ctx: child, cancel: cancel, watchDone: make(chan struct{})}
	claim, err := borrow(a.stop, a.Close)
	if err != nil {
		cancel()
		return nil, err
	}
	a.borrows = claim
	go func() {
		defer close(a.watchDone)
		claim.WaitRetirement(child)
		cancel()
	}()
	return a, nil
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
	if err := a.borrows.CheckLocal(); err != nil {
		return err
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
	if err := a.borrows.CheckLocal(); err != nil {
		return err
	}
	return errors.Join(a.caller.Err(), a.ctx.Err())
}

// Terminal cleanup still obeys the original caller and physical generations
// after local admission seals.
func (a *JoinAcquisition) originalCurrent() error {
	if err := a.borrows.CheckOriginal(); err != nil {
		return err
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

// Publication and synchronous prefix.Prefix.Seal share the original role locks. An
// acquisition lock alone cannot prevent a prefix from sealing before a delayed
// stream is assigned. No observation or physical I/O runs inside this commit.
func (a *JoinAcquisition) publish(ctx context.Context, stream *Joined) error {
	return a.borrows.Publish(ctx, a.caller, a.ctx, func(checkOriginal func() error) error {
		a.mu.Lock()
		defer a.mu.Unlock()
		if a.closed || ctx.Err() != nil || a.caller.Err() != nil || a.ctx.Err() != nil {
			return errors.Join(net.ErrClosed, ctx.Err(), a.caller.Err(), a.ctx.Err())
		}
		if err := checkOriginal(); err != nil {
			return err
		}
		a.stream = stream
		return nil
	})
}

// Close seals the acquisition synchronously, interrupts opening/stream I/O,
// joins them, then returns both original borrows. Repeated calls retain one
// terminal result. prefix.Prefix retirement uses this same join, never a timeout.
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
		a.borrows.ReturnJoined()
		a.mu.Lock()
		completion := a.completion
		a.mu.Unlock()
		if completion != nil {
			completion(a.result)
		}
	})
	return a.result
}
