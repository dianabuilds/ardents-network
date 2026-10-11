package runtime

import "github.com/dianabuilds/ardents-network/internal/successor/route/introduction"

// receiveIntroductions consumes actual typed children from this original
// Registration. Initial binding/replay/Responder/state checks decide success;
// no supplied acknowledgement callback can provide that authority.
func (p *Publisher) receiveIntroductions(pair *registrationPair) {
	defer p.flights.Done()
	for {
		delivery, err := pair.registration.NextDelivery(pair.ctx)
		if err != nil {
			return
		}
		p.mu.Lock()
		if p.closed {
			p.mu.Unlock()
			delivery.Close()
			return
		}
		p.flights.Add(1)
		p.mu.Unlock()
		go func() { defer p.flights.Done(); p.inspectIntroduction(delivery) }()
	}
}

func (p *Publisher) inspectIntroduction(delivery *introduction.Delivery) {
	state := p.newInitialDelivery(delivery)
	retained := false
	defer func() {
		if !retained {
			state.close()
		}
	}()
	var err error
	state.opening, err = p.OpenIntroduction(state.ctx, delivery.Capsule())
	if err == nil {
		err = state.opening.prepareResponder(state.ctx)
	}
	if err == nil {
		err = state.retain()
	}
	if err != nil {
		_ = delivery.Reply(delivery.Context(), 1)
		return
	}
	if err := delivery.Reply(state.ctx, 0); err != nil {
		return
	}
	if err := state.publishJoined(); err != nil {
		return
	}
	delivery.Close()
	retained = true
	go func() { <-state.ctx.Done(); state.close() }()
}
