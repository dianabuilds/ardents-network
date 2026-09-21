//go:build linux

package endpoint

import (
	"context"
	"errors"
	"time"
)

// textPublicationPairLifecycle owns local visibility of the current
// Publication/Registration pair, its bounded predecessor, and the drain
// barrier. Durable Publication and Instance storage remain with their existing
// owners; this lifecycle decides only when their acknowledged pair is usable.
type textPublicationPairLifecycle struct {
	registration         *textIntroductionRegistration
	pendingRegistration  *textIntroductionRegistration
	previousRegistration *textIntroductionRegistration
	previousUntil        time.Time
	registrationChanged  chan struct{}
	publicationDraining  bool
}

func (lifecycle *textPublicationPairLifecycle) currentLocked() *textIntroductionRegistration {
	if lifecycle == nil {
		return nil
	}
	return lifecycle.registration
}

func (lifecycle *textPublicationPairLifecycle) publicationTargetLocked() *textIntroductionRegistration {
	if lifecycle == nil {
		return nil
	}
	if lifecycle.pendingRegistration != nil {
		return lifecycle.pendingRegistration
	}
	return lifecycle.registration
}

func (lifecycle *textPublicationPairLifecycle) openingBaseLocked(previous *textIntroductionRegistration) bool {
	return lifecycle != nil && lifecycle.registration == previous && lifecycle.pendingRegistration == nil
}

func (lifecycle *textPublicationPairLifecycle) previousLocked() (*textIntroductionRegistration, time.Time) {
	if lifecycle == nil {
		return nil, time.Time{}
	}
	return lifecycle.previousRegistration, lifecycle.previousUntil
}

func (lifecycle *textPublicationPairLifecycle) selectLocked(now time.Time, slot [32]byte, revision uint64) *textIntroductionRegistration {
	if lifecycle == nil {
		return nil
	}
	if lifecycle.previousRegistration != nil && now.Before(lifecycle.previousUntil) && lifecycle.previousRegistration.request.Slot == slot && lifecycle.previousRegistration.request.Revision == revision {
		return lifecycle.previousRegistration
	}
	return lifecycle.registration
}

func (lifecycle *textPublicationPairLifecycle) retainedLocked(registered *textIntroductionRegistration, now time.Time) bool {
	return lifecycle != nil && registered != nil && (registered == lifecycle.registration || registered == lifecycle.previousRegistration && now.Before(lifecycle.previousUntil))
}

func (lifecycle *textPublicationPairLifecycle) drainingLocked() bool {
	return lifecycle != nil && lifecycle.publicationDraining
}

func (lifecycle *textPublicationPairLifecycle) beginDrainLocked() bool {
	if lifecycle == nil || lifecycle.publicationDraining {
		return false
	}
	lifecycle.publicationDraining = true
	return true
}

func (lifecycle *textPublicationPairLifecycle) installLocked(previous, registered *textIntroductionRegistration) bool {
	if lifecycle == nil || registered == nil || lifecycle.publicationDraining || !lifecycle.openingBaseLocked(previous) {
		return false
	}
	lifecycle.pendingRegistration = registered
	return true
}

func (lifecycle *textPublicationPairLifecycle) commitAcknowledgedLocked(ctx context.Context,
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
	if registered.published {
		return nil
	}
	predecessor := lifecycle.registration
	until := time.Time{}
	if predecessor != nil {
		switchAt := at.UTC().Truncate(time.Second)
		until = switchAt.Add(60 * time.Second)
		if predecessor.request.Expiry.Before(until) {
			until = predecessor.request.Expiry
		}
		if switchAt.Before(until) {
			if predecessor.recipient == nil {
				return errors.New("text publication predecessor recipient unavailable")
			}
			if err := predecessor.recipient.RetainPredecessor(switchAt, until); err != nil {
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
	registered.publishedAt = at
	registered.published = true
	return nil
}

func (lifecycle *textPublicationPairLifecycle) removeCurrentLocked(registered *textIntroductionRegistration) bool {
	if lifecycle == nil || lifecycle.registration != registered {
		return false
	}
	lifecycle.registration = nil
	return true
}

func (lifecycle *textPublicationPairLifecycle) removeTargetLocked(registered *textIntroductionRegistration) bool {
	if lifecycle == nil || registered == nil {
		return false
	}
	if lifecycle.pendingRegistration == registered {
		lifecycle.pendingRegistration = nil
		return true
	}
	return lifecycle.removeCurrentLocked(registered)
}

func (lifecycle *textPublicationPairLifecycle) removePreviousLocked(previous *textIntroductionRegistration) bool {
	if lifecycle == nil || lifecycle.previousRegistration != previous {
		return false
	}
	lifecycle.previousRegistration = nil
	lifecycle.previousUntil = time.Time{}
	return true
}

func (lifecycle *textPublicationPairLifecycle) detachLocked() (current, pending, previous *textIntroductionRegistration) {
	if lifecycle == nil {
		return nil, nil, nil
	}
	current, pending, previous = lifecycle.registration, lifecycle.pendingRegistration, lifecycle.previousRegistration
	lifecycle.registration, lifecycle.pendingRegistration, lifecycle.previousRegistration = nil, nil, nil
	lifecycle.previousUntil = time.Time{}
	return current, pending, previous
}

func (lifecycle *textPublicationPairLifecycle) changedLocked() <-chan struct{} {
	if lifecycle.registrationChanged == nil {
		lifecycle.registrationChanged = make(chan struct{})
	}
	return lifecycle.registrationChanged
}

func (lifecycle *textPublicationPairLifecycle) signalLocked() {
	if lifecycle.registrationChanged != nil {
		close(lifecycle.registrationChanged)
	}
	lifecycle.registrationChanged = make(chan struct{})
}
