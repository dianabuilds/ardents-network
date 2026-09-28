//go:build linux

package endpoint

import (
	"context"
	"errors"
	"sync"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/route/client"
)

type introductionExchange struct {
	cancel   context.CancelFunc
	done     chan struct{}
	retained bool
}

// Reserve before the first State read or I/O. Context retirement cancels and
// joins these exchanges even when the worker's own cleanup has already ended.
func (owner *textContext) beginIntroductionExchange(caller context.Context, job *textJobIdentity, surface broker.Surface) (context.Context, func(error) error, error) {
	owner.mu.Lock()
	if caller == nil || caller.Err() != nil || !owner.liveTextServiceJobLocked(job, surface) || owner.introduction.exchanges.fullLocked(owner.streamExchangeLimitLocked()) {
		owner.mu.Unlock()
		return nil, nil, errors.New("text Introduction exchange owner unavailable")
	}
	lifetime, cancel := context.WithCancel(job.context)
	flight := owner.introduction.exchanges.addLocked(cancel)
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
		return owner.finishIntroductionExchangeLocked(flight, outcome)
	}, nil
}

// beginTextServiceTransportExchange gives an accepted JOIN a cleanup lifetime
// distinct from job authorization. The caller still cancels setup; after
// detach and retain, job loss stops Service work while the transport remains
// alive only long enough for its owner to send terminal control and join it.
func (owner *textContext) beginTextServiceTransportExchange(caller context.Context, job *textJobIdentity, surface broker.Surface) (context.Context, *introductionExchange, func() bool, func(error) error, error) {
	owner.mu.Lock()
	if caller == nil || caller.Err() != nil || !owner.liveTextServiceJobLocked(job, surface) || owner.introduction.exchanges.fullLocked(owner.streamExchangeLimitLocked()) {
		owner.mu.Unlock()
		return nil, nil, nil, nil, errors.New("text Introduction exchange owner unavailable")
	}
	lifetime, cancel := context.WithCancel(context.WithoutCancel(job.context))
	flight := owner.introduction.exchanges.addLocked(cancel)
	owner.mu.Unlock()
	interrupted := make(chan struct{})
	stop := context.AfterFunc(caller, func() { defer close(interrupted); cancel() })
	var joinCaller sync.Once
	detached := false
	detach := func() bool {
		joinCaller.Do(func() {
			if !stop() {
				<-interrupted
			}
			detached = caller.Err() == nil
		})
		return detached
	}
	finish := func(outcome error) error {
		cancel()
		if !detach() {
			outcome = errors.Join(outcome, caller.Err())
		}
		outcome = errors.Join(outcome, job.context.Err())
		owner.mu.Lock()
		defer owner.mu.Unlock()
		return owner.finishIntroductionExchangeLocked(flight, outcome)
	}
	return lifetime, flight, detach, finish, nil
}

func (owner *textContext) retainTextServiceTransportExchangeLocked(job *textJobIdentity, flight *introductionExchange) bool {
	if flight == nil || owner.closed || !owner.liveTextServiceJobLocked(job, owner.surface) {
		return false
	}
	return owner.introduction.exchanges.retainLocked(flight)
}

// finishIntroductionExchangeLocked publishes a failed Route cleanup before
// releasing the reservation, so both exchange paths terminalize Context alike.
func (owner *textContext) finishIntroductionExchangeLocked(flight *introductionExchange, outcome error) error {
	if errors.Is(outcome, client.ErrClosedSourceCleanup) {
		owner.closeErr = errors.Join(owner.closeErr, outcome)
		owner.closed = true
		owner.endpoint.failTextContexts(outcome)
	}
	owner.introduction.exchanges.removeLocked(flight)
	return outcome
}
