//go:build linux

package entry

import (
	"crypto/rand"
	"errors"
	"math/big"
	"sync"
	"time"
)

// ClosedSetMember is a public State binding, not a transport or admission.
// Addresses and Carrier selection are resolved again by Route before dialing.
type ClosedSetMember struct {
	NodeID, PublicKey, FamilyID, RecordDigest [32]byte
	DutyGeneration                            uint64
	Domain                                    uint8
	NotAfter                                  time.Time
}

// ClosedSetView is supplied only by the participant's opened, live State owner.
// It contains the eligible adjacent duties after local role/family exclusions.
type ClosedSetView struct {
	NetworkID  [32]byte
	Now        time.Time
	Candidates []ClosedSetMember
}

// ClosedSetConfig claims the installation's selected-generation Entry root.
// A legacy Invite root is incompatible and requires explicit migration.
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

type closedDomainSet struct {
	Chosen   time.Time          `json:"chosen"`
	NotAfter time.Time          `json:"not_after"`
	Members  [2]ClosedSetMember `json:"members"`
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

func validClosedSetView(view ClosedSetView, network [32]byte) bool {
	if network == [32]byte{} || view.NetworkID != network || view.Now.IsZero() || len(view.Candidates) > 32 {
		return false
	}
	seen := make(map[[32]byte]bool)
	for _, member := range view.Candidates {
		if !validClosedSetMember(member) || seen[member.NodeID] || !view.Now.Before(member.NotAfter) {
			return false
		}
		seen[member.NodeID] = true
	}
	return true
}

func validClosedSetMember(member ClosedSetMember) bool {
	return member.NodeID != [32]byte{} && member.PublicKey != [32]byte{} && member.FamilyID != [32]byte{} && member.RecordDigest != [32]byte{} &&
		member.DutyGeneration != 0 && closedAdjacentIndex(member.Domain) >= 0 && !member.NotAfter.IsZero() && member.NotAfter == member.NotAfter.UTC().Truncate(time.Second)
}

func chooseClosedEntryPair(view ClosedSetView, domain uint8) (closedDomainSet, error) {
	// Uniformly draw one of the eligible ordered pairs. At most32 public Nodes
	// makes this finite enumeration small; the order fixes the initial member.
	var pairs [][2]ClosedSetMember
	for _, first := range view.Candidates {
		if first.Domain != domain {
			continue
		}
		for _, second := range view.Candidates {
			if second.Domain == domain && first.NodeID != second.NodeID && first.PublicKey != second.PublicKey && first.FamilyID != second.FamilyID {
				pairs = append(pairs, [2]ClosedSetMember{first, second})
			}
		}
	}
	if len(pairs) == 0 {
		return closedDomainSet{}, errors.New("two eligible closed Entry members unavailable")
	}
	chosen, err := rand.Int(rand.Reader, big.NewInt(int64(len(pairs))))
	if err != nil {
		return closedDomainSet{}, err
	}
	selected := closedDomainSet{Chosen: view.Now.UTC(), NotAfter: view.Now.Add(6 * time.Hour).UTC(), Members: pairs[chosen.Int64()]}
	for _, member := range selected.Members {
		if member.NotAfter.Before(selected.NotAfter) {
			selected.NotAfter = member.NotAfter
		}
	}
	return selected, nil
}

// Preserve version-1 storage positions for Initiator (0) and Responder (2).
// The remaining slot may hold Introduction only when its explicit Domain is 4;
// old nonempty Domain-2 records are refused, never renamed or resampled.
func closedAdjacentIndex(domain uint8) int {
	switch domain {
	case 1:
		return 0
	case 3:
		return 2
	case 4:
		return 1
	default:
		return -1
	}
}
