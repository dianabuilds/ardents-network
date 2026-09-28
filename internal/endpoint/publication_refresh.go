//go:build linux

package endpoint

import (
	"context"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/endpoint/publication"
	"github.com/dianabuilds/ardents-network/internal/endpoint/source"
	"github.com/dianabuilds/ardents-network/internal/route/client"
)

// Start once after a verified publication acknowledgement. Exact retries never
// move the original refresh time or renew the signed registration lifetime.
func (owner *dutyContext) startRefreshLocked(registered *introductionRegistration) {
	registered.scheduleRefreshLocked()
	owner.publication.refresh.Start(owner.lease.Context(), owner.runRefresh)
}

func (owner *dutyContext) runRefresh(flight *publication.Refresh) {
	for {
		owner.mu.Lock()
		registered := owner.publication.pair.currentLocked()
		previous, until := owner.publication.pair.previousLocked()
		var refreshAt, expiry time.Time
		if registered != nil {
			refreshAt, expiry = registered.refreshScheduleLocked()
		}
		now := owner.endpoint.clock().UTC()
		live := owner.liveLocked(owner.endpoint, broker.Administration) && owner.publication.refresh.Current() == flight && registered != nil
		owner.mu.Unlock()
		if !live || flight.Context.Err() != nil {
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
				if flight.Context.Err() == nil && refreshSourceContention(err) {
					timer := time.NewTimer(publication.RefreshContentionRetryDelay)
					select {
					case <-flight.Context.Done():
						timer.Stop()
						return
					case <-registered.doneSignal():
						timer.Stop()
						if flight.Context.Err() == nil {
							owner.failRefresh(flight, registered.endedStage(), errors.New("text publication registration ended"))
						}
						return
					case <-flight.Wake:
						timer.Stop()
					case <-timer.C:
					}
					continue
				}
				if flight.Context.Err() == nil {
					owner.failRefresh(flight, publication.RefreshFailureStage(err), err)
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
		case <-flight.Context.Done():
			timer.Stop()
			return
		case <-registered.doneSignal():
			timer.Stop()
			if flight.Context.Err() == nil {
				owner.failRefresh(flight, registered.endedStage(), errors.New("text publication registration ended"))
			}
			return
		case <-flight.Wake:
			timer.Stop()
		case <-timer.C:
		}
	}
}

func refreshSourceContention(cause error) bool {
	return errors.Is(cause, context.DeadlineExceeded) && roleMemberFailureStage(cause) == "conflict-read"
}

func (owner *dutyContext) rotatePublication(flight *publication.Refresh, previous *introductionRegistration) error {
	owner.mu.Lock()
	_, now, err := owner.permissionProfileLocked()
	retained, _ := owner.publication.pair.previousLocked()
	if err != nil || owner.publication.refresh.Current() != flight || owner.publication.pair.currentLocked() != previous || retained != nil ||
		previous.revisionExhausted() || !owner.liveLocked(owner.endpoint, broker.Administration) {
		owner.mu.Unlock()
		return publication.RefreshFailureAt("rotation-authority", errors.New("text publication refresh owner unavailable"))
	}
	prefix := owner.introduction.prefix.currentLocked()
	owner.mu.Unlock()
	if prefix == nil {
		return publication.RefreshFailureAt("rotation-prefix", errors.New("text publication refresh prefix unavailable"))
	}
	if err := owner.prepareSourceReady(flight.Context); err != nil {
		return publication.RefreshFailureAt("rotation-source-"+source.PreparationFailureStage(err), err)
	}
	_, until, err := prefix.introductionRecipient()
	if err != nil {
		return publication.RefreshFailureAt("rotation-recipient", err)
	}
	expiry := now.UTC().Truncate(time.Second).Add(600 * time.Second)
	if until.Before(expiry) {
		expiry = until
	}
	if !now.Before(expiry) {
		return publication.RefreshFailureAt("rotation-expired", errors.New("text publication refresh expired"))
	}
	if _, err := owner.openRegistration(flight.Context, previous.nextRevision(), expiry, previous); err != nil {
		return publication.RefreshFailureAt("rotation-registration", err)
	}
	_, err = owner.publishDescriptor(flight.Context)
	if err != nil {
		return publication.RefreshFailureAt("rotation-publication", err)
	}
	return nil
}

// Failed refresh removes accepting registration readiness. It does not grant
// a fallback to an older revision or erase a failed transport cleanup outcome.
func (owner *dutyContext) failRefresh(flight *publication.Refresh, failure string, cause error) {
	owner.mu.Lock()
	if owner.publication.refresh.Current() != flight {
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
	owner.publication.refresh.Fail(flight, errors.Join(cause, cleanup))
	if cleanup != nil {
		owner.closeErr = errors.Join(owner.closeErr, cleanup)
		owner.closed = true
		owner.endpoint.failDutyContexts(cleanup)
	}
	owner.mu.Unlock()
}
