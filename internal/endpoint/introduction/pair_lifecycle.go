//go:build linux

package introduction

import (
	"context"
	"errors"
	"time"
)

// FlightRef is the seam to the endpoint root's operation and registration
// flights, which fuse a pair transition with the duty context lifetime.
// Identity comparisons between the retained reference and a caller's concrete
// flight stay exact through interface boxing.
type FlightRef interface {
	CancelFlight()
	JoinFlight()
}

// PairLifecycle owns local visibility of the current
// Publication/Registration pair, its in-flight opening, bounded predecessor,
// and drain barrier. Durable Publication and Instance storage remain with
// their existing owners; this lifecycle decides when the acknowledged pair
// is usable. The zero value is ready for use under dutyContext.mu.
type PairLifecycle struct {
	opening              FlightRef
	withdrawal           FlightRef
	registration         *Registration
	pendingRegistration  *Registration
	previousRegistration *Registration
	previousUntil        time.Time
	registrationChanged  chan struct{}
	publicationDraining  bool
}

// WithdrawalInProgressLocked reports a reserved withdrawal flight.
func (lifecycle *PairLifecycle) WithdrawalInProgressLocked() bool {
	return lifecycle != nil && lifecycle.withdrawal != nil
}

// WithdrawalLocked returns the reserved withdrawal flight for the retirement
// snapshot.
func (lifecycle *PairLifecycle) WithdrawalLocked() FlightRef {
	if lifecycle == nil {
		return nil
	}
	return lifecycle.withdrawal
}

// ReserveWithdrawalLocked admits exactly one withdrawal flight.
func (lifecycle *PairLifecycle) ReserveWithdrawalLocked(flight FlightRef) bool {
	if lifecycle == nil || flight == nil || lifecycle.withdrawal != nil {
		return false
	}
	lifecycle.withdrawal = flight
	return true
}

// FinishWithdrawalLocked clears exactly the reserved flight.
func (lifecycle *PairLifecycle) FinishWithdrawalLocked(flight FlightRef) {
	if lifecycle == nil || flight == nil || lifecycle.withdrawal != flight {
		return
	}
	lifecycle.withdrawal = nil
}

// OpeningInProgressLocked reports an in-flight registration opening.
func (lifecycle *PairLifecycle) OpeningInProgressLocked() bool {
	return lifecycle != nil && lifecycle.opening != nil
}

// OpeningLocked returns the retained opening flight for the retirement
// snapshot.
func (lifecycle *PairLifecycle) OpeningLocked() FlightRef {
	if lifecycle == nil {
		return nil
	}
	return lifecycle.opening
}

// BeginOpeningLocked admits exactly one registration opening over the expected
// predecessor. Another opening, an in-progress withdrawal, or a pair no longer
// based on previous refuses admission; on success the flight becomes the
// retained opening.
func (lifecycle *PairLifecycle) BeginOpeningLocked(flight FlightRef, previous *Registration) bool {
	if lifecycle == nil || flight == nil || lifecycle.opening != nil || lifecycle.withdrawal != nil ||
		!lifecycle.OpeningBaseLocked(previous) {
		return false
	}
	lifecycle.opening = flight
	return true
}

// EvictEndedTargetLocked removes the current publication target whose
// registration channel has already ended and returns it for caller-owned
// close and cancel. A withdrawal in progress retains the target for its own
// completion.
func (lifecycle *PairLifecycle) EvictEndedTargetLocked() *Registration {
	if lifecycle == nil || lifecycle.withdrawal != nil {
		return nil
	}
	current := lifecycle.PublicationTargetLocked()
	if current == nil || !current.Ended() {
		return nil
	}
	lifecycle.RemoveTargetLocked(current)
	return current
}

// CompleteWithdrawalLocked retires the withdrawn target and its bounded
// predecessor in one transition, clears the withdrawal flight, and returns the
// predecessor for caller-owned close. Signaling waiters stays with the Context.
func (lifecycle *PairLifecycle) CompleteWithdrawalLocked(flight FlightRef, registered *Registration) *Registration {
	if lifecycle == nil {
		return nil
	}
	lifecycle.RemoveTargetLocked(registered)
	previous := lifecycle.previousRegistration
	lifecycle.RemovePreviousLocked(previous)
	lifecycle.FinishWithdrawalLocked(flight)
	return previous
}

