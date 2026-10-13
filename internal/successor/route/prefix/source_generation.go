package prefix

import (
	"context"
	"errors"
	"reflect"
	"sync"
)

type sourceGeneration struct {
	owner         *SourceScope
	physical      *Prefix
	borrow        *Borrow
	cancel        context.CancelFunc
	sealed        bool // owner.mu
	joined        bool // owner's physical turn
	work, cleanup error
}

// SourceHandle retains one exact original physical generation. Closing the
// setup handle does not interrupt the actual JOIN claim transferred from it.
type SourceHandle struct {
	self             *SourceHandle
	owner            *SourceScope
	generation       *sourceGeneration
	closed, consumed bool // owner.mu
}

func (scope *SourceScope) sealGeneration(g *sourceGeneration) {
	scope.mu.Lock()
	g.sealed = true
	scope.mu.Unlock()
}

func (scope *SourceScope) retireGeneration(g *sourceGeneration) {
	if g == nil || g.joined {
		return
	}
	scope.sealGeneration(g)
	g.physical.Seal()
	g.borrow.ReturnJoined()
	g.work = g.physical.Close()
	var joined bool
	g.cleanup, joined = g.physical.CleanupResult()
	if !joined {
		g.cleanup = errors.Join(g.cleanup, errors.New("source generation did not physically join"))
	}
	if g.cancel != nil {
		g.cancel()
	}
	g.joined = true
	scope.mu.Lock()
	if scope.current == g {
		scope.current = nil
	}
	scope.retiredWork = errors.Join(scope.retiredWork, g.work)
	scope.cleanup = errors.Join(scope.cleanup, g.cleanup)
	scope.failure = errors.Join(scope.failure, g.cleanup)
	scope.mu.Unlock()
}

// Acquire joins the ended generation before requesting fresh real resources.
// It performs one opening under its caller's existing bound; it never retries,
// redraws the original leg or extends the original logical lifetime.
func (scope *SourceScope) Acquire(ctx context.Context) (*SourceHandle, error) {
	if scope == nil || scope.self != scope || ctx == nil {
		return nil, errors.New("source acquisition absent")
	}
	select {
	case scope.turn <- struct{}{}:
		defer func() { <-scope.turn }()
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-scope.ctx.Done():
		return nil, scope.ctx.Err()
	}
	if err := errors.Join(ctx.Err(), scope.Check()); err != nil {
		return nil, err
	}
	scope.mu.Lock()
	g := scope.current
	scope.mu.Unlock()
	if g != nil && g.physical.Current() != nil {
		scope.retireGeneration(g)
		g = nil
	}
	if err := errors.Join(ctx.Err(), scope.Check()); err != nil {
		return nil, err
	}
	if g == nil {
		if scope.prepare == nil {
			return nil, errors.New("source physical preparation unavailable")
		}
		caller, cancel := context.WithCancel(scope.ctx)
		scope.mu.Lock()
		if err := scope.localLocked(); err != nil {
			scope.mu.Unlock()
			cancel()
			return nil, err
		}
		scope.openingCancel = cancel
		scope.mu.Unlock()
		interrupted := make(chan struct{})
		stop := context.AfterFunc(ctx, func() { defer close(interrupted); cancel() })
		defer func() {
			if !stop() {
				<-interrupted
			}
			scope.mu.Lock()
			scope.openingCancel = nil
			scope.mu.Unlock()
		}()
		config, err := scope.prepare(caller)
		if err == nil && (config.Release == nil || config.Present == nil || !config.Deadline.Equal(scope.config.Deadline) || !reflect.DeepEqual(config.Leg, scope.config.Leg)) {
			err = errors.New("prepared Source changed original binding or bounds")
			scope.mu.Lock()
			scope.failure = errors.Join(scope.failure, err)
			scope.mu.Unlock()
		}
		err = errors.Join(err, ctx.Err(), caller.Err())
		var releaseOnce sync.Once
		var releaseErr error
		originalRelease := config.Release
		if originalRelease != nil {
			config.Release = func() error { releaseOnce.Do(func() { releaseErr = originalRelease() }); return releaseErr }
		}
		if err != nil {
			cancel()
			var cleanup error
			if config.Release != nil {
				cleanup = config.Release()
			}
			scope.mu.Lock()
			scope.cleanup = errors.Join(scope.cleanup, cleanup)
			scope.failure = errors.Join(scope.failure, cleanup)
			scope.mu.Unlock()
			return nil, errors.Join(err, cleanup)
		}
		config.Current = scope.config.Current
		physical, err := Open(caller, config)
		if !stop() {
			<-interrupted
		}
		// The cancellation callback has joined before physical publication.
		stop = func() bool { return true }
		err = errors.Join(err, ctx.Err(), caller.Err())
		g = &sourceGeneration{owner: scope, physical: physical, cancel: cancel}
		if err == nil {
			g.borrow, err = physical.BorrowLifetime(scope.ctx, func() { scope.sealGeneration(g) }, func() error { return nil })
		}
		if err == nil {
			err = scope.Check()
		}
		if err == nil {
			err = physical.CommitPair(nil, func(checkPhysical func() error) error {
				scope.mu.Lock()
				defer scope.mu.Unlock()
				if err := errors.Join(ctx.Err(), scope.localLocked(), checkPhysical()); err != nil {
					return err
				}
				if scope.current != nil || g.sealed {
					return errors.New("source generation changed before publication")
				}
				scope.current = g
				scope.generations = append(scope.generations, g)
				return nil
			})
		}
		if err != nil {
			cancel()
			if physical != nil {
				scope.retireGeneration(g)
			} else {
				cleanup := config.Release()
				scope.mu.Lock()
				scope.cleanup = errors.Join(scope.cleanup, cleanup)
				scope.failure = errors.Join(scope.failure, cleanup)
				scope.mu.Unlock()
			}
			return nil, errors.Join(err, g.work, g.cleanup)
		}
	}
	handle := &SourceHandle{owner: scope, generation: g}
	handle.self = handle
	if err := errors.Join(ctx.Err(), handle.Check()); err != nil {
		return nil, err
	}
	return handle, nil
}

