package runtime

import (
	"context"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/route/introduction"
)

var errFlight = errors.New("Publisher flight unavailable")
var errPredecessor = errors.New("Publisher predecessor still retained")

func (p *Publisher) publish(ctx context.Context, replace bool) (string, error) {
	if p == nil || ctx == nil {
		return "", errors.New("Publisher absent")
	}
	if err := p.check(ctx); err != nil {
		return "", err
	}
	p.mu.Lock()
	if p.closed || p.flight {
		p.mu.Unlock()
		return "", errFlight
	}
	if p.current != nil && !replace && p.pending == nil {
		p.mu.Unlock()
		return p.Link(ctx)
	}
	if replace && p.current == nil {
		p.mu.Unlock()
		return "", errors.New("Publisher current pair absent")
	}
	if p.predecessor != nil {
		p.mu.Unlock()
		return "", errPredecessor
	}
	pair := p.pending
	if pair == nil {
		if p.revision == ^uint64(0) {
			p.mu.Unlock()
			return "", errors.New("Publisher revision exhausted")
		}
		p.revision++
		child, cancel := context.WithCancel(p.ctx)
		pair = &registrationPair{ctx: child, cancel: cancel, callerCallback: make(chan struct{})}
		pair.stopCaller = context.AfterFunc(ctx, func() { defer close(pair.callerCallback); cancel() })
		p.pending = pair
	}
	p.flight = true
	p.flights.Add(1)
	revision, initial := p.revision, p.current == nil
	p.mu.Unlock()
	defer func() {
		p.mu.Lock()
		p.flight = false
		p.mu.Unlock()
		p.flights.Done()
		select {
		case p.wake <- struct{}{}:
		default:
		}
	}()
	if pair.registration == nil {
		if err := p.preparePair(pair, revision, initial); err != nil {
			p.stop() // No successful slot/proof may be silently regenerated.
			return "", err
		}
	}
	if err := pair.check(p, ctx); err != nil {
		return "", err
	}
	end := time.Now().Add(30 * time.Second).UTC().Truncate(time.Second)
	if p.config.Deadline.Before(end) {
		end = p.config.Deadline
	}
	if pair.receipt.Facts().Expiry.Before(end) {
		end = pair.receipt.Facts().Expiry
	}
	exchange, cancel := context.WithCancel(pair.ctx)
	callerJoined := make(chan struct{})
	stopCaller := context.AfterFunc(ctx, func() { defer close(callerJoined); cancel() })
	defer func() {
		cancel()
		if !stopCaller() {
			<-callerJoined
		}
	}()
	if bound, ok := ctx.Deadline(); ok && bound.Before(end) {
		end = bound.UTC().Truncate(time.Second)
	}
	if err := p.config.Source.PublishDescriptor(exchange, end, pair.descriptor, p.config.Exclusions); err != nil {
		return "", err
	}
	// Prefix success is the actual synced Store ACK after physical exchange
	// join. Reobserve all original local/registration/Descriptor facts before
	// the accepting transition; no network call holds the lifecycle mutex.
	if err := pair.check(p, ctx); err != nil {
		return "", err
	}
	p.mu.Lock()
	if p.closed || p.pending != pair {
		p.mu.Unlock()
		return "", errors.New("Publisher late acknowledgement")
	}
	if err := p.check(ctx); err != nil {
		p.mu.Unlock()
		return "", err
	}
	now := time.Now()
	pair.firstACK = now
	pair.cutoff = pair.receipt.Facts().Expiry
	if p.current != nil {
		prior := p.current
		prior.cutoff = prior.receipt.Facts().Expiry
		if bound := now.Add(60 * time.Second); bound.Before(prior.cutoff) {
			prior.cutoff = bound
		}
		// Seal the private accepting boundary synchronously with the first
		// switch. Timer scheduling/physical join may be late; neither can
		// extend this recipient's authority or change its signed expiry.
		if err := prior.recipient.LimitAcceptance(prior.cutoff); err != nil {
			p.mu.Unlock()
			p.stop()
			return "", err
		}
		p.predecessor = prior
	}
	p.current, p.pending = pair, nil
	p.mu.Unlock()
	select {
	case p.wake <- struct{}{}:
	default:
	}
	return p.Link(ctx)
}

func (p *Publisher) preparePair(pair *registrationPair, revision uint64, initial bool) error {
	value := p.credential.Delegation()
	end := time.Now().Add(600 * time.Second).UTC().Truncate(time.Second)
	for _, bound := range []time.Time{p.config.Deadline, value.NotAfter, p.config.Duty.RecordValidUntil, p.config.Duty.Epoch.ValidUntil} {
		if bound.Before(end) {
			end = bound.UTC().Truncate(time.Second)
		}
	}
	if bound, ok := pair.ctx.Deadline(); ok && bound.Before(end) {
		end = bound.UTC().Truncate(time.Second)
	}
	registration, err := introduction.Register(pair.ctx, p.config.Introduction, introduction.RegistrationConfig{Duty: p.config.Duty, Revision: revision, Deadline: end})
	if err != nil {
		return err
	}
	// Retirement can interrupt a flight while REGISTER is completing. Publish
	// the returned physical owner under the same barrier used by Stop; a late
	// owner is sealed and remains ours to join, never an accepting replacement.
	p.mu.Lock()
	pair.registration = registration
	if p.closed {
		registration.Stop()
	}
	p.mu.Unlock()
	pair.receipt, err = registration.Receipt()
	if err != nil {
		return err
	}
	if initial {
		if _, err = p.config.Binding.Publication(pair.ctx, registration); err != nil {
			return err
		}
	}
	pair.recipient, err = p.config.Binding.NewRecipient(pair.ctx, registration)
	if err != nil {
		return err
	}
	pair.descriptor, err = pair.recipient.Descriptor(pair.ctx)
	if err == nil {
		p.mu.Lock()
		if !p.closed {
			p.flights.Add(1)
			go p.receiveIntroductions(pair)
		}
		p.mu.Unlock()
	}
	return err
}
