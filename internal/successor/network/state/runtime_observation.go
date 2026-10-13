package state

import networkdomain "github.com/dianabuilds/ardents-network/internal/successor/network"

// CurrentRuntime returns one authenticated, bounded Network observation.
// Profile, generation, membership and trusted time share the same read lock;
// trusted time is sampled after the durable profile read. It grants no lease
// on future State, no token authority and no reservation. The caller reobserves
// at its operation's admission points, outside every State lock.
func (s *networkState) CurrentRuntime() (networkdomain.RuntimeView, error) {
	witness, err := s.CurrentRuntimeWitness()
	return witness.View(), err
}

// CurrentRuntimeWitness performs the same complete observation and additionally
// retains its original local refusal projection for a later bounded handoff.
// Fresh observations remain mandatory; this does not lend authority or storage.
func (s *networkState) CurrentRuntimeWitness() (RuntimeWitness, error) {
	if s.config.permitWork != nil {
		if err := s.config.permitWork(); err != nil {
			s.invalidateRuntime()
			return RuntimeWitness{}, err
		}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	bound, observed, err := s.currentMembershipLocked(s.storage.LoadClosedProfile)
	if err != nil {
		s.invalidateRuntime()
		return RuntimeWitness{}, err
	}
	view, err := bound.accepted.Observe(observed)
	if err != nil {
		s.invalidateRuntime()
		return RuntimeWitness{}, err
	}
	return s.witnessLocked(view), nil
}
