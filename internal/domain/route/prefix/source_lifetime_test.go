package prefix

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/network"
)

// A retirement-only actual pipe supplies physical scheduling, never successful
// Network, Source opening, Job, Stock, JOIN or Service authority.
func TestSourceScopeCloseSealsBeforeOriginalPhysicalJoin(t *testing.T) {
	gate := make(chan struct{})
	late := errors.New("original Source late physical close failure")
	physical, _, _, returned, cleanup := idlePhysicalPrefix(t, gate, late, true)
	var release sync.Once
	unblock := func() { release.Do(func() { close(gate) }) }
	t.Cleanup(func() { unblock(); cleanup() })
	ctx, cancel := context.WithDeadline(t.Context(), physical.config.Deadline)
	var observed atomic.Int32
	scope := &SourceScope{caller: t.Context(), ctx: ctx, cancel: cancel,
		joined: make(chan struct{}), turn: make(chan struct{}, 1), config: Config{Deadline: physical.config.Deadline, Current: func() (network.RuntimeView, error) {
			observed.Add(1)
			return network.RuntimeView{}, errors.New("original authority unavailable")
		}}}
	scope.self = scope
	borrow, err := physical.BorrowLifetime(ctx, func() {}, func() error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	generation := &sourceGeneration{owner: scope, physical: physical, borrow: borrow}
	scope.current, scope.generations = generation, []*sourceGeneration{generation}
	closed := make(chan error, 1)
	go func() { closed <- scope.Close() }()
	deadline := time.Now().Add(time.Second)
	for {
		scope.mu.Lock()
		sealed := scope.closed
		scope.mu.Unlock()
		if sealed {
			break
		}
		if !time.Now().Before(deadline) {
			t.Fatal("Source scope did not seal before join")
		}
		time.Sleep(time.Millisecond)
	}
	if err := scope.Check(); err == nil || observed.Load() != 0 {
		t.Fatal("sealed Source scope observed or accepted effects", err)
	}
	if _, complete := scope.CleanupResult(); complete || returned.Load() != 0 || ctx.Err() != nil {
		t.Fatal("Source published completion, returned resources or canceled physical lifetime before join")
	}
	select {
	case <-closed:
		t.Fatal("Source scope forgot original reader")
	default:
	}
	unblock()
	result := <-closed
	if !errors.Is(result, late) || scope.Close() != result || returned.Load() != 1 {
		t.Fatal("Source lost retained late failure or once-only return", result)
	}
	if physical, complete := scope.CleanupResult(); !complete || !errors.Is(physical, late) || ctx.Err() == nil {
		t.Fatal("Source joined receipt or original cancellation missing", physical, complete)
	}
	if !errors.Is(scope.RetiredWork(), late) {
		t.Fatal("retired Source lost the original joined outcome after clearing its generation")
	}
}

// Close shares its accepting state with concurrent observers, acquisitions
// and repeated Close callers, and a dependent Responder retirement closes its
// exact Source handle through the same mu. The physical turn serializes
// retirement; no state lock holds I/O, and Close never calls a Responder, so
// the Source-before-Responder order stays intact under contention.
func TestSourceScopeCloseConcurrentObserversAndResponder(t *testing.T) {
	gate := make(chan struct{})
	late := errors.New("original Source concurrent close failure")
	physical, _, _, returned, cleanup := idlePhysicalPrefix(t, gate, late, true)
	var release sync.Once
	unblock := func() { release.Do(func() { close(gate) }) }
	t.Cleanup(func() { unblock(); cleanup() })
	ctx, cancel := context.WithDeadline(t.Context(), physical.config.Deadline)
	scope := &SourceScope{caller: t.Context(), ctx: ctx, cancel: cancel,
		joined: make(chan struct{}), turn: make(chan struct{}, 1), config: Config{Deadline: physical.config.Deadline, Current: func() (network.RuntimeView, error) {
			return network.RuntimeView{}, errors.New("original authority unavailable")
		}}}
	scope.self = scope
	borrow, err := physical.BorrowLifetime(ctx, func() {}, func() error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	generation := &sourceGeneration{owner: scope, physical: physical, borrow: borrow}
	scope.current, scope.generations = generation, []*sourceGeneration{generation}
	handle := &SourceHandle{owner: scope, generation: generation}
	handle.self = handle

	// A retirement-only dependent Responder closes this exact Source handle
	// through owner.mu while the scope itself is closing. It supplies no
	// physical generation, role, token or Service authority.
	rctx, rcancel := context.WithDeadline(t.Context(), physical.config.Deadline)
	defer rcancel()
	r := &ResponderScope{ctx: rctx, cancel: rcancel, source: scope, initialUsed: true,
		turn: make(chan struct{}, 1), joined: make(chan struct{}), config: scope.config}
	r.self = r
	rg := &responderGeneration{owner: r, source: handle}
	r.current, r.generations = rg, []*responderGeneration{rg}

	stop := make(chan struct{})
	var observers sync.WaitGroup
	for range 4 {
		observers.Go(func() {
			for {
				select {
				case <-stop:
					return
				default:
				}
				_ = scope.Check()
				_ = scope.RetiredWork()
				_, _ = scope.CleanupResult()
				_ = handle.Check()
				_, _ = scope.CurrentHandle(t.Context())
				_ = scope.Commit(func(func() error) error { return nil })
			}
		})
	}
	const closers = 8
	results := make(chan error, closers)
	var closed sync.WaitGroup
	for range closers {
		closed.Go(func() { results <- scope.Close() })
	}
	acquired := make(chan error, 1)
	go func() { _, err := scope.Acquire(t.Context()); acquired <- err }()
	responderClosed := make(chan error, 1)
	go func() { responderClosed <- r.Close() }()
	// Let observers, acquisitions and Close callers contend while the
	// original physical join is still gated.
	time.Sleep(5 * time.Millisecond)
	unblock()
	closed.Wait()
	close(stop)
	observers.Wait()
	if err := <-acquired; err == nil {
		t.Fatal("concurrent acquisition accepted a closing Source scope")
	}
	if err := <-responderClosed; err != nil {
		t.Fatal("dependent Responder retirement lost its joined outcome", err)
	}
	var first error
	for range closers {
		result := <-results
		if first == nil {
			first = result
		}
		if result != first || !errors.Is(result, late) {
			t.Fatal("concurrent Close diverged or lost the retained failure", result, first)
		}
	}
	if returned.Load() != 1 {
		t.Fatal("concurrent Close returned physical resources more than once", returned.Load())
	}
	if result, complete := scope.CleanupResult(); !complete || !errors.Is(result, late) {
		t.Fatal("concurrent Close lost the joined cleanup receipt", result, complete)
	}
	if !errors.Is(scope.RetiredWork(), late) {
		t.Fatal("concurrent Close lost the retired generation outcome")
	}
	if err := handle.Check(); err == nil {
		t.Fatal("closed Source scope still accepted its original handle")
	}
	if err := scope.Close(); err != first {
		t.Fatal("repeated Close after contention changed the retained result", err)
	}
}

func TestSourceScopeCopyCannotRetireOriginal(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	original := &SourceScope{ctx: ctx, cancel: cancel}
	original.self = original
	copyScope := &SourceScope{self: original, ctx: ctx, cancel: cancel}
	if err := copyScope.Close(); err == nil || ctx.Err() != nil {
		t.Fatal("copied Source scope retired original", err)
	}
	if err := copyScope.Commit(func(func() error) error { t.Fatal("copied scope admitted action"); return nil }); err == nil {
		t.Fatal("copied Source scope admitted handoff")
	}
	handle := &SourceHandle{owner: original}
	handle.self = handle
	copyHandle := &SourceHandle{self: handle, owner: original}
	copyHandle.Close()
	if handle.closed {
		t.Fatal("copied Source handle sealed original")
	}
	if _, err := copyHandle.BorrowJoin(ctx, func() {}, func() error { return nil }); err == nil {
		t.Fatal("copied Source handle acquired JOIN")
	}
}
