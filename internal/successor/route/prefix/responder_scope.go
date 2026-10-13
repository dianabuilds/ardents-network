package prefix

import (
	"context"
	"errors"
	"net"
	"reflect"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network"
	"github.com/dianabuilds/ardents-network/internal/successor/route"
)

// ResponderScope retains one original Source, selected Responder leg, caller
// and bound across joined physical generations. It grants no JOIN or Service
// authority. Each acquisition still opens and checks a genuine physical Prefix.
type ResponderScope struct {
	self                   *ResponderScope
	source                 *SourceScope
	ctx                    context.Context
	cancel                 context.CancelFunc
	config                 Config
	prepare                func(context.Context) (Config, error)
	turn                   chan struct{}
	mu                     sync.Mutex
	closed, initialUsed    bool
	current                *responderGeneration
	generations            []*responderGeneration
	failure, cleanup, work error
	once                   sync.Once
	closeErr               error
	openingCancel          context.CancelFunc // mu; only untransferred native opening.
	joined                 chan struct{}
}

type responderGeneration struct {
	owner         *ResponderScope
	source        *SourceHandle
	physical      *Prefix
	cancel        context.CancelFunc
	borrow        *Borrow
	sealed        bool // owner.mu; independent of the logical scope.
	work, cleanup error
	joined        bool
}

// ResponderHandle names one exact physical generation. It is never rebound.
// BorrowJoin transfers its one JOIN claim; closing the setup handle afterwards
// cannot close a transferred JOIN or the scope's shared physical generation.
type ResponderHandle struct {
	self             *ResponderHandle
	owner            *ResponderScope
	generation       *responderGeneration
	closed, consumed bool // owner.mu
}

// NewResponderScope consumes the prepared initial reservation only on success.
// prepare obtains a fresh genuine reservation/preparation for the SAME leg;
// its returned facts cannot change the original observer, presenter or bounds.
func NewResponderScope(ctx context.Context, source *SourceScope, config Config, prepare func(context.Context) (Config, error)) (*ResponderScope, error) {
	if ctx == nil || source == nil || source.self != source || config.Current == nil || config.Present == nil || config.Release == nil || prepare == nil ||
		config.Leg.EntryMember.RoleDomain != 3 || config.Leg.InteriorMember.RoleDomain != 3 || config.Deadline.IsZero() || !time.Now().Before(config.Deadline) {
		return nil, errors.New("retained Responder scope unavailable")
	}
	if end, ok := ctx.Deadline(); !ok || end.After(config.Deadline) {
		return nil, errors.New("responder scope exceeds original bound")
	}
	child, cancel := context.WithCancel(ctx)
	r := &ResponderScope{source: source, ctx: child, cancel: cancel, config: config, prepare: prepare, turn: make(chan struct{}, 1), joined: make(chan struct{})}
	r.self = r
	if err := r.Check(); err != nil {
		cancel()
		return nil, err
	}
	return r, nil
}

func (r *ResponderScope) localLocked() error {
	if r.self != r || r.closed {
		return net.ErrClosed
	}
	if !time.Now().Before(r.config.Deadline) {
		return context.DeadlineExceeded
	}
	return errors.Join(r.failure, r.ctx.Err())
}

// Check observes original authority, not the readiness of an ended generation.
// Physical currentness is required separately by the exact handle/JoinBorrow.
func (r *ResponderScope) Check() error {
	if r == nil || r.self != r || r.source == nil || r.ctx == nil {
		return errors.New("retained Responder scope absent")
	}
	r.mu.Lock()
	err := r.localLocked()
	r.mu.Unlock()
	if err != nil {
		return err
	}
	view, err := r.config.Current()
	if err != nil {
		return err
	}
	if err := errors.Join(r.source.Check(), r.config.Leg.Check(view, view.ObservedAt())); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.localLocked()
}

func (r *ResponderScope) CheckSource(source *SourceScope, end time.Time) error {
	if r == nil || r.self != r || r.source != source || end.IsZero() || end.After(r.config.Deadline) || end.After(r.config.Leg.NotAfter) {
		return errors.New("responder scope original Source or bound differs")
	}
	return r.Check()
}

// Commit holds original Source before scope state. The action is local only;
// fresh observations must run before this barrier, never from its callback.
func (r *ResponderScope) Commit(action func(func() error) error) error {
	if r == nil || r.self != r || action == nil {
		return errors.New("responder scope handoff absent")
	}
	return r.source.Commit(func(checkSource func() error) error {
		r.mu.Lock()
		defer r.mu.Unlock()
		check := func() error { return errors.Join(checkSource(), r.localLocked()) }
		if err := check(); err != nil {
			return err
		}
		return action(check)
	})
}

