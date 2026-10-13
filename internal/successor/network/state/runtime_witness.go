package state

import (
	"errors"
	"sync/atomic"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network"
)

// RuntimeWitness retains only the original State's local refusal boundary.
// It cannot replace fresh CurrentRuntime observations or authorize an effect.
// Its view remains an immutable observation; invalidation never changes facts.
type RuntimeWitness struct {
	view     network.RuntimeView
	original *runtimeAdmission
}

type runtimeAdmission struct {
	profile network.ProfileBinding
	retired atomic.Bool
}

// View returns the full authenticated observation that created this witness.
func (w RuntimeWitness) View() network.RuntimeView { return w.view }

// CheckCommit checks only the retained original identity and synchronous local
// refusal projection. It performs no I/O, callback or State-lock acquisition.
// Callers must independently perform their required fresh full observation.
func (w RuntimeWitness) CheckCommit(profile network.ProfileBinding) error {
	if w.original == nil || w.original.profile != profile || w.original.retired.Load() {
		return errors.New("original Network observation retired")
	}
	return w.view.Check(time.Now())
}

// SameOriginal prevents a newly prepared physical resource from replacing the
// original logical operation's Network owner or its invalidation generation.
func (w RuntimeWitness) SameOriginal(other RuntimeWitness) bool {
	return w.original != nil && w.original == other.original
}

// State admission mutex precedes runtimeMu. Readers may invalidate on a failed
// physical observation while holding a State read lock; no reverse edge exists.
func (s *networkState) invalidateRuntime() {
	s.runtimeMu.Lock()
	defer s.runtimeMu.Unlock()
	if s.runtimeAdmission != nil {
		s.runtimeAdmission.retired.Store(true)
		s.runtimeAdmission = nil
	}
}

func (s *networkState) witnessLocked(view network.RuntimeView) RuntimeWitness {
	s.runtimeMu.Lock()
	defer s.runtimeMu.Unlock()
	profile := view.Profile().ProfileBinding
	if s.runtimeAdmission != nil && s.runtimeAdmission.profile != profile {
		s.runtimeAdmission.retired.Store(true)
		s.runtimeAdmission = nil
	}
	if s.runtimeAdmission == nil {
		s.runtimeAdmission = &runtimeAdmission{profile: profile}
	}
	return RuntimeWitness{view: view, original: s.runtimeAdmission}
}
