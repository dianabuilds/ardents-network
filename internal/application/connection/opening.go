package connection

import (
	"context"
	"sync"
	"time"
)

const wholeOpenLifetime = 10 * time.Second

// opening owns a finite setup timer independently of the accepted lifetime.
// Acceptance and expiry compete under one lock; stopping joins a callback
// already selected by the timer before its context can be transferred.
type opening struct {
	ctx      context.Context
	cancel   context.CancelCauseFunc
	end      time.Time
	mu       sync.Mutex
	accepted bool
	timer    *time.Timer
	done     chan struct{}
}

func newOpening(parent context.Context) *opening {
	ctx, cancel := context.WithCancelCause(parent)
	owner := &opening{ctx: ctx, cancel: cancel, end: time.Now().Add(wholeOpenLifetime), done: make(chan struct{})}
	if end, available := parent.Deadline(); available && end.Before(owner.end) {
		owner.end = end
	}
	owner.timer = time.AfterFunc(time.Until(owner.end), func() {
		defer close(owner.done)
		owner.mu.Lock()
		if !owner.accepted {
			owner.cancel(context.DeadlineExceeded)
		}
		owner.mu.Unlock()
	})
	return owner
}

func (owner *opening) accept(commit func() error) error {
	owner.mu.Lock()
	var failure error
	if owner.ctx.Err() != nil {
		failure = context.Cause(owner.ctx)
	} else if !time.Now().Before(owner.end) {
		failure = context.DeadlineExceeded
	} else if commit != nil {
		failure = commit()
	}
	if failure == nil && owner.ctx.Err() != nil {
		failure = context.Cause(owner.ctx)
	}
	// A successful physical ACCEPT operation is the commit witness. Its
	// transport deadline bounds the operation; delayed goroutine resumption
	// must not let the setup timer revoke an already observed acceptance.
	if failure == nil {
		owner.accepted = true
	} else {
		owner.cancel(failure)
	}
	owner.mu.Unlock()
	owner.joinTimer()
	return failure
}

func (owner *opening) joinTimer() {
	if owner.timer.Stop() {
		close(owner.done)
	} else {
		<-owner.done
	}
}

func (owner *opening) close() {
	owner.mu.Lock()
	owner.accepted = true
	owner.mu.Unlock()
	owner.joinTimer()
	owner.cancel(context.Canceled)
}

func (owner *opening) failure(fallback error) error {
	if cause := context.Cause(owner.ctx); cause != nil {
		return cause
	}
	// A physical deadline may wake its reader before the timer callback runs.
	if !time.Now().Before(owner.end) {
		return context.DeadlineExceeded
	}
	return fallback
}
