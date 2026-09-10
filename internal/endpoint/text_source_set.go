//go:build linux

package endpoint

import (
	"crypto/rand"
	"errors"
	"math/big"
	"time"

	"github.com/dianabuilds/ardents-network/internal/entry"
	"github.com/dianabuilds/ardents-network/internal/route"
)

// The source Interior Set belongs to this local context, independently of its
// worker and of Publisher Introduction/data domains. Both members are fixed
// before dialing and neither transport failure nor worker loss resamples them.
type textSourceSet struct {
	chosen, notAfter time.Time
	interior         [2]entry.ClosedSetMember
}

func (owner *textContext) selectTextBootstrapLocked() (route.ClosedBootstrapSelection, error) {
	return owner.selectTextAdjacentLocked(1, &owner.sourceSet)
}

func (owner *textContext) selectTextAdjacentLocked(domain uint8, retained **textSourceSet) (route.ClosedBootstrapSelection, error) {
	profile, members, now, err := owner.endpoint.closedTextRoleMembers()
	if err != nil {
		return route.ClosedBootstrapSelection{}, err
	}
	entries, err := owner.endpoint.textEntrySets()
	if err != nil {
		return route.ClosedBootstrapSelection{}, err
	}
	pair, err := entries.Members(domain)
	if err != nil {
		return route.ClosedBootstrapSelection{}, err
	}
	if (*retained) != nil && now.Before((*retained).chosen) {
		return route.ClosedBootstrapSelection{}, errors.New("text source time regressed")
	}
	if (*retained) == nil || !now.Before((*retained).notAfter) {
		if owner.permission != nil && owner.permission.pending != nil {
			return route.ClosedBootstrapSelection{}, errors.New("pending issuance cannot replace source set")
		}
		selected, err := chooseTextInteriorSet(members, pair, now, domain)
		if err != nil {
			return route.ClosedBootstrapSelection{}, err
		}
		(*retained) = &selected
	}
	// The selected first member remains the initial route. An unavailable member
	// is a refusal here; this operation performs no automatic alternate attempt.
	first, err := entries.CurrentMember(domain, 0)
	if err != nil {
		return route.ClosedBootstrapSelection{}, err
	}
	interior := (*retained).interior[0]
	current := false
	for _, member := range members {
		if member.subrole == 2 && member.ClosedSetMember == interior {
			current = true
		}
	}
	if !current || sourceMemberConflict(first, interior) {
		return route.ClosedBootstrapSelection{}, errors.New("text source member unavailable")
	}
	return route.ClosedBootstrapSelection{ProfileDigest: profile.Digest, EntryNodeID: first.NodeID, InteriorNodeID: interior.NodeID}, nil
}

func chooseTextInteriorSet(members []textRoleMember, entries [2]entry.ClosedSetMember, now time.Time, domain uint8) (textSourceSet, error) {
	var eligible []entry.ClosedSetMember
	for _, member := range members {
		if member.Domain == domain && member.subrole == 2 && !sourceMemberConflict(entries[0], member.ClosedSetMember) && !sourceMemberConflict(entries[1], member.ClosedSetMember) {
			eligible = append(eligible, member.ClosedSetMember)
		}
	}
	var pairs [][2]entry.ClosedSetMember
	for _, first := range eligible {
		for _, second := range eligible {
			if !sourceMemberConflict(first, second) {
				pairs = append(pairs, [2]entry.ClosedSetMember{first, second})
			}
		}
	}
	if len(pairs) == 0 {
		return textSourceSet{}, errors.New("two eligible source Interior members unavailable")
	}
	selected, err := rand.Int(rand.Reader, big.NewInt(int64(len(pairs))))
	if err != nil {
		return textSourceSet{}, err
	}
	set := textSourceSet{chosen: now, notAfter: now.Add(30 * time.Minute), interior: pairs[selected.Int64()]}
	for _, member := range set.interior {
		if member.NotAfter.Before(set.notAfter) {
			set.notAfter = member.NotAfter
		}
	}
	return set, nil
}

func sourceMemberConflict(first, second entry.ClosedSetMember) bool {
	return first.NodeID == second.NodeID || first.PublicKey == second.PublicKey || first.FamilyID == second.FamilyID
}
