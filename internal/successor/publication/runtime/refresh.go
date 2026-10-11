package runtime

import (
	"errors"
	"time"
)

// The scheduler uses real wall time and original REGISTER creation, never ACK
// time. No accelerated clock is part of product scheduling.
func (p *Publisher) schedule() {
	defer close(p.schedulerDone)
	for {
		p.mu.Lock()
		current, prior, pending, closed, flight := p.current, p.predecessor, p.pending, p.closed, p.flight
		var next time.Time
		var ended <-chan struct{}
		if current != nil {
			ended = current.registration.Done()
		}
		if prior != nil {
			next = prior.cutoff
		} else if !flight && pending != nil {
			next = pending.receipt.Facts().Expiry
			ended = pending.registration.Done()
		} else if current != nil {
			next = current.receipt.Facts().Created.Add(300 * time.Second)
		}
		if flight && prior == nil {
			next = time.Time{}
		}
		p.mu.Unlock()
		if closed {
			return
		}
		var timer *time.Timer
		var tick <-chan time.Time
		if !next.IsZero() {
			timer = time.NewTimer(time.Until(next))
			tick = timer.C
		}
		select {
		case <-p.ctx.Done():
			if timer != nil {
				timer.Stop()
			}
			return
		case <-p.wake:
			if timer != nil {
				timer.Stop()
			}
			continue
		case <-ended:
			if timer != nil {
				timer.Stop()
			}
			p.stop()
			return
		case <-tick:
		}
		if prior != nil {
			if err := prior.close(); err != nil {
				p.mu.Lock()
				p.failure = errors.Join(p.failure, err)
				p.mu.Unlock()
				p.stop()
				return
			}
			p.mu.Lock()
			if p.predecessor == prior {
				p.predecessor = nil
			}
			p.mu.Unlock()
			continue
		}
		if pending != nil {
			p.stop()
			return
		}
		if _, err := p.Refresh(p.ctx); err != nil {
			if errors.Is(err, errFlight) || errors.Is(err, errPredecessor) {
				continue
			}
			// A failed exact refresh retains its pending proof and cleanup;
			// background wakeups never invent a fresh slot or widen retries.
			p.mu.Lock()
			p.failure = errors.Join(p.failure, err)
			p.mu.Unlock()
			p.mu.Lock()
			stopped, retained := p.closed, p.pending != nil
			p.mu.Unlock()
			if stopped {
				return
			}
			if !retained {
				p.stop()
				return
			}
		}
	}
}
