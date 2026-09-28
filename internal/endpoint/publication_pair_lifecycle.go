//go:build linux

package endpoint

import (
	"context"
	"errors"
	"time"
)

// publicationPairLifecycle owns local visibility of the current
// Publication/Registration pair, its in-flight opening, bounded predecessor,
// and drain barrier. Durable Publication and Instance storage remain with
// their existing owners; this lifecycle decides when the acknowledged pair
// is usable.
type publicationPairLifecycle struct {
	opening              *registrationFlight
	withdrawal           *textOperationFlight
	registration         *textIntroductionRegistration
	pendingRegistration  *textIntroductionRegistration
	previousRegistration *textIntroductionRegistration
	previousUntil        time.Time
	registrationChanged  chan struct{}
	publicationDraining  bool
}

func (lifecycle *publicationPairLifecycle) withdrawalInProgressLocked() bool {
	return lifecycle != nil && lifecycle.withdrawal != nil
}

func (lifecycle *publicationPairLifecycle) withdrawalLocked() *textOperationFlight {
	if lifecycle == nil {
		return nil
	}
	return lifecycle.withdrawal
}

func (lifecycle *publicationPairLifecycle) reserveWithdrawalLocked(flight *textOperationFlight) bool {
	if lifecycle == nil || flight == nil || lifecycle.withdrawal != nil {
		return false
	}
	lifecycle.withdrawal = flight
	return true
}

func (lifecycle *publicationPairLifecycle) finishWithdrawalLocked(flight *textOperationFlight) {
	if lifecycle == nil || flight == nil || lifecycle.withdrawal != flight {
		return
	}
	lifecycle.withdrawal = nil
}

func (lifecycle *publicationPairLifecycle) openingInProgressLocked() bool {
	return lifecycle != nil && lifecycle.opening != nil
}

func (lifecycle *publicationPairLifecycle) openingLocked() *registrationFlight {
	if lifecycle == nil {
		return nil
	}
	return lifecycle.opening
}

// beginOpeningLocked admits exactly one registration opening over the expected
// predecessor. Another opening, an in-progress withdrawal, or a pair no longer
// based on previous refuses admission; on success the flight becomes the
// retained opening.
func (lifecycle *publicationPairLifecycle) beginOpeningLocked(flight *registrationFlight, previous *textIntroductionRegistration) bool {
	if lifecycle == nil || flight == nil || lifecycle.opening != nil || lifecycle.withdrawal != nil ||
		!lifecycle.openingBaseLocked(previous) {
		return false
	}
	lifecycle.opening = flight
	return true
}

// evictEndedTargetLocked removes the current publication target whose
// registration channel has already ended and returns it for caller-owned
// close and cancel. A withdrawal in progress retains the target for its own
// completion.
func (lifecycle *publicationPairLifecycle) evictEndedTargetLocked() *textIntroductionRegistration {
	if lifecycle == nil || lifecycle.withdrawal != nil {
		return nil
	}
	current := lifecycle.publicationTargetLocked()
	if current == nil || !current.ended() {
		return nil
	}
	lifecycle.removeTargetLocked(current)
	return current
}

// completeWithdrawalLocked retires the withdrawn target and its bounded
// predecessor in one transition, clears the withdrawal flight, and returns the
// predecessor for caller-owned close. Signaling waiters stays with the Context.
func (lifecycle *publicationPairLifecycle) completeWithdrawalLocked(flight *textOperationFlight, registered *textIntroductionRegistration) *textIntroductionRegistration {
	if lifecycle == nil {
		return nil
	}
	lifecycle.removeTargetLocked(registered)
	previous := lifecycle.previousRegistration
	lifecycle.removePreviousLocked(previous)
	lifecycle.finishWithdrawalLocked(flight)
	return previous
}

func (lifecycle *publicationPairLifecycle) openingCurrentLocked(flight *registrationFlight) bool {
	return lifecycle != nil && flight != nil && lifecycle.opening == flight
}

func (lifecycle *publicationPairLifecycle) finishOpeningLocked(flight *registrationFlight) bool {
	if !lifecycle.openingCurrentLocked(flight) {
		return false
	}
	lifecycle.opening = nil
	return true
}

type publicationPairRetirement struct {
	current  *textIntroductionRegistration
	pending  *textIntroductionRegistration
	previous *textIntroductionRegistration
}

func (lifecycle *publicationPairLifecycle) currentLocked() *textIntroductionRegistration {
	if lifecycle == nil {
		return nil
	}
	return lifecycle.registration
}

func (lifecycle *publicationPairLifecycle) publicationTargetLocked() *textIntroductionRegistration {
	if lifecycle == nil {
		return nil
	}
	if lifecycle.pendingRegistration != nil {
		return lifecycle.pendingRegistration
	}
	return lifecycle.registration
}

func (lifecycle *publicationPairLifecycle) openingBaseLocked(previous *textIntroductionRegistration) bool {
	return lifecycle != nil && lifecycle.registration == previous && lifecycle.pendingRegistration == nil
}

func (lifecycle *publicationPairLifecycle) previousLocked() (*textIntroductionRegistration, time.Time) {
	if lifecycle == nil {
		return nil, time.Time{}
	}
	return lifecycle.previousRegistration, lifecycle.previousUntil
}

