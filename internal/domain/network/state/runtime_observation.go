package state

import (
	"errors"

	networkdomain "github.com/dianabuilds/ardents-network/internal/domain/network"
	"github.com/dianabuilds/ardents-network/internal/domain/network/state/durable"
)

// CurrentRuntime returns one authenticated, bounded Network observation.
// The configured PermitWork runs before State locks and durable authority I/O.
// Profile, generation, membership and trusted time share the same read lock;
// trusted time is sampled after the durable profile read. It grants no lease
// on future State, no token authority and no reservation. The caller reobserves
// at its operation's admission points, outside every State lock.
func (s *State) CurrentRuntime() (networkdomain.RuntimeView, error) {
	witness, err := s.CurrentRuntimeWitness()
	if err != nil {
		return networkdomain.RuntimeView{}, err
	}
	return witness.View(), nil
}

// CurrentRuntimeWitness performs the same complete observation and additionally
// retains its original local refusal projection for a later bounded handoff.
// Fresh observations remain mandatory; this does not lend authority or storage.
func (s *State) CurrentRuntimeWitness() (RuntimeWitness, error) {
	if s.config.permitWork != nil {
		if err := s.config.permitWork(); err != nil {
			s.invalidateRuntime()
			return RuntimeWitness{}, err
		}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, _, view, err := s.currentMembershipLocked(s.storage.LoadClosedProfile)
	if err != nil {
		s.invalidateRuntime()
		return RuntimeWitness{}, err
	}
	return s.witnessLocked(view), nil
}

// closedRuntimeAvailableLocked requires s.mu and performs no time observation.
func (s *State) closedRuntimeAvailableLocked() error {
	if s.closed || s.current == nil || s.current.Snapshot.Profile != closedRouteProfile || s.distribution.Conflicting {
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
// It returns the observed view so the caller reuses a single observation rather
// than observing the same immutable state twice (audit NW-2).
func (s *State) currentMembershipLocked(load func([32]byte) (durable.ClosedProfileState, []byte, error)) (membership, networkdomain.TrustedTime, networkdomain.RuntimeView, error) {
	if err := s.closedRuntimeAvailableLocked(); err != nil {
		return membership{}, networkdomain.TrustedTime{}, networkdomain.RuntimeView{}, err
	}
	generation, err := closedProfileGeneration(s.current.Snapshot.Generation)
	if err != nil {
		return membership{}, networkdomain.TrustedTime{}, networkdomain.RuntimeView{}, err
	}
	stored, raw, err := load(generation)
	if err != nil || stored == (durable.ClosedProfileState{}) || stored.Conflict != [32]byte{} || stored.Epoch != s.current.Snapshot.Epoch {
		return membership{}, networkdomain.TrustedTime{}, networkdomain.RuntimeView{}, errors.New("closed profile is unavailable")
	}
	// Observe once after durable I/O, so a slow read cannot extend validity.
	observed, err := trustedObservation(s.config, s.distribution)
	if err != nil {
		return membership{}, networkdomain.TrustedTime{}, networkdomain.RuntimeView{}, err
	}
	now := observed.Instant()
	profile, err := s.verifyClosedProfileLocked(raw, generation, now)
	if err != nil || profile.Digest != stored.Accepted {
		return membership{}, networkdomain.TrustedTime{}, networkdomain.RuntimeView{}, errors.New("closed profile is unavailable")
	}
	bound, err := s.bindCurrentMembership(profile)
	if err != nil {
		return membership{}, networkdomain.TrustedTime{}, networkdomain.RuntimeView{}, err
	}
	view, err := bound.accepted.Observe(observed)
	if err != nil {
		return membership{}, networkdomain.TrustedTime{}, networkdomain.RuntimeView{}, err
	}
	return bound, observed, view, nil
}