// Acquire joins a failed generation before obtaining any fresh preparation.
// Its caller supplies the existing setup/proposal bound; no retry is performed.
func (r *ResponderScope) Acquire(ctx context.Context) (_ *ResponderHandle, result error) {
	if r == nil || r.self != r || ctx == nil {
		return nil, errors.New("responder acquisition absent")
	}
	select {
	case r.turn <- struct{}{}:
		defer func() { <-r.turn }()
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-r.ctx.Done():
		return nil, r.ctx.Err()
	}
	if err := errors.Join(ctx.Err(), r.Check()); err != nil {
		return nil, err
	}
	r.mu.Lock()
	g := r.current
	r.mu.Unlock()
	if g != nil {
		if err := g.physical.Current(); err != nil {
			r.retire(g)
			if err := errors.Join(ctx.Err(), r.Check()); err != nil {
				return nil, errors.Join(err, r.RetiredWork())
			}
			g = nil
		}
	}
	if g == nil {
		source, err := r.source.Acquire(ctx)
		if err != nil {
			return nil, err
		}
		transferredSource := false
		defer func() {
			if !transferredSource {
				source.Close()
			}
		}()
		config := r.config
		if r.initialUsed {
			var err error
			config, err = r.prepare(ctx)
			if err != nil {
				if config.Release != nil {
					cleanup := config.Release()
					r.retainCleanup(cleanup)
					err = errors.Join(err, cleanup)
				}
				return nil, err
			}
			if config.Release == nil || !config.Deadline.Equal(r.config.Deadline) || !reflect.DeepEqual(config.Leg, r.config.Leg) {
				var cleanup error
				if config.Release != nil {
					cleanup = config.Release()
				}
				r.retainCleanup(cleanup)
				failure := errors.New("prepared Responder changed original leg or bound")
				r.mu.Lock()
				r.failure = errors.Join(r.failure, failure)
				r.mu.Unlock()
				return nil, errors.Join(failure, cleanup)
			}
			// New physical reservation/refill, original authentic observer and
			// finite Stock presenter. A factory never grants accepting authority.
			config.Current, config.Present = r.config.Current, r.config.Present
		} else {
			r.initialUsed = true
		}
		var releaseOnce sync.Once
		var releaseErr error
		release := config.Release
		config.Release = func() error { releaseOnce.Do(func() { releaseErr = release() }); return releaseErr }
		caller, cancel := context.WithCancel(r.ctx)
		r.mu.Lock()
		if err := r.localLocked(); err != nil {
			r.mu.Unlock()
			cancel()
			cleanup := config.Release()
			r.retainCleanup(cleanup)
			return nil, errors.Join(err, cleanup)
		}
		r.openingCancel = cancel
		r.mu.Unlock()
		interrupted := make(chan struct{})
		stop := context.AfterFunc(ctx, func() { defer close(interrupted); cancel() })
		physical, err := OpenResponder(caller, source.generation.physical, config)
		if !stop() {
			<-interrupted
		}
		r.mu.Lock()
		r.openingCancel = nil
		r.mu.Unlock()
		err = errors.Join(err, ctx.Err(), caller.Err())
		g = &responderGeneration{owner: r, source: source, physical: physical, cancel: cancel}
		if err == nil {
			// This quiet claim owns no terminal actor. Every actual JOIN has
			// its own Prefix borrow, interruption and joined physical result.
			g.borrow, err = physical.BorrowLifetime(r.ctx, func() { r.sealGeneration(g) }, func() error { return nil })
		}
		if err != nil {
			cancel()
			if physical != nil {
				r.retire(g)
			} else {
				cleanup := config.Release()
				r.mu.Lock()
				r.cleanup = errors.Join(r.cleanup, cleanup)
				r.failure = errors.Join(r.failure, cleanup)
				r.mu.Unlock()
			}
			return nil, errors.Join(err, g.work, g.cleanup)
		}
		if err := errors.Join(ctx.Err(), r.Check()); err != nil {
			r.retire(g)
			return nil, errors.Join(err, g.work, g.cleanup)
		}
		var committed bool
		err = source.commitResponder(physical, func(checkOriginal func() error) error {
			r.mu.Lock()
			defer r.mu.Unlock()
			if err := errors.Join(ctx.Err(), r.localLocked(), checkOriginal()); err != nil {
				return err
			}
			if r.current != nil || g.sealed {
				return errors.New("responder generation changed before handoff")
			}
			r.current = g
			r.generations = append(r.generations, g)
			committed = true
			return nil
		})
		if !committed {
			r.retire(g)
			return nil, errors.Join(err, g.work, g.cleanup)
		}
		transferredSource = true
	}
	h := &ResponderHandle{owner: r, generation: g}
	h.self = h
	if err := g.source.commitResponder(g.physical, func(checkOriginal func() error) error {
		r.mu.Lock()
		defer r.mu.Unlock()
		return errors.Join(ctx.Err(), r.handleLocalLocked(h), checkOriginal())
	}); err != nil {
		return nil, err
	}
	return h, nil
}

