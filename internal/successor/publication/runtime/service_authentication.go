package runtime

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/connection"
	"github.com/dianabuilds/ardents-network/internal/successor/route/join"
)

// NextAuthentication consumes a genuinely completed initial delivery and opens
// its exact original Source-bound Responder JOIN. It returns only the sealed
// coalesced Service authentication; workload handoff still requires the logical
// stream and its original Route-backed recovery composition.
func (p *Publisher) NextAuthentication(ctx context.Context) (_ *connection.AuthenticatedAttachment, result error) {
	if p == nil || p.self != p || ctx == nil {
		return nil, errors.New("Publisher Service authentication caller is absent")
	}
	var state *initialDelivery
	for state == nil {
		p.mu.Lock()
		closed := p.closed
		p.mu.Unlock()
		if closed {
			return nil, p.stoppedAcquisition(ctx)
		}
		if err := p.check(ctx); err != nil {
			return nil, err
		}
		p.mu.Lock()
		if p.closed {
			p.mu.Unlock()
			return nil, p.stoppedAcquisition(ctx)
		}
		for _, candidate := range p.initialDeliveries {
			if candidate.wireJoined && !candidate.serviceClaimed && candidate.opening.stream == nil && candidate.ctx.Err() == nil {
				candidate.serviceClaimed = true
				state = candidate
				p.flights.Add(1)
				break
			}
		}
		p.mu.Unlock()
		if state == nil {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-p.ctx.Done():
				return nil, p.stoppedAcquisition(ctx)
			case <-p.initialWake:
			}
		}
	}
	defer p.flights.Done()
	defer state.close()
	if err := state.observe(); err != nil {
		return nil, err
	}
	request, _, err := state.opening.Candidate(state.ctx)
	if err != nil {
		return nil, err
	}
	acquisition, err := join.AcquireScopedResponderJoin(p.ctx, state.opening.responder)
	if err != nil {
		return nil, err
	}
	setup, cancel := context.WithCancel(state.ctx)
	defer cancel()
	callerDone := make(chan struct{})
	stopCaller := context.AfterFunc(ctx, func() { defer close(callerDone); cancel() })
	defer func() {
		if !stopCaller() {
			<-callerDone
		}
	}()
	interrupted := make(chan struct{})
	stopInterruption := context.AfterFunc(setup, func() {
		defer close(interrupted)
		_ = acquisition.Close()
	})
	var interruptionOnce sync.Once
	joinInterruption := func() {
		interruptionOnce.Do(func() {
			if !stopInterruption() {
				<-interrupted
			}
		})
	}
	defer func() {
		joinInterruption()
		if result != nil {
			result = errors.Join(result, acquisition.Close())
		}
	}()
	if err := errors.Join(ctx.Err(), setup.Err(), state.observe()); err != nil {
		return nil, err
	}
	physical, err := acquisition.Join(p.physicalContext, join.JoinConfig{Duty: state.opening.rendezvous,
		Secret: request.JoinSecret, Context: request.HandshakeContext, Deadline: time.Unix(request.WorkSafetyNotAfter, 0).UTC(), SetupDeadline: request.Deadline})
	if err != nil {
		return nil, err
	}
	if err := errors.Join(ctx.Err(), setup.Err(), state.observe()); err != nil {
		return nil, err
	}
	owner, err := connection.AuthenticatePublisher(setup, p.config.Operation, state.opening.binding, p.lifetime, state.opening.private, physical)
	if err != nil {
		return nil, err
	}
	defer func() {
		if result != nil {
			result = errors.Join(result, owner.Close())
		}
	}()
	// Join the attempt callback before publishing transfer. Normal setup
	// cancellation below must never close the successfully retained JOIN.
	joinInterruption()
	if err := errors.Join(ctx.Err(), setup.Err(), state.observe(), p.check(ctx)); err != nil {
		return nil, err
	}
	if err := state.opening.responder.Commit(func(checkOriginal func() error) error {
		p.mu.Lock()
		defer p.mu.Unlock()
		if p.initialDeliveries[state.nonce] != state || !state.wireJoined || !state.serviceClaimed {
			return errors.New("original accepted delivery changed before Service handoff")
		}
		return errors.Join(ctx.Err(), setup.Err(), state.checkLocal(checkOriginal))
	}); err != nil {
		return nil, err
	}
	return owner, nil
}
