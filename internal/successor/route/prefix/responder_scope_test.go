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

// This is a retirement-only scope over the existing actual pipe generations.
// Its observer always refuses: it supplies no successful role, token, JOIN,
// Publication or worker authority. Genuine acquire/reopen stays in composition.
func TestResponderScopeCloseDeniesBeforeHeldPhysicalJoin(t *testing.T) {
	gate := make(chan struct{})
	late := errors.New("original physical responder close failure")
	physical, _, _, returned, cleanup := idlePhysicalPrefix(t, gate, late, true)
	var release sync.Once
	unblock := func() { release.Do(func() { close(gate) }) }
	t.Cleanup(func() { unblock(); cleanup() })
	ctx, cancel := context.WithDeadline(t.Context(), physical.config.Deadline)
	var observed atomic.Int32
	r := &ResponderScope{ctx: ctx, cancel: cancel, source: &SourceScope{}, initialUsed: true, turn: make(chan struct{}, 1), joined: make(chan struct{}), config: Config{Deadline: physical.config.Deadline,
		Current: func() (network.RuntimeView, error) {
			observed.Add(1)
			return network.RuntimeView{}, errors.New("authority unavailable")
		}}}
	r.self = r
	g := &responderGeneration{owner: r, physical: physical}
	r.current, r.generations = g, []*responderGeneration{g}
	var err error
	g.borrow, err = physical.BorrowLifetime(ctx, func() { r.sealGeneration(g) }, func() error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	h := &ResponderHandle{owner: r, generation: g}
	h.self = h
	closed := make(chan error, 1)
	go func() { closed <- r.Close() }()
	deadline := time.Now().Add(time.Second)
	for {
		r.mu.Lock()
		sealed := r.closed
		r.mu.Unlock()
		if sealed {
			break
		}
		if !time.Now().Before(deadline) {
			t.Fatal("scope did not seal before join")
		}
		time.Sleep(time.Millisecond)
	}
	if err := h.Check(); err == nil || observed.Load() != 0 {
		t.Fatal("sealed acquisition observed or accepted new authority", err)
	}
	if _, complete := r.CleanupResult(); complete || returned.Load() != 0 {
		t.Fatal("scope released before actual parent join")
	}
	select {
	case <-closed:
		t.Fatal("scope Close did not retain its original reader")
	default:
	}
	unblock()
	if err := <-closed; !errors.Is(err, late) {
		t.Fatal("original joined failure lost", err)
	}
	if result, complete := r.CleanupResult(); !complete || !errors.Is(result, late) {
		t.Fatal("scope physical receipt incomplete", result, complete)
	}
	if err := r.Close(); !errors.Is(err, late) || returned.Load() != 1 {
		t.Fatal("repeated Close changed retained result or return", err, returned.Load())
	}
}

func TestResponderScopeCopiesCannotRetireOriginal(t *testing.T) {
	ctx, cancel := context.WithDeadline(t.Context(), time.Now().Add(time.Second))
	defer cancel()
	r := &ResponderScope{ctx: ctx, cancel: cancel, turn: make(chan struct{}, 1), joined: make(chan struct{}), config: Config{Deadline: time.Now().Add(time.Second)}}
	r.self = r
	// Copy only immutable provenance, not mutexes or physical resource state.
	copyScope := &ResponderScope{self: r, ctx: ctx, cancel: cancel}
	if err := copyScope.Close(); err == nil || ctx.Err() != nil {
		t.Fatal("copied scope canceled original", err)
	}
	h := &ResponderHandle{owner: r}
	h.self = h
	copyHandle := &ResponderHandle{self: h, owner: r}
	copyHandle.Close()
	if h.closed {
		t.Fatal("copied handle retired original")
	}
	if _, err := copyHandle.BorrowJoin(ctx, func() {}, func() error { return nil }); err == nil {
		t.Fatal("copied handle acquired JOIN")
	}
}
