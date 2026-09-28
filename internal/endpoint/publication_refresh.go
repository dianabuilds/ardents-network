//go:build linux

package endpoint

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/route/client"
)

const refreshContentionRetryDelay = 100 * time.Millisecond

// publicationRefreshLifecycle is the sole owner of scheduler identity,
// wake-up, cancellation and its joined terminal result. The surrounding text
// context supplies the rotation callback but never publishes or replaces a
// flight directly.
type publicationRefreshLifecycle struct {
	mu      sync.Mutex
	flight  *publicationRefresh
	stopped bool
}

type publicationRefresh struct {
	context context.Context
	cancel  context.CancelFunc
	done    chan struct{}
	wake    chan struct{}
	err     error
}

type publicationRefreshRetirement struct {
	owner  *publicationRefreshLifecycle
	flight *publicationRefresh
}

func (lifecycle *publicationRefreshLifecycle) start(parent context.Context, run func(*publicationRefresh)) *publicationRefresh {
	lifecycle.mu.Lock()
	defer lifecycle.mu.Unlock()
	if lifecycle.stopped {
		return lifecycle.flight
	}
	if lifecycle.flight != nil {
		select {
		case <-lifecycle.flight.done:
			lifecycle.flight = nil
		default:
			lifecycle.wakeLocked()
			return lifecycle.flight
		}
	}
	ctx, cancel := context.WithCancel(parent)
	flight := &publicationRefresh{context: ctx, cancel: cancel, done: make(chan struct{}), wake: make(chan struct{}, 1)}
	lifecycle.flight = flight
	lifecycle.wakeLocked()
	go func() {
		run(flight)
		lifecycle.mu.Lock()
		if lifecycle.flight == flight {
			lifecycle.stopped = true
		}
		lifecycle.mu.Unlock()
		close(flight.done)
	}()
	return flight
}

func (lifecycle *publicationRefreshLifecycle) wake() {
	lifecycle.mu.Lock()
	defer lifecycle.mu.Unlock()
	lifecycle.wakeLocked()
}

func (lifecycle *publicationRefreshLifecycle) wakeLocked() {
	if lifecycle.flight == nil || lifecycle.flight.context.Err() != nil {
		return
	}
	select {
	case lifecycle.flight.wake <- struct{}{}:
	default:
	}
}

func (lifecycle *publicationRefreshLifecycle) current() *publicationRefresh {
	lifecycle.mu.Lock()
	defer lifecycle.mu.Unlock()
	return lifecycle.flight
}

func (lifecycle *publicationRefreshLifecycle) matchesContext(ctx context.Context) bool {
	lifecycle.mu.Lock()
	defer lifecycle.mu.Unlock()
	return lifecycle.flight != nil && lifecycle.flight.context == ctx
}

func (lifecycle *publicationRefreshLifecycle) cancel() *publicationRefresh {
	lifecycle.mu.Lock()
	defer lifecycle.mu.Unlock()
	lifecycle.stopped = true
	if lifecycle.flight != nil {
		lifecycle.flight.cancel()
	}
	return lifecycle.flight
}

func (lifecycle *publicationRefreshLifecycle) join(flight *publicationRefresh) error {
	if flight == nil {
		return nil
	}
	<-flight.done
	return lifecycle.outcome(flight)
}

func (lifecycle *publicationRefreshLifecycle) outcome(flight *publicationRefresh) error {
	lifecycle.mu.Lock()
	defer lifecycle.mu.Unlock()
	if flight == nil {
		return nil
	}
	return flight.err
}

func (lifecycle *publicationRefreshLifecycle) stop() error {
	return lifecycle.join(lifecycle.cancel())
}

func (lifecycle *publicationRefreshLifecycle) stopAsync() *publicationRefreshRetirement {
	return &publicationRefreshRetirement{owner: lifecycle, flight: lifecycle.cancel()}
}

func (retirement *publicationRefreshRetirement) join() error {
	if retirement == nil || retirement.owner == nil {
		return nil
	}
	outcome := retirement.owner.join(retirement.flight)
	retirement.flight = nil
	return outcome
}

func (lifecycle *publicationRefreshLifecycle) fail(flight *publicationRefresh, err error) bool {
	lifecycle.mu.Lock()
	defer lifecycle.mu.Unlock()
	if flight == nil || lifecycle.flight != flight {
		return false
	}
	flight.err = err
	return true
}

// refreshFailure retains a fixed, locally reportable stage while preserving
// the underlying error for the Endpoint's own terminal cleanup semantics.
type refreshFailure struct {
	stage string
	cause error
}

func (failure *refreshFailure) Error() string { return failure.cause.Error() }

func (failure *refreshFailure) Unwrap() error { return failure.cause }

func refreshFailureAt(stage string, cause error) error {
	return &refreshFailure{stage: stage, cause: cause}
}

func refreshFailureStage(cause error) string {
	var failure *refreshFailure
	if errors.As(cause, &failure) && failure.stage != "" {
		return failure.stage
	}
	return "rotation"
}

// Start once after a verified publication acknowledgement. Exact retries never
// move the original refresh time or renew the signed registration lifetime.
func (owner *dutyContext) startRefreshLocked(registered *introductionRegistration) {
	registered.scheduleRefreshLocked()
	owner.publication.refresh.start(owner.lease.Context(), owner.runRefresh)
}

