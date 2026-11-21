package runtime

import (
	"context"
	"errors"
	"time"
)

// Private nonce acceptance precedes any actual physical opening. Replacement
// opens its exact generation later, only after Connection consumes its finite
// proposal; this delivery cannot independently replenish recovery attempts.
func (opening *IntroductionOpening) prepareResponder(ctx context.Context) error {
	if opening == nil || opening.publisher == nil {
		return errors.New("Publisher Responder opening absent")
	}
	p := opening.publisher
	if _, err := opening.Binding(ctx); err != nil {
		return err
	}
	if err := p.responderScope.Check(); err != nil {
		return err
	}
	if err := p.responderScope.Commit(func(checkOriginal func() error) error {
		p.mu.Lock()
		defer p.mu.Unlock()
		if !p.acceptingPair(opening.pair, time.Now()) {
			return errors.New("Publisher recipient retired before nonce commit")
		}
		select {
		case <-opening.pair.registration.Done():
			return errors.New("Publisher original Registration retired before nonce commit")
		default:
		}
		if err := errors.Join(p.ctx.Err(), opening.pair.ctx.Err(), p.lifetime.Context().Err(), checkOriginal(), opening.binding.Check(ctx, p.config.Operation)); err != nil {
			return err
		}
		return opening.reservation.commit(time.Now())
	}); err != nil {
		return err
	}
	if opening.stream != nil {
		return ctx.Err()
	}
	if p.config.PrepareJoin != nil {
		// The accepted nonce and both selected recipients are now fixed. Fund
		// JOIN on the exact existing Source while opening the independently
		// funded Responder. Neither result is published before both join.
		source, err := p.sourceScope.CurrentHandle(ctx)
		if err != nil {
			return err
		}
		defer source.Close()
		preparing, cancel := context.WithCancel(ctx)
		defer cancel()
		funded := make(chan error, 1)
		go func() {
			err := p.config.PrepareJoin(preparing, source, opening.rendezvous)
			if err != nil {
				cancel()
			}
			funded <- err
		}()
		opening.responder, err = p.responderScope.Acquire(preparing)
		if err != nil {
			cancel()
		}
		// A failed or canceled sibling cannot leave an issuer operation using
		// the Source after this preparation/initial delivery has returned.
		return errors.Join(err, <-funded, ctx.Err())
	}
	var err error
	opening.responder, err = p.responderScope.Acquire(ctx)
	if err != nil {
		return err
	}
	// The sole caller immediately runs initialDelivery.retain. That owner
	// performs the complete post-acquisition Binding and Rendezvous observations
	// before its local commit and before any successful delivery output. No
	// accepting state escapes this private preparation, so do not duplicate
	// those same observations here with no intervening owned effect.
	return ctx.Err()
}
