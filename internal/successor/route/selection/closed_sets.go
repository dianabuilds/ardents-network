//go:build linux

package selection

import (
	"errors"
	"sync"
)

// ClosedSetConfig claims the installation's selected-generation Entry root.
// A legacy Invite root is incompatible: its marker and recipient file fail
// the closed-root inspection, and ADR-0106 removed every migration path.
type ClosedSetConfig struct {
	Root      string
	NetworkID [32]byte
	Current   func() (ClosedSetView, error)
}

// ClosedSets retains exactly two Entry members for each activated adjacent
// Role Domain. Losing a member never causes replacement before set expiry.
// It generates no identity, Invite, holder key, admission, or transport.
type ClosedSets struct {
	mu      sync.Mutex
	root    string
	lease   rootLease
	current func() (ClosedSetView, error)
	state   closedSetState
	name    string
	closed  bool
	failure error
}

type closedSetState struct {
	Version    uint8              `json:"version"`
	NetworkID  [32]byte           `json:"network"`
	Generation uint64             `json:"generation"`
	Previous   string             `json:"previous,omitempty"`
	Sets       [3]closedDomainSet `json:"sets"`
}

// Members fixes both alternatives before effects and returns their retained
// public bindings. Call CurrentMember immediately before using either slot;
// neither a missing current member nor a transport failure rotates this set.
func (owner *ClosedSets) Members(domain uint8) ([2]ClosedSetMember, error) {
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if owner.closed || owner.failure != nil || closedAdjacentIndex(domain) < 0 {
		return [2]ClosedSetMember{}, errors.New("closed Entry set unavailable")
	}
	view, err := owner.current()
	if err != nil || !validClosedSetView(view, owner.state.NetworkID) {
		return [2]ClosedSetMember{}, errors.New("closed Entry State unavailable")
	}
	selected := owner.state.Sets[closedAdjacentIndex(domain)]
	if !selected.Chosen.IsZero() && view.Now.Before(selected.Chosen) {
		return [2]ClosedSetMember{}, errors.New("closed Entry time regressed")
	}
	if selected.Chosen.IsZero() || !view.Now.Before(selected.NotAfter) {
		selected, err = chooseClosedEntryPair(view, domain)
		if err != nil {
			return [2]ClosedSetMember{}, err
		}
		next := owner.state
		next.Sets[closedAdjacentIndex(domain)] = selected
		if err := owner.commitClosedSet(next); err != nil {
			owner.failure = err
			return [2]ClosedSetMember{}, err
		}
	}
	return selected.Members, nil
}

// CurrentMember validates one already selected slot. It never activates a
// domain, draws randomness, rotates an expired set, or selects an alternative.
func (owner *ClosedSets) CurrentMember(domain, slot uint8) (ClosedSetMember, error) {
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if owner.closed || owner.failure != nil || closedAdjacentIndex(domain) < 0 || slot > 1 {
		return ClosedSetMember{}, errors.New("closed Entry member unavailable")
	}
	view, err := owner.current()
	if err != nil || !validClosedSetView(view, owner.state.NetworkID) {
		return ClosedSetMember{}, errors.New("closed Entry State unavailable")
	}
	selected := owner.state.Sets[closedAdjacentIndex(domain)]
	if selected.Chosen.IsZero() || view.Now.Before(selected.Chosen) || !view.Now.Before(selected.NotAfter) {
		return ClosedSetMember{}, errors.New("closed Entry set expired")
	}
	member := selected.Members[slot]
	for _, candidate := range view.Candidates {
		if candidate == member {
			return member, nil
		}
	}
	return ClosedSetMember{}, errors.New("closed Entry member no longer current")
}

func (owner *ClosedSets) Close() error {
	if owner == nil {
		return nil
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if !owner.closed {
		owner.closed = true
		owner.failure = errors.Join(owner.failure, owner.lease.release())
	}
	return owner.failure
}
