//go:build linux

package selection

import (
	"errors"
	"sync"
	"time"

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
	mu         sync.Mutex
	entries    *ClosedSets
	interior   *interiorStore
	current    func() (network.RuntimeView, error)
	domain     uint8
	exclusions []route.Member
	closed     bool
	failure    error
}

// Leg binds a selected initial pair to this exact authenticated observation.
// Its alternatives are retained but this slice performs no automatic retry.
type Leg struct {
	Entry, Interior             network.RetainedDuty
	EntryMember, InteriorMember network.Member
	Profile                     network.ProfileBinding
	NotAfter                    time.Time
	known                       []route.Member
}

func Open(config Config) (_ *Owner, result error) {
	if config.Current == nil || config.EntryRoot == config.InteriorRoot || config.EntryRoot == "" || config.InteriorRoot == "" || closedAdjacentIndex(config.Domain) < 0 {
		return nil, errors.New("route selection setup unavailable")
	}
	o := &Owner{current: config.Current, domain: config.Domain, exclusions: append([]route.Member(nil), config.Exclusions...)}
	initial, err := config.Current()
	if err != nil {
		return nil, err
	}
	profile := initial.Profile()
	o.entries, err = openClosedSets(ClosedSetConfig{Root: config.EntryRoot, NetworkID: profile.Network, Current: func() (ClosedSetView, error) {
		view, err := o.current()
		if err != nil {
			return ClosedSetView{}, err
		}
		return ClosedSetView{NetworkID: view.Profile().Network, Now: view.ObservedAt(), Candidates: o.candidates(view, 1)}, nil
	}})
	if err != nil {
		return nil, err
	}
	defer func() {
		if result != nil {
			result = errors.Join(result, o.entries.Close())
		}
	}()
	o.interior, err = openInteriorStore(config.InteriorRoot, profile.Network, config.Domain)
	if err != nil {
		return nil, err
	}
	return o, nil
}

func (o *Owner) candidates(view network.RuntimeView, subrole uint8) []ClosedSetMember {
	var candidates []ClosedSetMember
	now := view.ObservedAt()
	for _, member := range view.Members() {
		if member.Subrole != subrole || !route.PurposePermitsDuty(7, member.RoleDomain, member.Subrole) || !member.Current(now) {
			continue
		}
		candidate := ClosedSetMember{NodeID: member.NodeID, PublicKey: member.PublicKey, FamilyID: member.FamilyID, RecordDigest: member.RecordDigest, DutyGeneration: member.DutyGeneration, Domain: member.RoleDomain, NotAfter: member.NotAfter()}
		excluded := false
		for _, known := range o.exclusions {
			if route.Conflict(route.Member(candidate), known) {
				excluded = true
				break
			}
		}
		if !excluded {
			candidates = append(candidates, candidate)
		}
	}
	return candidates
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
	return Leg{Entry: entry, Interior: interior, EntryMember: entryMember, InteriorMember: interiorMember, Profile: view.Profile().ProfileBinding, NotAfter: end, known: known}, nil
}

// Check reobserves current Network authority, never rebinds a retained leg,
// and checks exact assignment, role, keys, family and immutable profile.
func (leg Leg) Check(view network.RuntimeView, now time.Time) error {
	if now.Before(view.ObservedAt()) {
		now = view.ObservedAt()
	}
	if !now.Before(leg.NotAfter) || view.Profile().ProfileBinding != leg.Profile || view.MatchDuty(leg.Entry, now) != nil || view.MatchDuty(leg.Interior, now) != nil {
		return errors.New("retained Route authority unavailable")
	}
	first, err := view.Member(leg.Entry.NodeID, now)
	if err != nil {
		return err
	}
	second, err := view.Member(leg.Interior.NodeID, now)
	if err != nil {
		return err
	}
	if first.RecordDigest != leg.EntryMember.RecordDigest || second.RecordDigest != leg.InteriorMember.RecordDigest || first.Subrole != 1 || second.Subrole != 2 || first.RoleDomain != leg.EntryMember.RoleDomain || second.RoleDomain != leg.InteriorMember.RoleDomain || first.RoleDomain != second.RoleDomain ||
		!route.PurposePermitsDuty(7, first.RoleDomain, first.Subrole) || !route.PurposePermitsDuty(7, second.RoleDomain, second.Subrole) ||
		route.Conflict(route.Member{NodeID: first.NodeID, PublicKey: first.PublicKey, FamilyID: first.FamilyID}, route.Member{NodeID: second.NodeID, PublicKey: second.PublicKey, FamilyID: second.FamilyID}) {
		return errors.New("retained Route participant changed")
	}
	return nil
}

func (o *Owner) Close() error {
	if o == nil {
		return nil
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if !o.closed {
		o.closed = true
		o.failure = errors.Join(o.failure, o.interior.close(), o.entries.Close())
	}
	return o.failure
}
func minTime(first time.Time, rest ...time.Time) time.Time {
	for _, value := range rest {
		if value.Before(first) {
			first = value
		}
	}
	return first
}
