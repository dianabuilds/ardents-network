package state

import (
	"errors"
	networkdomain "github.com/dianabuilds/ardents-network/internal/successor/network"
	"github.com/dianabuilds/ardents-network/internal/successor/network/state/durable"
)

// closedRuntimeAvailableLocked requires s.mu and performs no time observation.
func (s *networkState) closedRuntimeAvailableLocked() error {
	if s.closed || s.current == nil || s.current.Snapshot.Profile != closedRouteProfile || s.distribution.conflicting {
		return errors.New("closed profile is unavailable")
	}
	if err := errors.Join(s.automaticErr, s.terminalErr); err != nil {
		return err
	}
	if s.config.observe == nil {
		return errClockUncertain
	}
	return nil
}

// currentMembershipLocked requires s.mu. Loading is explicit so failure and
// expiry at the durable read boundary can be tested without clock-call counts.
func (s *networkState) currentMembershipLocked(load func([32]byte) (durable.ClosedProfileState, []byte, error)) (membership, networkdomain.TrustedTime, error) {
	if err := s.closedRuntimeAvailableLocked(); err != nil {
		return membership{}, networkdomain.TrustedTime{}, err
	}
	generation, err := closedProfileGeneration(s.current.Snapshot.Generation)
	if err != nil {
		return membership{}, networkdomain.TrustedTime{}, err
	}
	stored, raw, err := load(generation)
	if err != nil || stored == (durable.ClosedProfileState{}) || stored.Conflict != [32]byte{} || stored.Epoch != s.current.Snapshot.Epoch {
		return membership{}, networkdomain.TrustedTime{}, errors.New("closed profile is unavailable")
	}
	// Observe once after durable I/O, so a slow read cannot extend validity.
	observed, err := trustedObservation(s.config, s.distribution)
	if err != nil {
		return membership{}, networkdomain.TrustedTime{}, err
	}
	now := observed.Instant()
	profile, err := s.verifyClosedProfileLocked(raw, generation, now)
	if err != nil || profile.Digest != stored.Accepted {
		return membership{}, networkdomain.TrustedTime{}, errors.New("closed profile is unavailable")
	}
	bound, err := s.bindCurrentMembership(profile)
	if err == nil {
		_, err = bound.accepted.Observe(observed)
	}
	return bound, observed, err
}
