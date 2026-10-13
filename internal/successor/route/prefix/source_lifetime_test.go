package prefix

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network"
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
