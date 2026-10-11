package runtime

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/route"
	"github.com/dianabuilds/ardents-network/internal/successor/route/introduction"
)

// initialDelivery retains the exact initial binding/private opening through
// real wire completion, then until its original capsule deadline. It grants no
// Service TLS or Application stream and cannot accept a recovery Attachment.
// The original delivery caller can cancel setup; its callback is relinquished
// only after matching RESULT/CLOSE and final original authority checks.
type initialDelivery struct {
	publisher  *Publisher
	delivery   *introduction.Delivery
	opening    *IntroductionOpening
	ctx        context.Context
	cancel     context.CancelFunc
	stopCaller func() bool
	callerDone chan struct{}
	callerOnce sync.Once
	closeOnce  sync.Once
	nonce      [32]byte
	wireJoined bool // Publisher.mu owns publication of this retained state.
}

func (p *Publisher) newInitialDelivery(delivery *introduction.Delivery) *initialDelivery {
	p.initialUsers.Add(1) // The original inspect flight is already retained.
	ctx, cancel := context.WithDeadline(p.ctx, delivery.Capsule().Header().Expiry)
	state := &initialDelivery{publisher: p, delivery: delivery, ctx: ctx, cancel: cancel, callerDone: make(chan struct{})}
	state.stopCaller = context.AfterFunc(delivery.Context(), func() { defer close(state.callerDone); cancel() })
	return state
}

func (state *initialDelivery) joinCaller() {
	state.callerOnce.Do(func() {
		if !state.stopCaller() {
			<-state.callerDone
		}
	})
}

// observe checks actual owners outside generation/state locks. It must run
// after each network flight; copied transcripts alone cannot publish a state.
func (state *initialDelivery) observe() error {
	if state.opening == nil {
		return errors.New("Publisher initial binding absent")
	}
	if _, err := state.opening.Binding(state.ctx); err != nil {
		return err
	}
	p := state.publisher
	known := append([]route.Member(nil), p.config.Exclusions...)
	intro := p.config.Duty
	known = append(known, route.Member{NodeID: intro.NodeID, PublicKey: intro.PublicKey, FamilyID: intro.FamilyID})
	return errors.Join(state.delivery.Context().Err(), p.config.Source.CheckResponderRendezvous(p.responder, state.opening.rendezvous, known))
}

func (state *initialDelivery) retain() error {
	if err := state.observe(); err != nil {
		return err
	}
	p := state.publisher
	nonce := state.delivery.Capsule().Header().DeliveryNonce
	return p.config.Source.CommitPair(p.responder, func(checkOriginal func() error) error {
		p.mu.Lock()
		defer p.mu.Unlock()
		if err := state.checkLocal(checkOriginal); err != nil {
			return err
		}
		if nonce == [32]byte{} || p.initialDeliveries[nonce] != nil || len(p.initialDeliveries) >= 2640 {
			return errors.New("Publisher initial delivery already retained or full")
		}
		state.nonce = nonce
		p.initialDeliveries[nonce] = state
		return nil
	})
}

// publishJoined follows the actual matching terminal CLOSE. The callback stop
// is joined before the delivery borrower is returned: our own Close must not
// revoke the independently retained initial scope or a sibling prefix.
func (state *initialDelivery) publishJoined() error {
	state.joinCaller()
	if err := state.observe(); err != nil {
		return err
	}
	p := state.publisher
	return p.config.Source.CommitPair(p.responder, func(checkOriginal func() error) error {
		p.mu.Lock()
		defer p.mu.Unlock()
		if p.initialDeliveries[state.nonce] != state {
			return errors.New("Publisher initial binding retention changed")
		}
		if err := state.checkLocal(checkOriginal); err != nil {
			return err
		}
		state.wireJoined = true
		return nil
	})
}

// Caller owns the Publisher lock and both physical generation locks. No
// observation, storage, wire I/O or join occurs in this local transition.
func (state *initialDelivery) checkLocal(checkOriginal func() error) error {
	p := state.publisher
	if !p.acceptingPair(state.opening.pair, time.Now()) {
		return errors.New("Publisher initial pair retired")
	}
	select {
	case <-state.opening.pair.registration.Done():
		return errors.New("Publisher original initial Registration retired")
	default:
	}
	return errors.Join(state.ctx.Err(), state.delivery.Context().Err(), p.ctx.Err(), p.lifetime.Context().Err(),
		checkOriginal(), state.opening.binding.Check(state.ctx, p.config.Operation))
}

func (state *initialDelivery) close() {
	state.closeOnce.Do(func() {
		defer state.publisher.initialUsers.Done()
		state.cancel()
		state.joinCaller()
		if state.opening != nil {
			state.opening.Close()
		}
		state.delivery.Close()
		p := state.publisher
		p.mu.Lock()
		if p.initialDeliveries[state.nonce] == state {
			delete(p.initialDeliveries, state.nonce)
		}
		p.mu.Unlock()
	})
}
