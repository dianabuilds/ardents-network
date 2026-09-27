//go:build linux

package endpoint

import (
	"context"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/route/client"
)

// Select across at most the current and bounded predecessor registrations.
// Refresh wakes an already waiting receiver without consuming/losing a capsule
// in a canceled speculative goroutine. Acceptance still rechecks its owner.
func (owner *textContext) nextTextIntroductionDelivery(ctx context.Context) (*client.ClosedIntroductionDelivery, error) {
	for {
		owner.mu.Lock()
		if !owner.liveLocked(owner.endpoint, broker.Administration) || ctx.Err() != nil {
			owner.mu.Unlock()
			return nil, errors.Join(ctx.Err(), errors.New("text Introduction receiver retired"))
		}
		if owner.publication.pair.drainingLocked() {
			owner.mu.Unlock()
			return nil, errTextPublicationDraining
		}
		current := owner.publication.pair.currentLocked()
		previous, previousUntil := owner.publication.pair.previousLocked()
		if !owner.endpoint.clock().Before(previousUntil) {
			previous = nil
		}
		changed := owner.publication.pair.changedLocked()
		owner.mu.Unlock()
		var ready, priorReady, done, priorDone <-chan struct{}
		live := 0
		for i, registered := range []*textIntroductionRegistration{current, previous} {
			if registered == nil || registered.ended() {
				continue
			}
			delivery, err := registered.takeDelivery(ctx)
			if err != nil {
				continue
			}
			if delivery != nil {
				return delivery, nil
			}
			live++
			if i == 0 {
				ready, done = registered.deliverySignals()
			} else {
				priorReady, priorDone = registered.deliverySignals()
			}
		}
		if live == 0 {
			return nil, errors.New("text Introduction registrations unavailable")
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-changed:
		case <-ready:
		case <-priorReady:
		case <-done:
		case <-priorDone:
		}
	}
}
