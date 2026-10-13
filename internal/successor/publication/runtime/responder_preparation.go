package runtime

import (
	"context"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/route"
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
		_, err := opening.Binding(ctx)
		return err
	}
	var err error
	opening.responder, err = p.responderScope.Acquire(ctx)
	if err != nil {
		return err
	}
	known := append([]route.Member(nil), p.config.Exclusions...)
	intro := p.config.Duty
	known = append(known, route.Member{NodeID: intro.NodeID, PublicKey: intro.PublicKey, FamilyID: intro.FamilyID})
	if err := opening.responder.CheckRendezvous(opening.rendezvous, known); err != nil {
		return err
	}
	_, err = opening.Binding(ctx)
	return err
}
