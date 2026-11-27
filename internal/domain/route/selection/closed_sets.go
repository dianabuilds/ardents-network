package selection

import (
	"errors"
	"slices"
	"sync"
	"time"
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
	// operations serializes observation and use without holding mu in Network.
	operations sync.Mutex
	closeOnce  sync.Once
	mu         sync.Mutex
	root       string
	lease      rootLease
	current    func() (ClosedSetView, error)
	state      closedSetState
	name       string
	closed     bool
	failure    error
}

type closedSetState struct {
	Version    uint8              `json:"version"`
	NetworkID  [32]byte           `json:"network"`
	Generation uint64             `json:"generation"`
	Previous   string             `json:"previous,omitempty"`
	Sets       [3]closedDomainSet `json:"sets"`
}

func (owner *ClosedSets) retainsLocked(domain uint8, selected closedDomainSet, now time.Time) bool {
	return !owner.closed && owner.failure == nil && owner.state.Sets[closedAdjacentIndex(domain)] == selected &&
		!now.Before(selected.Chosen) && now.Before(selected.NotAfter)
}

// selectMembers fixes both alternatives before effects. Call currentMember
// immediately before using a slot. A missing member or transport failure
// cannot rotate the retained set.
func (owner *ClosedSets) selectMembers(domain uint8, role *Owner) (closedDomainSet, ClosedSetView, error) {
	index := closedAdjacentIndex(domain)
	owner.operations.Lock()
	defer owner.operations.Unlock()
	owner.mu.Lock()
	unavailable := owner.closed || owner.failure != nil || index < 0
	owner.mu.Unlock()
	if unavailable {
		return closedDomainSet{}, ClosedSetView{}, errors.New("closed Entry set unavailable")
	}
	if role != nil {
		if err := role.available(); err != nil {
			return closedDomainSet{}, ClosedSetView{}, err
		}
	}
	view, err := owner.current()
	if role != nil {
		role.mu.Lock()
		defer role.mu.Unlock()
		if err := role.availableLocked(); err != nil {
			return closedDomainSet{}, ClosedSetView{}, err
		}
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if owner.closed || owner.failure != nil {
		return closedDomainSet{}, ClosedSetView{}, errors.New("closed Entry set unavailable")
	}
	if err != nil || !validClosedSetView(view, owner.state.NetworkID) {
		return closedDomainSet{}, ClosedSetView{}, errors.New("closed Entry State unavailable")
	}
	selected := owner.state.Sets[index]
	if !selected.Chosen.IsZero() && view.Now.Before(selected.Chosen) {
		return closedDomainSet{}, ClosedSetView{}, errors.New("closed Entry time regressed")
	}
	if selected.Chosen.IsZero() || !view.Now.Before(selected.NotAfter) {
		selected, err = chooseClosedEntryPair(view, domain)
		if err != nil {
			return closedDomainSet{}, ClosedSetView{}, err
		}
		next := owner.state
		next.Sets[index] = selected
		if role != nil {
			if err := role.availableLocked(); err != nil {
				return closedDomainSet{}, ClosedSetView{}, err
			}
		}
		if err := owner.commitClosedSet(next); err != nil {
			owner.failure = err
			return closedDomainSet{}, ClosedSetView{}, err
		}
	}
	return selected, view, nil
}

// currentMember validates one already selected slot. It never activates a
// domain, draws randomness, rotates an expired set, or selects an alternative.
func (owner *ClosedSets) currentMember(domain, slot uint8) (ClosedSetMember, ClosedSetView, error) {
	owner.operations.Lock()
	defer owner.operations.Unlock()
	owner.mu.Lock()
	index := closedAdjacentIndex(domain)
	unavailable := owner.closed || owner.failure != nil || index < 0 || slot > 1
	owner.mu.Unlock()
	if unavailable {
		return ClosedSetMember{}, ClosedSetView{}, errors.New("closed Entry member unavailable")
	}
	view, err := owner.current()
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if owner.closed || owner.failure != nil {
		return ClosedSetMember{}, ClosedSetView{}, errors.New("closed Entry member unavailable")
	}
	if err != nil || !validClosedSetView(view, owner.state.NetworkID) {
		return ClosedSetMember{}, ClosedSetView{}, errors.New("closed Entry State unavailable")
	}
	selected := owner.state.Sets[index]
	if selected.Chosen.IsZero() || view.Now.Before(selected.Chosen) || !view.Now.Before(selected.NotAfter) {
		return ClosedSetMember{}, ClosedSetView{}, errors.New("closed Entry set expired")
	}
	member := selected.Members[slot]
	if slices.Contains(view.Candidates, member) {
		return member, view, nil
	}
	return ClosedSetMember{}, ClosedSetView{}, errors.New("closed Entry member no longer current")
}

func (owner *ClosedSets) Close() error {
	if owner == nil {
		return nil
	}
	owner.closeOnce.Do(func() {
		owner.mu.Lock()
		owner.closed = true
		owner.mu.Unlock()
		owner.operations.Lock()
		defer owner.operations.Unlock()
		owner.mu.Lock()
		defer owner.mu.Unlock()
		owner.failure = errors.Join(owner.failure, owner.lease.release())
	})
	return owner.failure
}