func (scope *SourceScope) handleLocalLocked(h *SourceHandle) error {
	if h == nil || h.self != h || h.owner != scope || h.closed || h.generation == nil || h.generation.owner != scope || scope.current != h.generation || h.generation.sealed {
		return errors.New("original Source handle retired")
	}
	return scope.localLocked()
}

func (h *SourceHandle) Check() error {
	if h == nil || h.self != h || h.owner == nil {
		return errors.New("source handle absent")
	}
	if err := h.owner.Check(); err != nil {
		return err
	}
	if err := h.generation.physical.Current(); err != nil {
		return err
	}
	h.owner.mu.Lock()
	defer h.owner.mu.Unlock()
	return h.owner.handleLocalLocked(h)
}

// CurrentHandle observes the exact current generation only. It cannot open or
// replace a Source while a dependent Responder's presentation is in flight.
func (scope *SourceScope) CurrentHandle(ctx context.Context) (*SourceHandle, error) {
	if scope == nil || scope.self != scope || ctx == nil {
		return nil, errors.New("current Source acquisition absent")
	}
	scope.mu.Lock()
	g := scope.current
	scope.mu.Unlock()
	if g == nil {
		return nil, errors.New("current Source physical generation absent")
	}
	h := &SourceHandle{owner: scope, generation: g}
	h.self = h
	if err := errors.Join(ctx.Err(), h.Check()); err != nil {
		return nil, err
	}
	return h, nil
}

func (h *SourceHandle) commitResponder(responder *Prefix, action func(func() error) error) error {
	if h == nil || h.self != h || responder == nil || responder.source != h.generation.physical || action == nil {
		return errors.New("source-bound Responder commit absent")
	}
	return h.generation.physical.CommitPair(responder, func(checkPhysical func() error) error {
		h.owner.mu.Lock()
		defer h.owner.mu.Unlock()
		check := func() error { return errors.Join(checkPhysical(), h.owner.handleLocalLocked(h)) }
		if err := check(); err != nil {
			return err
		}
		return action(check)
	})
}

func (h *SourceHandle) Commit(action func(func() error) error) error {
	if h == nil || h.self != h || action == nil {
		return errors.New("source handle commit absent")
	}
	return h.generation.physical.CommitPair(nil, func(checkPhysical func() error) error {
		h.owner.mu.Lock()
		defer h.owner.mu.Unlock()
		check := func() error { return errors.Join(checkPhysical(), h.owner.handleLocalLocked(h)) }
		if err := check(); err != nil {
			return err
		}
		return action(check)
	})
}

func (h *SourceHandle) Close() {
	if h == nil || h.self != h || h.owner == nil {
		return
	}
	h.owner.mu.Lock()
	h.closed = true
	h.owner.mu.Unlock()
}
