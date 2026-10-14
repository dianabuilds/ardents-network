package prefix

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network"
)

// ResponderJoinBorrow transfers one exact acquired generation to JOIN. Its
// resource claim remains the original Prefix pair; the scope cannot substitute
// a new generation after observations, opening or handoff.
type ResponderJoinBorrow struct {
	self       *ResponderJoinBorrow
	owner      *ResponderScope
	generation *responderGeneration
	pair       *JoinBorrow
	once       sync.Once
}

func (h *ResponderHandle) BorrowJoin(ctx context.Context, interrupt func(), join func() error) (*ResponderJoinBorrow, error) {
	if ctx == nil {
		return nil, errors.New("responder JOIN caller absent")
	}
	if err := h.Check(); err != nil {
		return nil, err
	}
	h.owner.mu.Lock()
	if err := h.owner.handleLocalLocked(h); err != nil || h.consumed {
		h.owner.mu.Unlock()
		return nil, errors.Join(errors.New("responder handle already consumed or retired"), err)
	}
	h.consumed = true
	h.owner.mu.Unlock()
	pair, err := h.generation.physical.BorrowResponderJoin(ctx, interrupt, join)
	if err != nil {
		return nil, err
	}
	b := &ResponderJoinBorrow{owner: h.owner, generation: h.generation, pair: pair}
	b.self = b
	if err := b.Publish(ctx, ctx, h.owner.ctx, func(check func() error) error { return check() }); err != nil {
		pair.ReturnJoined()
		return nil, err
	}
	return b, nil
}

func (b *ResponderJoinBorrow) localLocked() error {
	if b.self != b || b.owner.self != b.owner || b.generation.owner != b.owner || b.owner.current != b.generation || b.generation.sealed {
		return errors.New("original scoped Responder JOIN retired")
	}
	return b.owner.localLocked()
}

func (b *ResponderJoinBorrow) CheckLocal() error {
	if b == nil || b.self != b {
		return errors.New("responder JOIN borrow absent")
	}
	if err := b.pair.CheckLocal(); err != nil {
		return err
	}
	// A Source scope can already be sealed while its physical users are still
	// joining. Preserve that immediate logical refusal independently of both
	// physical Prefixes, with the same Source-before-Responder lock order.
	return b.owner.source.Commit(func(checkSource func() error) error {
		if err := b.owner.source.handleLocalLocked(b.generation.source); err != nil {
			return err
		}
		b.owner.mu.Lock()
		defer b.owner.mu.Unlock()
		return errors.Join(checkSource(), b.localLocked())
	})
}

func (b *ResponderJoinBorrow) CheckOriginal() error {
	if err := b.CheckLocal(); err != nil {
		return err
	}
	// The exact physical pair retains both scopes' original leg/observer
	// contracts. Reobserve it fully once, fenced by the original logical
	// scopes; repeated logical observation grants no additional provenance.
	return errors.Join(b.pair.CheckOriginal(), b.CheckLocal())
}

// CheckScope compares opaque original provenance, not equal public role facts.
func (b *ResponderJoinBorrow) CheckScope(scope *ResponderScope) error {
	if b == nil || b.self != b || b.owner != scope {
		return errors.New("responder JOIN belongs to another scope")
	}
	return b.CheckOriginal()
}

func (b *ResponderJoinBorrow) WaitRetirement(ctx context.Context) { b.pair.WaitRetirement(ctx) }

func (b *ResponderJoinBorrow) CheckRecipient(duty network.RetainedDuty, end time.Time) error {
	if err := b.CheckOriginal(); err != nil {
		return err
	}
	return b.pair.CheckRecipient(duty, end)
}

func (b *ResponderJoinBorrow) OpenChannel(ctx, caller context.Context, duty network.RetainedDuty, end, setup time.Time) (*JoinChannel, uint8, error) {
	if err := b.CheckOriginal(); err != nil {
		return nil, 0, err
	}
	return b.pair.OpenChannel(ctx, caller, duty, end, setup)
}

func (b *ResponderJoinBorrow) Publish(ctx, caller, operation context.Context, action func(func() error) error) error {
	if b == nil || b.self != b || action == nil {
		return errors.New("responder JOIN handoff absent")
	}
	return b.pair.Publish(ctx, caller, operation, func(checkOriginal func() error) error {
		return b.generation.source.owner.Commit(func(checkSource func() error) error {
			b.owner.mu.Lock()
			defer b.owner.mu.Unlock()
			check := func() error { return errors.Join(checkOriginal(), checkSource(), b.localLocked()) }
			if err := check(); err != nil {
				return err
			}
			return action(check)
		})
	})
}

func (b *ResponderJoinBorrow) ReturnJoined() {
	if b == nil || b.self != b {
		return
	}
	b.once.Do(b.pair.ReturnJoined)
}
