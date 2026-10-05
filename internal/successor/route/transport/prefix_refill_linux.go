//go:build linux

package transport

import (
	"context"
	"errors"
	"net"
)

type prefixRefill struct{ cancel context.CancelFunc }

// Replenish requests fresh admission on the retained Entry and Interior parents.
// It preserves each original HELLO and does not create or retry a channel.
func (p *Prefix) Replenish(ctx context.Context) error {
	if p == nil || ctx == nil {
		return errors.New("route refill prefix absent")
	}
	op, cancel := context.WithCancel(ctx)
	if p.config.HoldRefill == nil {
		cancel()
		return errors.New("route refill local Hosting absent")
	}
	defer cancel()
	refill := &prefixRefill{cancel: cancel}
	p.registrationMu.Lock()
	select {
	case <-p.closing:
		p.registrationMu.Unlock()
		return net.ErrClosed
	default:
	}
	if p.entry == nil || p.interior == nil || len(p.refills) != 0 {
		p.registrationMu.Unlock()
		return errors.New("route refill prefix unpublished")
	}
	if p.refills == nil {
		p.refills = make(map[*prefixRefill]struct{})
	}
	p.refills[refill] = struct{}{}
	p.openings.Add(1)
	p.registrationMu.Unlock()
	defer func() {
		p.registrationMu.Lock()
		delete(p.refills, refill)
		p.registrationMu.Unlock()
		p.openings.Done()
	}()
	if err := errors.Join(op.Err(), p.originalCurrent()); err != nil {
		return err
	}
	hold := func(ctx context.Context, additional uint64) error {
		release, err := p.config.HoldRefill(ctx, additional)
		if release != nil {
			p.registrationMu.Lock()
			p.refillReturns = append(p.refillReturns, release)
			p.registrationMu.Unlock()
		}
		return err
	}
	if err := p.entry.replenish(op, p.caller, p.entryHello, p.config.Present, hold); err != nil {
		return err
	}
	if err := errors.Join(op.Err(), p.originalCurrent()); err != nil {
		return err
	}
	if err := p.interior.replenish(op, p.caller, p.interiorHello, p.config.Present); err != nil {
		return err
	}
	return errors.Join(op.Err(), p.originalCurrent())
}
