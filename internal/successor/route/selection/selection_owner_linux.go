//go:build linux

package selection

import (
	"errors"
	"sync"

	"github.com/dianabuilds/ardents-network/internal/successor/network"
	"github.com/dianabuilds/ardents-network/internal/successor/route"
)

// Config names independent local selection roots and a genuine current
// Network observer. Domain is the local Initiator, Responder or Introduction
// role; exclusions are locally known controlled identities and families.
type Config struct {
	EntryRoot, InteriorRoot string
	Domain                  uint8
	Current                 func() (network.RuntimeView, error)
	Exclusions              []route.Member
}

// Owner serializes retained choices. Transport never runs while it is locked.
type Owner struct {
	mu                  sync.Mutex
	entries             *ClosedSets
	interior            *interiorStore
	current             func() (network.RuntimeView, error)
	domain              uint8
	exclusions          []route.Member
	installation        *Installation
	root                string
	privateInstallation bool
	closeDone           chan struct{}
	closed              bool
	failure             error
}

func Open(config Config) (*Owner, error) {
	if config.Current == nil || config.EntryRoot == "" || config.InteriorRoot == "" || closedAdjacentIndex(config.Domain) < 0 {
		return nil, errors.New("route selection setup unavailable")
	}
	entryRoot, err := selectionRoot(config.EntryRoot)
	if err != nil {
		return nil, err
	}
	interiorRoot, err := selectionRoot(config.InteriorRoot)
	if err != nil {
		return nil, err
	}
	if rootsOverlap(entryRoot, interiorRoot) {
		return nil, errors.New("route selection roots overlap")
	}
	installation, err := OpenInstallation(InstallationConfig{EntryRoot: config.EntryRoot, Current: config.Current, Exclusions: config.Exclusions})
	if err != nil {
		return nil, err
	}
	owner, err := installation.Borrow(RoleConfig{InteriorRoot: config.InteriorRoot, Domain: config.Domain})
	if err != nil {
		return nil, errors.Join(err, installation.Close())
	}
	owner.privateInstallation = true
	return owner, nil
}
func (o *Owner) candidates(view network.RuntimeView, subrole uint8) []ClosedSetMember {
	return selectionCandidates(view, subrole, o.exclusions)
}

func (o *Owner) Select() (Leg, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closed || o.failure != nil {
		return Leg{}, errors.Join(errors.New("route selection closed"), o.failure)
	}
	entries, err := o.entries.Members(o.domain)
	if err != nil {
		return Leg{}, err
	}
	view, err := o.current()
	if err != nil {
		return Leg{}, err
	}
	now := view.ObservedAt()
	candidates := o.candidates(view, 2)
	peers := make([]route.Member, len(candidates))
	for index, member := range candidates {
		peers[index] = route.Member(member)
	}
	pair := [2]route.Member{route.Member(entries[0]), route.Member(entries[1])}
	for _, entry := range pair {
		for _, excluded := range o.exclusions {
			if route.Conflict(entry, excluded) {
				return Leg{}, errors.New("selected Entry excluded by role")
			}
		}
	}
	if err := o.installation.available(); err != nil {
		return Leg{}, err
	}
	set, err := o.interior.selectPair(peers, pair, now)
	if err != nil {
		return Leg{}, err
	}
	// Reobserve after both durable selection effects; retain from this same
	// observation only after comparing every selected binding.
	view, err = o.current()
	if err != nil {
		return Leg{}, err
	}
	now = view.ObservedAt()
	currentEntry, err := o.entries.CurrentMember(o.domain, 0)
	if err != nil || currentEntry != entries[0] {
		return Leg{}, errors.Join(errors.New("selected Entry unavailable"), err)
	}
	after := o.candidates(view, 2)
	peers = peers[:0]
	for _, member := range after {
		peers = append(peers, route.Member(member))
	}
	if err := set.Check(peers, pair, now, o.domain); err != nil {
		return Leg{}, err
	}
	entryMember, err := view.Member(entries[0].NodeID, now)
	if err != nil {
		return Leg{}, err
	}
	interiorMember, err := view.Member(set.Members[0].NodeID, now)
	if err != nil {
		return Leg{}, err
	}
	entry, err := view.RetainDuty(entryMember.NodeID, now)
	if err != nil {
		return Leg{}, err
	}
	interior, err := view.RetainDuty(interiorMember.NodeID, now)
	if err != nil {
		return Leg{}, err
	}
	end := minTime(entries[0].NotAfter, set.NotAfter, view.Profile().NotAfter)
	known := []route.Member{route.Member(entries[0]), route.Member(entries[1]), set.Members[0], set.Members[1]}
	known = append(known, o.exclusions...)
	if err := o.installation.available(); err != nil {
		return Leg{}, err
	}
	return Leg{Entry: entry, Interior: interior, EntryMember: entryMember, InteriorMember: interiorMember, Profile: view.Profile().ProfileBinding, NotAfter: end, known: known}, nil
}

func (o *Owner) Close() error {
	if o == nil {
		return nil
	}
	o.mu.Lock()
	if o.closed {
		done := o.closeDone
		o.mu.Unlock()
		<-done
		o.mu.Lock()
		result := o.failure
		o.mu.Unlock()
		return result
	}
	o.closed = true
	o.mu.Unlock()
	result := errors.Join(o.failure, o.interior.close())
	o.installation.returnBorrow(o.root, result)
	if o.privateInstallation {
		result = errors.Join(result, o.installation.Close())
	}
	o.mu.Lock()
	o.failure = result
	close(o.closeDone)
	result = o.failure
	o.mu.Unlock()
	return result
}