func (r *ResponderScope) sealGeneration(g *responderGeneration) {
	r.mu.Lock()
	g.sealed = true
	r.mu.Unlock()
}

// The physical turn serializes retirement/opening. No state lock holds I/O.
func (r *ResponderScope) retire(g *responderGeneration) {
	if g == nil || g.joined {
		return
	}
	r.sealGeneration(g)
	if g.physical != nil {
		g.physical.Seal()
		g.borrow.ReturnJoined()
		g.work = g.physical.Close()
		var complete bool
		g.cleanup, complete = g.physical.CleanupResult()
		if !complete {
			g.cleanup = errors.Join(g.cleanup, errors.New("responder generation cleanup did not join"))
		}
	}
	if g.cancel != nil {
		g.cancel()
	}
	if g.source != nil {
		g.source.Close()
	}
	g.joined = true
	r.mu.Lock()
	if r.current == g {
		r.current = nil
	}
	r.work = errors.Join(r.work, g.work)
	r.cleanup = errors.Join(r.cleanup, g.cleanup)
	r.failure = errors.Join(r.failure, g.cleanup)
	r.mu.Unlock()
}

func (r *ResponderScope) RetiredWork() error { r.mu.Lock(); defer r.mu.Unlock(); return r.work }

func (r *ResponderScope) retainCleanup(cleanup error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cleanup = errors.Join(r.cleanup, cleanup)
	r.failure = errors.Join(r.failure, cleanup)
}

func (r *ResponderScope) CleanupResult() (error, bool) {
	if r == nil || r.self != r || r.joined == nil {
		return nil, false
	}
	select {
	case <-r.joined:
		return r.cleanup, true
	default:
		return nil, false
	}
}

// Close denies all effects before waiting for the original physical turn.
// Generation work remains with its attempt; all physical failures are retained.
func (r *ResponderScope) Close() error {
	if r == nil {
		return nil
	}
	if r.self != r {
		return errors.New("copied Responder scope")
	}
	r.once.Do(func() {
		r.mu.Lock()
		r.closed = true
		opening := r.openingCancel
		r.mu.Unlock()
		if opening != nil {
			opening()
		}
		r.turn <- struct{}{}
		defer func() { <-r.turn }()
		r.mu.Lock()
		current := r.current
		r.mu.Unlock()
		for _, g := range r.generations {
			r.retire(g)
		}
		if !r.initialUsed {
			r.retainCleanup(r.config.Release())
		}
		r.cancel()
		var finalWork error
		if current != nil {
			finalWork = current.work
		}
		r.mu.Lock()
		r.closeErr = errors.Join(finalWork, r.failure, r.cleanup)
		r.mu.Unlock()
		close(r.joined)
	})
	return r.closeErr
}

func (r *ResponderScope) handleLocalLocked(h *ResponderHandle) error {
	if h == nil || h.self != h || h.owner != r || h.closed || h.generation == nil || h.generation.owner != r || r.current != h.generation || h.generation.sealed {
		return errors.New("original Responder acquisition retired")
	}
	return r.localLocked()
}

func (h *ResponderHandle) Check() error {
	if h == nil || h.self != h || h.owner == nil {
		return errors.New("responder handle absent")
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

func (h *ResponderHandle) Close() {
	if h == nil || h.self != h || h.owner == nil {
		return
	}
	h.owner.mu.Lock()
	h.closed = true
	h.owner.mu.Unlock()
}

func (h *ResponderHandle) CheckRendezvous(duty network.RetainedDuty, excluded []route.Member) error {
	if err := h.Check(); err != nil {
		return err
	}
	return h.generation.source.generation.physical.CheckResponderRendezvous(h.generation.physical, duty, excluded)
}

func (h *ResponderHandle) Commit(action func(func() error) error) error {
	if h == nil || h.self != h || action == nil {
		return errors.New("responder handle handoff absent")
	}
	return h.generation.source.commitResponder(h.generation.physical, func(checkOriginal func() error) error {
		h.owner.mu.Lock()
		defer h.owner.mu.Unlock()
		check := func() error { return errors.Join(checkOriginal(), h.owner.handleLocalLocked(h)) }
		if err := check(); err != nil {
			return err
		}
		return action(check)
	})
}
