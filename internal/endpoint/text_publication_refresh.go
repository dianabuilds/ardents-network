//go:build linux

package endpoint

import (
	"context"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/route"
)

type textPublicationRefresh struct {
	context context.Context
	cancel  context.CancelFunc
	done    chan struct{}
	wake    chan struct{}
	err     error
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
	if owner.refresh != nil {
		select {
		case <-owner.refresh.done:
			owner.refresh = nil
		default:
		}
	}
	if owner.refresh == nil {
		ctx, cancel := context.WithCancel(owner.lease.Context())
		flight := &textPublicationRefresh{context: ctx, cancel: cancel, done: make(chan struct{}), wake: make(chan struct{}, 1)}
		owner.refresh = flight
		go owner.runTextRefresh(flight)
	}
	select {
	case owner.refresh.wake <- struct{}{}:
	default:
	}
}
func (owner *textContext) signalTextRegistrationsLocked() {
	if owner.registrationChanged != nil {
		close(owner.registrationChanged)
	}
	owner.registrationChanged = make(chan struct{})
	if owner.refresh != nil {
		select {
		case owner.refresh.wake <- struct{}{}:
		default:
		}
	}
}
func (owner *textContext) stopTextRefresh() {
	owner.mu.Lock()
	flight := owner.refresh
	if flight != nil {
		flight.cancel()
	}
	owner.mu.Unlock()
	if flight != nil {
		<-flight.done
	}
	owner.mu.Lock()
	if owner.refresh == flight {
		owner.refresh = nil
	}
	owner.mu.Unlock()
}

func (owner *textContext) runTextRefresh(flight *textPublicationRefresh) {
	defer close(flight.done)
	for {
		owner.mu.Lock()
		registered, previous := owner.registration, owner.previousRegistration
		until := owner.previousUntil
		var refreshAt, expiry time.Time
		if registered != nil {
			refreshAt, expiry = registered.refreshAt, registered.request.Expiry
		}
		now := owner.endpoint.clock().UTC()
		live := owner.liveLocked(owner.endpoint, broker.Administration) && owner.refresh == flight && registered != nil
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
			if owner.previousRegistration == previous {
				owner.previousRegistration = nil
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
				owner.failTextRefresh(flight, "registration-ended", errors.New("text publication registration ended"))
			}
			return
		case <-flight.wake:
			timer.Stop()
		case <-timer.C:
		}
	}
}

func (owner *textContext) rotateTextPublication(flight *textPublicationRefresh, previous *textIntroductionRegistration) error {
	owner.mu.Lock()
	_, now, err := owner.textPermissionProfileLocked()
	if err != nil || owner.refresh != flight || owner.registration != previous || owner.previousRegistration != nil ||
		previous.request.Revision == ^uint64(0) || !owner.liveLocked(owner.endpoint, broker.Administration) {
		owner.mu.Unlock()
		return textRefreshFailureAt("rotation-authority", errors.New("text publication refresh owner unavailable"))
	}
	prefix := owner.introduction.prefix
	owner.mu.Unlock()
	if prefix == nil {
		return textRefreshFailureAt("rotation-prefix", errors.New("text publication refresh prefix unavailable"))
	}
	if err := owner.prepareTextSourceReady(flight.context); err != nil {
		return textRefreshFailureAt("rotation-source", err)
	}
	_, until, err := prefix.IntroductionRecipient()
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
	if owner.refresh != flight {
		owner.mu.Unlock()
		return
	}
	current, previous, report := owner.registration, owner.previousRegistration, owner.refreshFailure
	owner.registration, owner.previousRegistration = nil, nil
	owner.signalTextRegistrationsLocked()
	owner.mu.Unlock()
	if report != nil {
		report(failure)
	}
	var cleanup error
	if errors.Is(cause, route.ErrClosedSourceCleanup) {
		cleanup = cause
	}
	for _, registered := range []*textIntroductionRegistration{current, previous} {
		if registered != nil {
			registered.cancel()
			cleanup = errors.Join(cleanup, registered.close())
		}
	}
	owner.mu.Lock()
	flight.err = errors.Join(cause, cleanup)
	if cleanup != nil {
		owner.closeErr = errors.Join(owner.closeErr, cleanup)
		owner.closed = true
		owner.endpoint.failTextContexts(cleanup)
	}
	owner.mu.Unlock()
}
