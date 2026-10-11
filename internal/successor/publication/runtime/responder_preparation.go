package runtime

import (
	"context"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/route"
	"github.com/dianabuilds/ardents-network/internal/successor/route/prefix"
)

// prepareResponder runs under an existing private-opening flight. The gate
// serializes this one finite physical attempt; it holds no domain mutex during
// observations, opening or join. Drain waits for all such flights before
// returning the prepared reservation or closing the retained physical prefix.
func (opening *IntroductionOpening) prepareResponder(ctx context.Context) error {
	if opening == nil || opening.publisher == nil {
		return errors.New("Publisher Responder opening absent")
	}
	p := opening.publisher
	if _, err := opening.Binding(ctx); err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-p.ctx.Done():
		return p.ctx.Err()
	case <-p.responderGate:
	}
	defer func() { p.responderGate <- struct{}{} }()
	if _, err := opening.Binding(ctx); err != nil {
		return err
	}
	if p.responder == nil && p.responderAttempted {
		return errors.Join(errors.New("Publisher original Responder attempt ended"), p.responderFailure)
	}
	if p.config.Responder.Release == nil {
		return errors.New("Publisher Responder reservation absent")
	}
	// Fresh observations above run outside all generation locks. Only the
	// bounded local nonce transition runs with the original Source seal and
	// accepting-pair barrier. Later physical failure cannot undo this history.
	if err := p.config.Source.CommitPair(nil, func(checkOriginal func() error) error {
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
	if p.responder == nil {
		p.responderAttempted = true
		// Only opening I/O is interrupted by this capsule caller. Once joined,
		// the exact prefix retains the original Publisher lifetime; closing a
		// private opening must not silently retire a sibling's physical prefix.
		caller, cancel := context.WithCancel(p.physicalContext)
		callbackDone := make(chan struct{})
		stop := context.AfterFunc(ctx, func() { defer close(callbackDone); cancel() })
		physical, err := prefix.OpenResponder(caller, p.config.Source, p.config.Responder)
		if !stop() {
			<-callbackDone
		}
		err = errors.Join(err, ctx.Err(), caller.Err())
		if err == nil {
			_, err = opening.Binding(ctx)
		}
		var borrow *prefix.Borrow
		if err == nil {
			borrow, err = physical.BorrowLifetime(p.ctx, p.stop, p.joinSourceUsers)
		}
		if err != nil {
			cancel()
			retirement := errors.Join(physical.Close(), p.config.Responder.Release())
			p.responderFailure = errors.Join(err, retirement)
			p.mu.Lock()
			// A failed operation remains with this exact attempt. Only failed
			// physical retirement poisons joined cleanup capacity; an ordinary
			// peer refusal must not fabricate an unfinished original worker.
			p.failure = errors.Join(p.failure, retirement)
			p.mu.Unlock()
			return p.responderFailure
		}
		p.responder, p.responderCancel = physical, cancel
		p.responderBorrow = borrow
	}
	known := append([]route.Member(nil), p.config.Exclusions...)
	intro := p.config.Duty
	known = append(known, route.Member{NodeID: intro.NodeID, PublicKey: intro.PublicKey, FamilyID: intro.FamilyID})
	if err := p.config.Source.CheckResponderRendezvous(p.responder, opening.rendezvous, known); err != nil {
		return err
	}
	_, err := opening.Binding(ctx)
	return err
}
