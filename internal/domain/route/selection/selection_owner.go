package selection

import (
	"errors"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/network"
	"github.com/dianabuilds/ardents-network/internal/domain/route"
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

// Owner serializes retained choices and joins its selection before releasing
// the original Interior history.
type Owner struct {
	selection           sync.Mutex
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
	o.selection.Lock()
	defer o.selection.Unlock()
	if err := o.available(); err != nil {
		return Leg{}, err
	}
	entrySet, initial, err := o.entries.selectMembers(o.domain, o)
	if err != nil {
		return Leg{}, err
	}
	entries := entrySet.Members
	if err := o.available(); err != nil {
		return Leg{}, err
	}
	view, err := o.current()
	if err != nil {
		return Leg{}, err
	}
	now := view.ObservedAt()
	if view.ProfileBinding() != initial.profile || now.Before(initial.Now) {
		return Leg{}, errors.New("route selection observation changed")
	}
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
	set, err := o.selectInterior(peers, pair, now, entrySet)
	if err != nil {
		return Leg{}, err
	}
	// Reobserve after both durable selection effects; retain from this same
	// observation only after comparing every selected binding.
	if err := o.available(); err != nil {
		return Leg{}, err
	}
	view, err = o.current()
	if err != nil {
		return Leg{}, err
	}
	if view.ProfileBinding() != initial.profile || view.ObservedAt().Before(now) {
		return Leg{}, errors.New("route selection observation changed")
	}
	now = view.ObservedAt()
	if err := o.available(); err != nil {
		return Leg{}, err
	}
	currentEntry, latest, err := o.entries.currentMember(o.domain, 0)
	if err != nil || currentEntry != entries[0] {
		return Leg{}, errors.Join(errors.New("selected Entry unavailable"), err)
	}
	if latest.profile != initial.profile || latest.Now.Before(now) {
		return Leg{}, errors.New("route selection observation changed")
	}
	// The last Entry observation can advance trusted time past an Interior
	// bound. Check the whole retained leg at that later time.
	now = latest.Now
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
	end := minTime(entries[0].NotAfter, set.NotAfter, view.ProfileBinding().NotAfter)
	known := []route.Member{route.Member(entries[0]), route.Member(entries[1]), set.Members[0], set.Members[1]}
	known = append(known, o.exclusions...)
	o.mu.Lock()
	defer o.mu.Unlock()
	o.entries.mu.Lock()
	defer o.entries.mu.Unlock()
	if err := o.availableLocked(); err != nil {
		return Leg{}, err
	}
	if !o.entries.retainsLocked(o.domain, entrySet, now) {
		return Leg{}, errors.New("selected Entry set changed")
	}
	return Leg{Entry: entry, Interior: interior, EntryMember: entryMember, InteriorMember: interiorMember, Profile: view.ProfileBinding(), NotAfter: end, known: known}, nil
}

func (o *Owner) selectInterior(peers []route.Member, pair [2]route.Member, now time.Time, entrySet closedDomainSet) (route.InteriorSet, error) {
	// Hold the same Entry generation across the dependent local write. Neither
	// this phase nor the final handoff performs a Network observation.
	o.mu.Lock()
	defer o.mu.Unlock()
	o.entries.mu.Lock()
	defer o.entries.mu.Unlock()
	if !o.entries.retainsLocked(o.domain, entrySet, now) {
		return route.InteriorSet{}, errors.New("selected Entry set changed")
	}
	if err := o.availableLocked(); err != nil {
		return route.InteriorSet{}, err
	}
	return o.interior.selectPair(peers, pair, now)
}

func (o *Owner) availableLocked() error {
	if o.closed || o.failure != nil {
		return errors.Join(errors.New("route selection closed"), o.failure)
	}
	return o.installation.available()
}

func (o *Owner) available() error {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.availableLocked()
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
	o.selection.Lock()
	result := errors.Join(o.failure, o.interior.close())
	o.selection.Unlock()
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
