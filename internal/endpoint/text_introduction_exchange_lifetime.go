//go:build linux

package endpoint

import (
	"context"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/route"
)

type textIntroductionExchange struct {
	cancel context.CancelFunc
	done   chan struct{}
}

// Reserve before the first State read or I/O. Context retirement cancels and
// joins these exchanges even when the worker's own cleanup has already ended.
func (owner *textContext) beginTextIntroductionExchange(caller context.Context, job *textJobIdentity, surface broker.Surface) (context.Context, func(error) error, error) {
	owner.mu.Lock()
	if caller == nil || caller.Err() != nil || !owner.liveTextServiceJobLocked(job, surface) || len(owner.introductionExchanges) >= 16 {
		owner.mu.Unlock()
		return nil, nil, errors.New("text Introduction exchange owner unavailable")
	}
	lifetime, cancel := context.WithCancel(job.context)
	flight := &textIntroductionExchange{cancel: cancel, done: make(chan struct{})}
	if owner.introductionExchanges == nil {
		owner.introductionExchanges = make(map[*textIntroductionExchange]struct{})
	}
	owner.introductionExchanges[flight] = struct{}{}
	owner.mu.Unlock()
	interrupted := make(chan struct{})
	stop := context.AfterFunc(caller, func() { defer close(interrupted); cancel() })
	if caller.Err() != nil {
		cancel()
	}
	return lifetime, func(outcome error) error {
		cancel()
		if !stop() {
			<-interrupted
		}
		outcome = errors.Join(outcome, caller.Err(), job.context.Err())
		owner.mu.Lock()
		defer owner.mu.Unlock()
		if errors.Is(outcome, route.ErrClosedSourceCleanup) {
			owner.closeErr = errors.Join(owner.closeErr, outcome)
			owner.closed = true
			owner.endpoint.failTextContexts(outcome)
		}
		delete(owner.introductionExchanges, flight)
		close(flight.done)
		return outcome
	}, nil
}
