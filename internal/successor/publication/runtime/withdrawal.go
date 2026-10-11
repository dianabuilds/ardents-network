package runtime

import (
	"context"
	"errors"
)

// Withdraw synchronously closes acquisition, interrupts a pending flight,
// sends owning WITHDRAW on retained current/predecessor registrations, then
// joins the original cleanup. A canceled caller never releases cleanup early.
func (p *Publisher) Withdraw(ctx context.Context) error {
	if p == nil {
		return nil
	}
	if ctx == nil {
		return errors.New("Publisher withdrawal caller absent")
	}
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return p.Close()
	}
	p.closed = true
	p.flights.Add(1)
	pending := p.pending
	current, prior := p.current, p.predecessor
	if pending != nil {
		if pending.registration != nil {
			pending.registration.Stop()
		}
		pending.cancel()
	}
	p.mu.Unlock()
	select {
	case p.wake <- struct{}{}:
	default:
	}
	var result error
	for _, pair := range []*registrationPair{current, prior} {
		if pair != nil {
			result = errors.Join(result, pair.registration.Withdraw(ctx))
		}
	}
	p.mu.Lock()
	p.failure = errors.Join(p.failure, result)
	p.mu.Unlock()
	p.flights.Done()
	return p.Close()
}
