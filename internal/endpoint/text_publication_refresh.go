//go:build linux

package endpoint

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/route"
)

const textRefreshContentionRetryDelay = 100 * time.Millisecond

// textPublicationRefreshLifecycle is the sole owner of scheduler identity,
// wake-up, cancellation and its joined terminal result. The surrounding text
// context supplies the rotation callback but never publishes or replaces a
// flight directly.
type textPublicationRefreshLifecycle struct {
	mu      sync.Mutex
	flight  *textPublicationRefresh
	stopped bool
}

type textPublicationRefresh struct {
	context context.Context
	cancel  context.CancelFunc
	done    chan struct{}
	wake    chan struct{}
	err     error
}

func (lifecycle *textPublicationRefreshLifecycle) start(parent context.Context, run func(*textPublicationRefresh)) *textPublicationRefresh {
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
	flight := &textPublicationRefresh{context: ctx, cancel: cancel, done: make(chan struct{}), wake: make(chan struct{}, 1)}
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

func (lifecycle *textPublicationRefreshLifecycle) wake() {
	lifecycle.mu.Lock()
	defer lifecycle.mu.Unlock()
	lifecycle.wakeLocked()
}

func (lifecycle *textPublicationRefreshLifecycle) wakeLocked() {
	if lifecycle.flight == nil || lifecycle.flight.context.Err() != nil {
		return
	}
	select {
	case lifecycle.flight.wake <- struct{}{}:
	default:
	}
}

func (lifecycle *textPublicationRefreshLifecycle) current() *textPublicationRefresh {
	lifecycle.mu.Lock()
	defer lifecycle.mu.Unlock()
	return lifecycle.flight
}

func (lifecycle *textPublicationRefreshLifecycle) matchesContext(ctx context.Context) bool {
	lifecycle.mu.Lock()
	defer lifecycle.mu.Unlock()
	return lifecycle.flight != nil && lifecycle.flight.context == ctx
}

func (lifecycle *textPublicationRefreshLifecycle) cancel() *textPublicationRefresh {
	lifecycle.mu.Lock()
	defer lifecycle.mu.Unlock()
	lifecycle.stopped = true
	if lifecycle.flight != nil {
		lifecycle.flight.cancel()
	}
	return lifecycle.flight
}

func (lifecycle *textPublicationRefreshLifecycle) join(flight *textPublicationRefresh) error {
	if flight == nil {
		return nil
	}
	<-flight.done
	return lifecycle.outcome(flight)
}

func (lifecycle *textPublicationRefreshLifecycle) outcome(flight *textPublicationRefresh) error {
	lifecycle.mu.Lock()
	defer lifecycle.mu.Unlock()
	if flight == nil {
		return nil
	}
	return flight.err
}

func (lifecycle *textPublicationRefreshLifecycle) stop() error {
	return lifecycle.join(lifecycle.cancel())
}

func (lifecycle *textPublicationRefreshLifecycle) fail(flight *textPublicationRefresh, err error) bool {
	lifecycle.mu.Lock()
	defer lifecycle.mu.Unlock()
	if flight == nil || lifecycle.flight != flight {
		return false
	}
	flight.err = err
	return true
}

// textRefreshFailure retains a fixed, locally reportable stage while preserving
// the underlying error for the Endpoint's own terminal cleanup semantics.
type textRefreshFailure struct {
	stage string
	cause error
}

func (failure *textRefreshFailure) Error() string { return failure.cause.Error() }

func (failure *textRefreshFailure) Unwrap() error { return failure.cause }

func textRefreshFailureAt(stage string, cause error) error {
	return &textRefreshFailure{stage: stage, cause: cause}
}

func textRefreshFailureStage(cause error) string {
	var failure *textRefreshFailure
	if errors.As(cause, &failure) && failure.stage != "" {
		return failure.stage
	}
	return "rotation"
}

// Start once after a verified publication acknowledgement. Exact retries never
// move the original refresh time or renew the signed registration lifetime.
func (owner *textContext) startTextRefreshLocked(registered *textIntroductionRegistration) {
	if registered.refreshAt.IsZero() {
		registered.refreshAt = registered.createdAt.Add(300 * time.Second)
	}
	owner.refresh.start(owner.lease.Context(), owner.runTextRefresh)
}
func (owner *textContext) signalTextRegistrationsLocked() {
	owner.textPublicationPairLifecycle.signalLocked()
	owner.refresh.wake()
}
func (owner *textContext) stopTextRefresh() {
	_ = owner.refresh.stop()
}

func (owner *textContext) runTextRefresh(flight *textPublicationRefresh) {
	for {
		owner.mu.Lock()
		registered := owner.textPublicationPairLifecycle.currentLocked()
		previous, until := owner.textPublicationPairLifecycle.previousLocked()
		var refreshAt, expiry time.Time
		if registered != nil {
			refreshAt, expiry = registered.refreshAt, registered.request.Expiry
		}
		now := owner.endpoint.clock().UTC()
		live := owner.liveLocked(owner.endpoint, broker.Administration) && owner.refresh.current() == flight && registered != nil
		owner.mu.Unlock()
		if !live || flight.context.Err() != nil {
			return
		}
		if !now.Before(expiry) {
			owner.failTextRefresh(flight, "registration-expired", errors.New("text publication registration expired"))
			return
		}
		if previous != nil && !now.Before(until) {
			previous.cancel()
			err := previous.close()
			owner.mu.Lock()
			if owner.textPublicationPairLifecycle.removePreviousLocked(previous) {
				owner.signalTextRegistrationsLocked()
			}
			owner.mu.Unlock()
			if err != nil {
				owner.failTextRefresh(flight, "predecessor-retirement", errors.Join(route.ErrClosedSourceCleanup, err))
				return
			}
			continue
		}
		if !now.Before(refreshAt) {
			if err := owner.rotateTextPublication(flight, registered); err != nil {
				if flight.context.Err() == nil && textRefreshSourceContention(err) {
					timer := time.NewTimer(textRefreshContentionRetryDelay)
					select {
					case <-flight.context.Done():
						timer.Stop()
						return
					case <-registered.channel.Done():
						timer.Stop()
						if flight.context.Err() == nil {
							owner.failTextRefresh(flight, "registration-ended-"+string(registered.channel.EndReason()), errors.New("text publication registration ended"))
						}
						return
					case <-flight.wake:
						timer.Stop()
					case <-timer.C:
					}
					continue
				}
				if flight.context.Err() == nil {
					owner.failTextRefresh(flight, textRefreshFailureStage(err), err)
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
		case <-registered.channel.Done():
			timer.Stop()
			if flight.context.Err() == nil {
				owner.failTextRefresh(flight, "registration-ended-"+string(registered.channel.EndReason()), errors.New("text publication registration ended"))
			}
			return
		case <-flight.wake:
			timer.Stop()
		case <-timer.C:
		}
	}
}

func textRefreshSourceContention(cause error) bool {
	return errors.Is(cause, context.DeadlineExceeded) && textRoleMemberFailureStage(cause) == "conflict-read"
}

func (owner *textContext) rotateTextPublication(flight *textPublicationRefresh, previous *textIntroductionRegistration) error {
	owner.mu.Lock()
	_, now, err := owner.textPermissionProfileLocked()
	retained, _ := owner.textPublicationPairLifecycle.previousLocked()
	if err != nil || owner.refresh.current() != flight || owner.textPublicationPairLifecycle.currentLocked() != previous || retained != nil ||
		previous.request.Revision == ^uint64(0) || !owner.liveLocked(owner.endpoint, broker.Administration) {
		owner.mu.Unlock()
		return textRefreshFailureAt("rotation-authority", errors.New("text publication refresh owner unavailable"))
	}
	prefix := owner.introduction.currentLocked()
	owner.mu.Unlock()
	if prefix == nil {
		return textRefreshFailureAt("rotation-prefix", errors.New("text publication refresh prefix unavailable"))
	}
	if err := owner.prepareTextSourceReady(flight.context); err != nil {
		return textRefreshFailureAt("rotation-source-"+textSourcePreparationFailureStage(err), err)
	}
	_, until, err := prefix.introductionRecipient()
	if err != nil {
		return textRefreshFailureAt("rotation-recipient", err)
	}
	expiry := now.UTC().Truncate(time.Second).Add(600 * time.Second)
	if until.Before(expiry) {
		expiry = until
	}
	if !now.Before(expiry) {
		return textRefreshFailureAt("rotation-expired", errors.New("text publication refresh expired"))
	}
	if _, err := owner.openTextRegistration(flight.context, previous.request.Revision+1, expiry, previous); err != nil {
		return textRefreshFailureAt("rotation-registration", err)
	}
	_, err = owner.publishTextDescriptor(flight.context)
	if err != nil {
		return textRefreshFailureAt("rotation-publication", err)
	}
	return nil
}

// Failed refresh removes accepting registration readiness. It does not grant
// a fallback to an older revision or erase a failed transport cleanup outcome.
func (owner *textContext) failTextRefresh(flight *textPublicationRefresh, failure string, cause error) {
	owner.mu.Lock()
	if owner.refresh.current() != flight {
		owner.mu.Unlock()
		return
	}
	current, pending, previous := owner.textPublicationPairLifecycle.detachLocked()
	report := owner.refreshFailure
	owner.signalTextRegistrationsLocked()
	owner.mu.Unlock()
	if report != nil {
		report(failure)
	}
	var cleanup error
	if errors.Is(cause, route.ErrClosedSourceCleanup) {
		cleanup = cause
	}
	for _, registered := range []*textIntroductionRegistration{current, pending, previous} {
		if registered != nil {
			registered.cancel()
			cleanup = errors.Join(cleanup, registered.close())
		}
	}
	owner.mu.Lock()
	owner.refresh.fail(flight, errors.Join(cause, cleanup))
	if cleanup != nil {
		owner.closeErr = errors.Join(owner.closeErr, cleanup)
		owner.closed = true
		owner.endpoint.failTextContexts(cleanup)
	}
	owner.mu.Unlock()
}
