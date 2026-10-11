package runtime

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/connection"
	"github.com/dianabuilds/ardents-network/internal/successor/network"
	"github.com/dianabuilds/ardents-network/internal/successor/publication/instance"
	"github.com/dianabuilds/ardents-network/internal/successor/reachability"
	"github.com/dianabuilds/ardents-network/internal/successor/route"
	"github.com/dianabuilds/ardents-network/internal/successor/route/capsule"
)

// IntroductionOpening retains one accepting pair, its private recipient and
// context-local nonce reservation. Candidate plaintext is not a Connection or
// delivery acknowledgement. Close joins private opening cleanup before returning
// this borrow; successful replay retention will belong to downstream acceptance.
type IntroductionOpening struct {
	publisher   *Publisher
	pair        *registrationPair
	private     *instance.Opening
	reservation *openingReservation
	binding     *connection.RecipientBinding
	rendezvous  network.RetainedDuty
	once        sync.Once
}

func (p *Publisher) acceptingPair(pair *registrationPair, now time.Time) bool {
	return !p.closed && pair != nil && (p.current == pair || p.predecessor == pair) &&
		!pair.firstACK.IsZero() && now.Before(pair.cutoff)
}

// OpenIntroduction reserves rate and replay capacity before HPKE under an
// actual acknowledged current/predecessor pair. Pending pairs cannot open.
// The returned bounded borrow must be closed even after caller/worker loss.
func (p *Publisher) OpenIntroduction(ctx context.Context, envelope capsule.Envelope) (_ *IntroductionOpening, result error) {
	if p == nil || ctx == nil {
		return nil, errors.New("Publisher introduction unavailable")
	}
	if err := p.check(ctx); err != nil {
		return nil, err
	}
	header := envelope.Header()
	now := time.Now()
	p.mu.Lock()
	var pair *registrationPair
	for _, candidate := range []*registrationPair{p.current, p.predecessor} {
		if p.acceptingPair(candidate, now) && candidate.receipt.Facts().Slot == header.Slot && candidate.receipt.Facts().Revision == header.Revision {
			pair = candidate
			break
		}
	}
	if pair == nil {
		p.mu.Unlock()
		return nil, errors.New("Publisher accepting recipient absent")
	}
	p.flights.Add(1)
	p.mu.Unlock()
	opening := &IntroductionOpening{publisher: p, pair: pair}
	defer func() {
		if result != nil {
			opening.Close()
		}
	}()
	if err := pair.check(p, ctx); err != nil {
		return nil, err
	}
	reservation, err := p.history.reserve(header.DeliveryNonce, header.Expiry, pair.receipt.Facts().Expiry, now)
	if err != nil {
		return nil, err
	}
	opening.reservation = reservation
	opening.private, err = pair.recipient.OpenCapsule(ctx, envelope)
	if err != nil {
		return nil, err
	}
	request, digest, err := opening.Candidate(ctx)
	if err != nil {
		return nil, err
	}
	known := append([]route.Member(nil), p.config.Exclusions...)
	intro := p.config.Duty
	known = append(known, route.Member{NodeID: intro.NodeID, PublicKey: intro.PublicKey, FamilyID: intro.FamilyID})
	duty, end, err := p.config.Source.ResolveRendezvous(request.RendezvousNode, request.RendezvousDutyGeneration, known)
	if err != nil {
		return nil, err
	}
	if p.config.Deadline.Before(end) {
		end = p.config.Deadline
	}
	if bound, exists := p.config.Operation.Context().Deadline(); exists && bound.Before(end) {
		end = bound
	}
	value := p.credential.Delegation()
	proof, err := reachability.Verify(pair.descriptor, value.Target, value.Network, pair.receipt.Facts().Profile, time.Now())
	if err != nil {
		return nil, err
	}
	opening.binding, err = connection.BindRecipient(opening.private.Context(), p.config.Operation, proof.Publication, pair.receipt.Facts().Profile, request, digest, end)
	if err != nil {
		return nil, err
	}
	opening.rendezvous = duty
	if _, _, err := opening.Candidate(ctx); err != nil {
		return nil, err
	}
	if err := p.config.Source.CheckRendezvous(duty, known); err != nil {
		return nil, err
	}
	return opening, nil
}

// Binding returns the original immutable initial binding only after checking
// the same live pair and qualified operation. It supplies no replay commit,
// Responder readiness, successful delivery or Service TLS authentication.
func (opening *IntroductionOpening) Binding(ctx context.Context) (*connection.RecipientBinding, error) {
	if opening == nil || opening.binding == nil {
		return nil, errors.New("Publisher recipient binding unavailable")
	}
	if _, _, err := opening.Candidate(ctx); err != nil {
		return nil, err
	}
	if err := opening.binding.Check(ctx, opening.publisher.config.Operation); err != nil {
		return nil, err
	}
	p := opening.publisher
	known := append([]route.Member(nil), p.config.Exclusions...)
	intro := p.config.Duty
	known = append(known, route.Member{NodeID: intro.NodeID, PublicKey: intro.PublicKey, FamilyID: intro.FamilyID})
	duty, _, err := p.config.Source.ResolveRendezvous(opening.rendezvous.NodeID, opening.rendezvous.RecordGeneration, known)
	if err != nil || duty != opening.rendezvous {
		return nil, errors.Join(errors.New("Publisher original Rendezvous changed"), err)
	}
	if _, _, err := opening.Candidate(ctx); err != nil {
		return nil, err
	}
	return opening.binding, nil
}

func (opening *IntroductionOpening) Candidate(ctx context.Context) (capsule.Request, [32]byte, error) {
	if opening == nil || opening.publisher == nil || opening.private == nil {
		return capsule.Request{}, [32]byte{}, errors.New("Publisher opening unavailable")
	}
	p := opening.publisher
	if err := opening.pair.check(p, ctx); err != nil {
		return capsule.Request{}, [32]byte{}, err
	}
	p.mu.Lock()
	accepting := p.acceptingPair(opening.pair, time.Now())
	p.mu.Unlock()
	if !accepting {
		return capsule.Request{}, [32]byte{}, errors.New("Publisher opening pair retired")
	}
	return opening.private.Request(ctx)
}

func (opening *IntroductionOpening) Close() {
	if opening == nil {
		return
	}
	opening.once.Do(func() {
		if opening.private != nil {
			opening.private.Close()
		}
		if opening.reservation != nil {
			opening.reservation.release()
		}
		opening.publisher.flights.Done()
	})
}