func (lifecycle *publicationPairLifecycle) selectLocked(now time.Time, slot [32]byte, revision uint64) *textIntroductionRegistration {
	if lifecycle == nil {
		return nil
	}
	if lifecycle.previousRegistration != nil && now.Before(lifecycle.previousUntil) && lifecycle.previousRegistration.matchesRequest(slot, revision) {
		return lifecycle.previousRegistration
	}
	return lifecycle.registration
}

func (lifecycle *publicationPairLifecycle) retainedLocked(registered *textIntroductionRegistration, now time.Time) bool {
	return lifecycle != nil && registered != nil && (registered == lifecycle.registration || registered == lifecycle.previousRegistration && now.Before(lifecycle.previousUntil))
}

func (lifecycle *publicationPairLifecycle) drainingLocked() bool {
	return lifecycle != nil && lifecycle.publicationDraining
}

func (lifecycle *publicationPairLifecycle) beginDrainLocked() bool {
	if lifecycle == nil || lifecycle.publicationDraining {
		return false
	}
	lifecycle.publicationDraining = true
	return true
}

func (lifecycle *publicationPairLifecycle) installLocked(previous, registered *textIntroductionRegistration) bool {
	if lifecycle == nil || registered == nil || lifecycle.publicationDraining || !lifecycle.openingBaseLocked(previous) {
		return false
	}
	lifecycle.pendingRegistration = registered
	return true
}

func (lifecycle *publicationPairLifecycle) commitAcknowledgedLocked(ctx context.Context,
	registered *textIntroductionRegistration, at time.Time) error {
	if lifecycle == nil || ctx == nil {
		return errors.New("text publication pair unavailable")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if lifecycle.publicationDraining || lifecycle.publicationTargetLocked() != registered || registered == nil {
		return errors.New("text publication pair owner changed")
	}
	if registered.publishedLocked() {
		return nil
	}
	predecessor := lifecycle.registration
	until := time.Time{}
	if predecessor != nil {
		switchAt := at.UTC().Truncate(time.Second)
		until = switchAt.Add(60 * time.Second)
		if predecessor.expiry().Before(until) {
			until = predecessor.expiry()
		}
		if switchAt.Before(until) {
			if err := predecessor.retainPredecessorLocked(switchAt, until); err != nil {
				return err
			}
		}
	}
	// From the first predecessor mutation onward this owner completes the local
	// switch under textContext.mu. Cancellation and drain are admitted only
	// before that point, so they cannot strand a shortened predecessor beside
	// an uncommitted current registration.
	lifecycle.previousRegistration = predecessor
	lifecycle.previousUntil = until
	lifecycle.registration = registered
	lifecycle.pendingRegistration = nil
	registered.commitPublicationLocked(at)
	return nil
}

func (lifecycle *publicationPairLifecycle) removeCurrentLocked(registered *textIntroductionRegistration) bool {
	if lifecycle == nil || lifecycle.registration != registered {
		return false
	}
	lifecycle.registration = nil
	return true
}

func (lifecycle *publicationPairLifecycle) removeTargetLocked(registered *textIntroductionRegistration) bool {
	if lifecycle == nil || registered == nil {
		return false
	}
	if lifecycle.pendingRegistration == registered {
		lifecycle.pendingRegistration = nil
		return true
	}
	return lifecycle.removeCurrentLocked(registered)
}

func (lifecycle *publicationPairLifecycle) removePreviousLocked(previous *textIntroductionRegistration) bool {
	if lifecycle == nil || lifecycle.previousRegistration != previous {
		return false
	}
	lifecycle.previousRegistration = nil
	lifecycle.previousUntil = time.Time{}
	return true
}

func (lifecycle *publicationPairLifecycle) detachLocked() (current, pending, previous *textIntroductionRegistration) {
	if lifecycle == nil {
		return nil, nil, nil
	}
	current, pending, previous = lifecycle.registration, lifecycle.pendingRegistration, lifecycle.previousRegistration
	lifecycle.registration, lifecycle.pendingRegistration, lifecycle.previousRegistration = nil, nil, nil
	lifecycle.previousUntil = time.Time{}
	return current, pending, previous
}

func (lifecycle *publicationPairLifecycle) stopLocked() *publicationPairRetirement {
	current, pending, previous := lifecycle.detachLocked()
	for _, registration := range []*textIntroductionRegistration{previous, current, pending} {
		if registration != nil {
			registration.cancel()
		}
	}
	return &publicationPairRetirement{current: current, pending: pending, previous: previous}
}

func (retirement *publicationPairRetirement) join() error {
	if retirement == nil {
		return nil
	}
	var outcome error
	for _, registration := range []*textIntroductionRegistration{retirement.previous, retirement.current, retirement.pending} {
		if registration != nil {
			outcome = errors.Join(outcome, registration.close())
		}
	}
	retirement.previous, retirement.current, retirement.pending = nil, nil, nil
	return outcome
}

func (lifecycle *publicationPairLifecycle) changedLocked() <-chan struct{} {
	if lifecycle.registrationChanged == nil {
		lifecycle.registrationChanged = make(chan struct{})
	}
	return lifecycle.registrationChanged
}

func (lifecycle *publicationPairLifecycle) signalLocked() {
	if lifecycle.registrationChanged != nil {
		close(lifecycle.registrationChanged)
	}
	lifecycle.registrationChanged = make(chan struct{})
}
