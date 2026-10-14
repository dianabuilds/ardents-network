package prefix

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network"
)

// SourceJoinBorrow transfers one exact acquired generation to JOIN. Its
// resource claim remains the original Prefix pair; the scope cannot substitute
// a new generation after observations, opening or handoff.
type SourceJoinBorrow struct {
	self       *SourceJoinBorrow
	owner      *SourceScope
	generation *sourceGeneration
	pair       *JoinBorrow
	once       sync.Once
}

func (h *SourceHandle) BorrowJoin(ctx context.Context, interrupt func(), join func() error) (*SourceJoinBorrow, error) {
	if ctx == nil {
		return nil, errors.New("source JOIN caller absent")
	}
	if err := h.Check(); err != nil {
		return nil, err
	}
	h.owner.mu.Lock()
	if err := h.owner.handleLocalLocked(h); err != nil || h.consumed {
		h.owner.mu.Unlock()
		return nil, errors.Join(errors.New("source handle already consumed or retired"), err)
	}
	h.consumed = true
	h.owner.mu.Unlock()
	pair, err := h.generation.physical.BorrowSourceJoin(ctx, interrupt, join)
	if err != nil {
		return nil, err
	}
	b := &SourceJoinBorrow{owner: h.owner, generation: h.generation, pair: pair}
	b.self = b
	if err := b.Publish(ctx, ctx, h.owner.ctx, func(check func() error) error { return check() }); err != nil {
		pair.ReturnJoined()
		return nil, err
	}
	return b, nil
}

func (b *SourceJoinBorrow) localLocked() error {
	if b.self != b || b.owner.self != b.owner || b.generation.owner != b.owner || b.owner.current != b.generation || b.generation.sealed {
		return errors.New("original scoped Source JOIN retired")
	}
	return b.owner.localLocked()
}

func (b *SourceJoinBorrow) CheckLocal() error {
	if b == nil || b.self != b {
		return errors.New("source JOIN borrow absent")
	}
	if err := b.pair.CheckLocal(); err != nil {
		return err
	}
	b.owner.mu.Lock()
	defer b.owner.mu.Unlock()
	return b.localLocked()
}

func (b *SourceJoinBorrow) CheckOriginal() error {
	if err := b.CheckLocal(); err != nil {
		return err
	}
	// Every physical generation retains this scope's exact leg and authentic
	// observer, including after replacement. Its full observation checks those
	// same public facts; the scope fences retain original local authority.
	return errors.Join(b.pair.CheckOriginal(), b.CheckLocal())
}

// CheckScope compares opaque original provenance, not equal public role facts.
func (b *SourceJoinBorrow) CheckScope(scope *SourceScope) error {
	if b == nil || b.self != b || b.owner != scope {
		return errors.New("source JOIN belongs to another scope")
	}
	return b.CheckOriginal()
}

// MatchSourceCaller compares original provenance and local seals only. JOIN
// must separately observe its actual acquisition before accepting the match.
func (b *SourceJoinBorrow) MatchSourceCaller(scope *SourceScope, caller context.Context, network, profile [32]byte, end time.Time) error {
	if b == nil || b.self != b || b.owner != scope {
		return errors.New("source JOIN belongs to another scope")
	}
	if err := scope.matchSourceCaller(caller, network, profile, end); err != nil {
		return err
	}
	return b.CheckLocal()
}

func (b *SourceJoinBorrow) WaitRetirement(ctx context.Context) { b.pair.WaitRetirement(ctx) }

func (b *SourceJoinBorrow) CheckRecipient(duty network.RetainedDuty, end time.Time) error {
	if err := b.CheckOriginal(); err != nil {
		return err
	}
	return b.pair.CheckRecipient(duty, end)
}

func (b *SourceJoinBorrow) OpenChannel(ctx, caller context.Context, duty network.RetainedDuty, end, setup time.Time) (*JoinChannel, uint8, error) {
	if err := b.CheckOriginal(); err != nil {
		return nil, 0, err
	}
	return b.pair.OpenChannel(ctx, caller, duty, end, setup)
}

func (b *SourceJoinBorrow) Publish(ctx, caller, operation context.Context, action func(func() error) error) error {
	if b == nil || b.self != b || action == nil {
		return errors.New("source JOIN handoff absent")
	}
	return b.pair.Publish(ctx, caller, operation, func(checkOriginal func() error) error {
		b.owner.mu.Lock()
		defer b.owner.mu.Unlock()
		check := func() error { return errors.Join(checkOriginal(), b.localLocked()) }
		if err := check(); err != nil {
			return err
		}
		return action(check)
	})
}

func (b *SourceJoinBorrow) ReturnJoined() {
	if b == nil || b.self != b {
		return
	}
	b.once.Do(b.pair.ReturnJoined)
}