// OpeningCurrentLocked reports whether the flight is still the retained
// opening.
func (lifecycle *PairLifecycle) OpeningCurrentLocked(flight FlightRef) bool {
	return lifecycle != nil && flight != nil && lifecycle.opening == flight
}

// FinishOpeningLocked clears exactly the retained opening flight.
func (lifecycle *PairLifecycle) FinishOpeningLocked(flight FlightRef) bool {
	if !lifecycle.OpeningCurrentLocked(flight) {
		return false
	}
	lifecycle.opening = nil
	return true
}

// PairRetirement is one concrete snapshot of what the pair held at stop time.
// It contains no admission path.
type PairRetirement struct {
	current  *Registration
	pending  *Registration
	previous *Registration
}

// CurrentLocked returns the acknowledged current registration.
func (lifecycle *PairLifecycle) CurrentLocked() *Registration {
	if lifecycle == nil {
		return nil
	}
	return lifecycle.registration
}

// PublicationTargetLocked returns the registration that publication acts on:
// the pending successor while installed, else the acknowledged current.
func (lifecycle *PairLifecycle) PublicationTargetLocked() *Registration {
	if lifecycle == nil {
		return nil
	}
	if lifecycle.pendingRegistration != nil {
		return lifecycle.pendingRegistration
	}
	return lifecycle.registration
}

// OpeningBaseLocked reports whether the pair still rests on exactly previous
// with no pending successor.
func (lifecycle *PairLifecycle) OpeningBaseLocked(previous *Registration) bool {
	return lifecycle != nil && lifecycle.registration == previous && lifecycle.pendingRegistration == nil
}

// PreviousLocked returns the bounded predecessor and its retention instant.
func (lifecycle *PairLifecycle) PreviousLocked() (*Registration, time.Time) {
	if lifecycle == nil {
		return nil, time.Time{}
	}
	return lifecycle.previousRegistration, lifecycle.previousUntil
}

// SelectLocked returns the registration that owns a delivery identity: the
// retained predecessor inside its overlap window, else the current.
func (lifecycle *PairLifecycle) SelectLocked(now time.Time, slot [32]byte, revision uint64) *Registration {
	if lifecycle == nil {
		return nil
	}
	if lifecycle.previousRegistration != nil && now.Before(lifecycle.previousUntil) && lifecycle.previousRegistration.MatchesRequest(slot, revision) {
		return lifecycle.previousRegistration
	}
	return lifecycle.registration
}

// RetainedLocked reports whether the registration is the current or a
// predecessor still inside its overlap window.
func (lifecycle *PairLifecycle) RetainedLocked(registered *Registration, now time.Time) bool {
	return lifecycle != nil && registered != nil && (registered == lifecycle.registration || registered == lifecycle.previousRegistration && now.Before(lifecycle.previousUntil))
}

// DrainingLocked reports the explicit admission stop.
func (lifecycle *PairLifecycle) DrainingLocked() bool {
	return lifecycle != nil && lifecycle.publicationDraining
}

// BeginDrainLocked performs the one explicit admission-stop transition.
func (lifecycle *PairLifecycle) BeginDrainLocked() bool {
	if lifecycle == nil || lifecycle.publicationDraining {
		return false
	}
	lifecycle.publicationDraining = true
	return true
}

// InstallLocked admits a finished registration as the pending successor of
// exactly the expected predecessor.
func (lifecycle *PairLifecycle) InstallLocked(previous, registered *Registration) bool {
	if lifecycle == nil || registered == nil || lifecycle.publicationDraining || !lifecycle.OpeningBaseLocked(previous) {
		return false
	}
	lifecycle.pendingRegistration = registered
	return true
}

