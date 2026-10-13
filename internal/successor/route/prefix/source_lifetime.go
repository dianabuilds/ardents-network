package prefix

import (
	"context"
	"errors"
	"net"
	"reflect"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network"
)

// SourceScope retains the original caller, authenticated selected Source leg
// and absolute bound independently of a physical Prefix's retirement. It owns
// the transferred physical Prefix, not selection, permission or a new caller.
// Physical operations still need their exact Prefix acquisition and checks.
type SourceScope struct {
	self          *SourceScope
	caller        context.Context
	ctx           context.Context
	cancel        context.CancelFunc
	config        Config
	mu            sync.Mutex
	closed        bool
	once          sync.Once
	joined        chan struct{}
	work          error
	retiredWork   error // mu; joined generation outcomes, separate from accepting state.
	cleanup       error
	closeErr      error
	prepare       func(context.Context) (Config, error)
	turn          chan struct{}
	current       *sourceGeneration
	generations   []*sourceGeneration
	failure       error
	openingCancel context.CancelFunc
}

// NewSourceScope takes ownership only on success. Detached facts or a retired,
// foreign-caller, bootstrap or non-Source Prefix cannot construct this scope.
func NewSourceScope(caller context.Context, physical *Prefix, prepare func(context.Context) (Config, error)) (*SourceScope, error) {
	if caller == nil || physical == nil || physical.self != physical || physical.bootstrap ||
		!reflect.TypeOf(caller).Comparable() || physical.caller != caller ||
		physical.config.Leg.EntryMember.RoleDomain != 1 || physical.config.Leg.InteriorMember.RoleDomain != 1 {
		return nil, errors.New("original Source lifetime unavailable")
	}
	if err := physical.Current(); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithDeadline(caller, physical.config.Deadline)
	scope := &SourceScope{caller: caller, ctx: ctx, cancel: cancel, config: physical.config,
		joined: make(chan struct{}), prepare: prepare, turn: make(chan struct{}, 1)}
	scope.self = scope
	generation := &sourceGeneration{owner: scope, physical: physical}
	scope.current, scope.generations = generation, []*sourceGeneration{generation}
	if err := scope.Check(); err != nil {
		cancel()
		return nil, err
	}
	// This quiet claim owns no record actor. Every exact JOIN/opening keeps
	// its existing actual Prefix claim and physical retirement callback.
	borrow, err := physical.BorrowLifetime(ctx, func() { scope.sealGeneration(generation) }, func() error { return nil })
	if err != nil {
		cancel()
		return nil, err
	}
	generation.borrow = borrow
	if err := physical.CommitPair(nil, func(checkOriginal func() error) error {
		return errors.Join(checkOriginal(), ctx.Err())
	}); err != nil {
		borrow.ReturnJoined()
		cancel()
		return nil, err
	}
	return scope, nil
}

func (scope *SourceScope) localLocked() error {
	if scope.self != scope || scope.closed {
		return net.ErrClosed
	}
	if !time.Now().Before(scope.config.Deadline) {
		return context.DeadlineExceeded
	}
	return errors.Join(scope.failure, scope.ctx.Err())
}

// Check reobserves authentic original authority without consulting readiness
// of a retired physical generation. This grants no transport opening or JOIN.
func (scope *SourceScope) Check() error {
	_, err := scope.observeOriginal()
	return err
}

func (scope *SourceScope) observeOriginal() (network.RuntimeView, error) {
	if scope == nil || scope.self != scope || scope.ctx == nil || scope.config.Current == nil {
		return network.RuntimeView{}, errors.New("original Source scope absent")
	}
	scope.mu.Lock()
	err := scope.localLocked()
	scope.mu.Unlock()
	if err != nil {
		return network.RuntimeView{}, err
	}
	view, err := scope.config.Current()
	if err != nil {
		return network.RuntimeView{}, err
	}
	if err := scope.config.Leg.Check(view, view.ObservedAt()); err != nil {
		return network.RuntimeView{}, err
	}
	scope.mu.Lock()
	defer scope.mu.Unlock()
	return view, scope.localLocked()
}

func (scope *SourceScope) CheckSourceCaller(caller context.Context, network, profile [32]byte, end time.Time) error {
	if scope == nil || scope.self != scope || caller == nil || !reflect.TypeOf(caller).Comparable() || scope.caller != caller ||
		network == [32]byte{} || profile == [32]byte{} || scope.config.Leg.Profile.Network != network || scope.config.Leg.Profile.Digest != profile ||
		end.IsZero() || end != end.UTC().Truncate(time.Second) || !time.Now().Before(end) ||
		end.After(scope.config.Deadline) || end.After(scope.config.Leg.NotAfter) || end.After(scope.config.Leg.Profile.NotAfter) {
		return errors.New("original Source scope caller or bounds differ")
	}
	return scope.Check()
}

// Commit admits only a bounded local transition after the caller's fresh
// observations. It never waits for foreign physical I/O under scope state.
func (scope *SourceScope) Commit(action func(func() error) error) error {
	if scope == nil || scope.self != scope || action == nil {
		return errors.New("original Source scope handoff absent")
	}
	scope.mu.Lock()
	defer scope.mu.Unlock()
	check := scope.localLocked
	if err := check(); err != nil {
		return err
	}
	return action(check)
}

// Close seals logical effects before joining physical users. The original
// caller remains retained during ordinary physical retirement; its own revoke
// still interrupts the exact Prefix through that Prefix's original context.
func (scope *SourceScope) Close() error {
	if scope == nil {
		return nil
	}
	if scope.self != scope {
		return errors.New("copied Source scope")
	}
	scope.once.Do(func() {
		scope.mu.Lock()
		scope.closed = true
		opening := scope.openingCancel
		scope.mu.Unlock()
		if opening != nil {
			opening()
		}
		scope.turn <- struct{}{}
		defer func() { <-scope.turn }()
		current := scope.current
		for _, generation := range scope.generations {
			scope.retireGeneration(generation)
		}
		if current != nil {
			scope.work = current.work
		}
		scope.cancel()
		scope.closeErr = errors.Join(scope.work, scope.failure, scope.cleanup)
		close(scope.joined)
	})
	return scope.closeErr
}

func (scope *SourceScope) CleanupResult() (error, bool) {
	if scope == nil || scope.self != scope || scope.joined == nil {
		return nil, false
	}
	select {
	case <-scope.joined:
		return scope.cleanup, true
	default:
		return nil, false
	}
}

// RetiredWork retains original joined generation outcomes when a later
// acquisition refuses before returning a handle. It grants no live Source.
func (scope *SourceScope) RetiredWork() error {
	if scope == nil || scope.self != scope {
		return errors.New("original Source scope absent")
	}
	scope.mu.Lock()
	defer scope.mu.Unlock()
	return scope.retiredWork
}