func (owner *dutyContext) runRefresh(flight *publicationRefresh) {
	for {
		owner.mu.Lock()
		registered := owner.publication.pair.currentLocked()
		previous, until := owner.publication.pair.previousLocked()
		var refreshAt, expiry time.Time
		if registered != nil {
			refreshAt, expiry = registered.refreshScheduleLocked()
		}
		now := owner.endpoint.clock().UTC()
		live := owner.liveLocked(owner.endpoint, broker.Administration) && owner.publication.refresh.current() == flight && registered != nil
		owner.mu.Unlock()
		if !live || flight.context.Err() != nil {
			return
		}
		if !now.Before(expiry) {
			owner.failRefresh(flight, "registration-expired", errors.New("text publication registration expired"))
			return
		}
		if previous != nil && !now.Before(until) {
			previous.cancel()
			err := previous.close()
			owner.mu.Lock()
			if owner.publication.pair.removePreviousLocked(previous) {
				owner.publication.signalRegistrationsLocked()
			}
			owner.mu.Unlock()
			if err != nil {
				owner.failRefresh(flight, "predecessor-retirement", errors.Join(client.ErrClosedSourceCleanup, err))
				return
			}
			continue
		}
		if !now.Before(refreshAt) {
			if err := owner.rotatePublication(flight, registered); err != nil {
				if flight.context.Err() == nil && refreshSourceContention(err) {
					timer := time.NewTimer(refreshContentionRetryDelay)
					select {
					case <-flight.context.Done():
						timer.Stop()
						return
					case <-registered.doneSignal():
						timer.Stop()
						if flight.context.Err() == nil {
							owner.failRefresh(flight, registered.endedStage(), errors.New("text publication registration ended"))
						}
						return
					case <-flight.wake:
						timer.Stop()
					case <-timer.C:
					}
					continue
				}
				if flight.context.Err() == nil {
					owner.failRefresh(flight, refreshFailureStage(err), err)
				}
				return
			}
			continue
		}
		next := refreshAt
		if expiry.Before(next) {
			next = expiry
		}
		if previous != nil && until.Before(next) {
			next = until
		}
		timer := time.NewTimer(next.Sub(now))
		select {
		case <-flight.context.Done():
			timer.Stop()
			return
		case <-registered.doneSignal():
			timer.Stop()
			if flight.context.Err() == nil {
				owner.failRefresh(flight, registered.endedStage(), errors.New("text publication registration ended"))
			}
			return
		case <-flight.wake:
			timer.Stop()
		case <-timer.C:
		}
	}
}

func refreshSourceContention(cause error) bool {
	return errors.Is(cause, context.DeadlineExceeded) && roleMemberFailureStage(cause) == "conflict-read"
}

func (owner *dutyContext) rotatePublication(flight *publicationRefresh, previous *introductionRegistration) error {
	owner.mu.Lock()
	_, now, err := owner.permissionProfileLocked()
	retained, _ := owner.publication.pair.previousLocked()
	if err != nil || owner.publication.refresh.current() != flight || owner.publication.pair.currentLocked() != previous || retained != nil ||
		previous.revisionExhausted() || !owner.liveLocked(owner.endpoint, broker.Administration) {
		owner.mu.Unlock()
		return refreshFailureAt("rotation-authority", errors.New("text publication refresh owner unavailable"))
	}
	prefix := owner.introduction.prefix.currentLocked()
	owner.mu.Unlock()
	if prefix == nil {
		return refreshFailureAt("rotation-prefix", errors.New("text publication refresh prefix unavailable"))
	}
	if err := owner.prepareSourceReady(flight.context); err != nil {
		return refreshFailureAt("rotation-source-"+sourcePreparationFailureStage(err), err)
	}
	_, until, err := prefix.introductionRecipient()
	if err != nil {
		return refreshFailureAt("rotation-recipient", err)
	}
	expiry := now.UTC().Truncate(time.Second).Add(600 * time.Second)
	if until.Before(expiry) {
		expiry = until
	}
	if !now.Before(expiry) {
		return refreshFailureAt("rotation-expired", errors.New("text publication refresh expired"))
	}
	if _, err := owner.openRegistration(flight.context, previous.nextRevision(), expiry, previous); err != nil {
		return refreshFailureAt("rotation-registration", err)
	}
	_, err = owner.publishDescriptor(flight.context)
	if err != nil {
		return refreshFailureAt("rotation-publication", err)
	}
	return nil
}

// Failed refresh removes accepting registration readiness. It does not grant
// a fallback to an older revision or erase a failed transport cleanup outcome.
func (owner *dutyContext) failRefresh(flight *publicationRefresh, failure string, cause error) {
	owner.mu.Lock()
	if owner.publication.refresh.current() != flight {
		owner.mu.Unlock()
		return
	}
	current, pending, previous := owner.publication.pair.detachLocked()
	report := owner.publication.refreshFailure
	owner.publication.signalRegistrationsLocked()
	owner.mu.Unlock()
	if report != nil {
		report(failure)
	}
	var cleanup error
	if errors.Is(cause, client.ErrClosedSourceCleanup) {
		cleanup = cause
	}
	for _, registered := range []*introductionRegistration{current, pending, previous} {
		if registered != nil {
			registered.cancel()
			cleanup = errors.Join(cleanup, registered.close())
		}
	}
	owner.mu.Lock()
	owner.publication.refresh.fail(flight, errors.Join(cause, cleanup))
	if cleanup != nil {
		owner.closeErr = errors.Join(owner.closeErr, cleanup)
		owner.closed = true
		owner.endpoint.failDutyContexts(cleanup)
	}
	owner.mu.Unlock()
}