// CommitAcknowledgedLocked switches the pair to the verified-ACK registration
// and bounds its predecessor's overlap window.
func (lifecycle *PairLifecycle) CommitAcknowledgedLocked(ctx context.Context,
	registered *Registration, at time.Time) error {
	if lifecycle == nil || ctx == nil {
		return errors.New("text publication pair unavailable")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if lifecycle.publicationDraining || lifecycle.PublicationTargetLocked() != registered || registered == nil {
		return errors.New("text publication pair owner changed")
	}
	if registered.PublishedLocked() {
		return nil
	}
	predecessor := lifecycle.registration
	until := time.Time{}
	if predecessor != nil {
		switchAt := at.UTC().Truncate(time.Second)
		until = switchAt.Add(60 * time.Second)
		if predecessor.Expiry().Before(until) {
			until = predecessor.Expiry()
		}
		if switchAt.Before(until) {
			if err := predecessor.RetainPredecessorLocked(switchAt, until); err != nil {
				return err
			}
		}
	}
	// From the first predecessor mutation onward this owner completes the local
	// switch under dutyContext.mu. Cancellation and drain are admitted only
	// before that point, so they cannot strand a shortened predecessor beside
	// an uncommitted current registration.
	lifecycle.previousRegistration = predecessor
	lifecycle.previousUntil = until
	lifecycle.registration = registered
	lifecycle.pendingRegistration = nil
	registered.CommitPublicationLocked(at)
	return nil
}

// RemoveCurrentLocked drops exactly the acknowledged current registration.
func (lifecycle *PairLifecycle) RemoveCurrentLocked(registered *Registration) bool {
	if lifecycle == nil || lifecycle.registration != registered {
		return false
	}
	lifecycle.registration = nil
	return true
}

// RemoveTargetLocked drops exactly the pending successor or current.
func (lifecycle *PairLifecycle) RemoveTargetLocked(registered *Registration) bool {
	if lifecycle == nil || registered == nil {
		return false
	}
	if lifecycle.pendingRegistration == registered {
		lifecycle.pendingRegistration = nil
		return true
	}
	return lifecycle.RemoveCurrentLocked(registered)
}

// RemovePreviousLocked drops exactly the bounded predecessor.
func (lifecycle *PairLifecycle) RemovePreviousLocked(previous *Registration) bool {
	if lifecycle == nil || lifecycle.previousRegistration != previous {
		return false
	}
	lifecycle.previousRegistration = nil
	lifecycle.previousUntil = time.Time{}
	return true
}

// DetachLocked removes and returns every retained registration.
func (lifecycle *PairLifecycle) DetachLocked() (current, pending, previous *Registration) {
	if lifecycle == nil {
		return nil, nil, nil
	}
	current, pending, previous = lifecycle.registration, lifecycle.pendingRegistration, lifecycle.previousRegistration
	lifecycle.registration, lifecycle.pendingRegistration, lifecycle.previousRegistration = nil, nil, nil
	lifecycle.previousUntil = time.Time{}
	return current, pending, previous
}

// StopLocked cancels every retained registration and returns the retirement
// snapshot for the caller-owned join.
func (lifecycle *PairLifecycle) StopLocked() *PairRetirement {
	current, pending, previous := lifecycle.DetachLocked()
	for _, registration := range []*Registration{previous, current, pending} {
		if registration != nil {
			registration.Cancel()
		}
	}
	return &PairRetirement{current: current, pending: pending, previous: previous}
}

// Join closes every snapshot registration outside dutyContext.mu.
func (retirement *PairRetirement) Join() error {
	if retirement == nil {
		return nil
	}
	var outcome error
	for _, registration := range []*Registration{retirement.previous, retirement.current, retirement.pending} {
		if registration != nil {
			outcome = errors.Join(outcome, registration.Close())
		}
	}
	retirement.previous, retirement.current, retirement.pending = nil, nil, nil
	return outcome
}

// ChangedLocked returns the registration-change broadcast channel, replacing
// it on first use.
func (lifecycle *PairLifecycle) ChangedLocked() <-chan struct{} {
	if lifecycle.registrationChanged == nil {
		lifecycle.registrationChanged = make(chan struct{})
	}
	return lifecycle.registrationChanged
}

// SignalLocked wakes every registration-change waiter by closing and
// replacing the broadcast channel.
func (lifecycle *PairLifecycle) SignalLocked() {
	if lifecycle.registrationChanged != nil {
		close(lifecycle.registrationChanged)
	}
	lifecycle.registrationChanged = make(chan struct{})
}
