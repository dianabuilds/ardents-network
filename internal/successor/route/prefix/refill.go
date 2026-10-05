package prefix

import (
	"context"
	"errors"
	framing "github.com/dianabuilds/ardents-network/internal/successor/route/channel"
	"net"
)

type prefixRefill struct{ cancel context.CancelFunc }

// Replenish requests fresh admission on the retained Entry and Interior parents.
// It preserves each original HELLO and does not create or retry a channel.
func (p *Prefix) Replenish(ctx context.Context) error {
	if p == nil || ctx == nil {
		return errors.New("route refill prefix absent")
	}
	derived, cancel := context.WithCancel(ctx)
	op := framing.WithRequest(ctx, derived)
	if p.config.HoldRefill == nil {
		cancel()
		return errors.New("route refill local Hosting absent")
	}
	defer cancel()
	refill := &prefixRefill{cancel: cancel}
	p.lifetimeMu.Lock()
	select {
	case <-p.closing:
		p.lifetimeMu.Unlock()
		return net.ErrClosed
	default:
	}
	if p.entry == nil || p.interior == nil || len(p.refills) != 0 {
		p.lifetimeMu.Unlock()
		return errors.New("route refill prefix unpublished")
	}
	if p.refills == nil {
		p.refills = make(map[*prefixRefill]struct{})
	}
	p.refills[refill] = struct{}{}
	p.openings.Add(1)
	p.lifetimeMu.Unlock()
	defer func() {
		p.lifetimeMu.Lock()
		delete(p.refills, refill)
		p.lifetimeMu.Unlock()
		p.openings.Done()
	}()
	check := func() error {
		if err := op.Err(); err != nil {
			return err
		}
		return errors.Join(p.originalCurrent(), op.Err())
	}
	if err := check(); err != nil {
		return err
	}
	hold := func(ctx context.Context, additional uint64) error {
		release, err := p.config.HoldRefill(ctx, additional)
		if release != nil {
			p.lifetimeMu.Lock()
			p.refillReturns = append(p.refillReturns, release)
			p.lifetimeMu.Unlock()
		}
		return err
	}
	if err := replenishParent(p.entry, op, ctx, p.entryHello, p.config.Present, hold); err != nil {
		return err
	}
	if err := check(); err != nil {
		return err
	}
	if err := replenishParent(p.interior, op, ctx, p.interiorHello, p.config.Present); err != nil {
		return err
	}
	return check()
}
