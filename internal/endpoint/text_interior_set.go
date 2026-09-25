//go:build linux

package endpoint

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"slices"
	"time"

	"github.com/dianabuilds/ardents-network/internal/entry"
	"github.com/dianabuilds/ardents-network/internal/route"
)

// Each Source, Introduction, and Responder lifecycle retains its own Interior
// Set. Both members are fixed before dialing; transport failure and worker loss
// do not resample the retained choice.
type textInteriorSet struct {
	chosen, notAfter time.Time
	interior         [2]entry.ClosedSetMember
}

type textInteriorSelectionFailure struct {
	stage string
	cause error
}

func (failure *textInteriorSelectionFailure) Error() string { return failure.cause.Error() }

func (failure *textInteriorSelectionFailure) Unwrap() error { return failure.cause }

func textInteriorSelectionFailureAt(stage string, cause error) error {
	return &textInteriorSelectionFailure{stage: stage, cause: cause}
}

func textInteriorSelectionFailureStage(cause error) string {
	var failure *textInteriorSelectionFailure
	if errors.As(cause, &failure) && failure.stage != "" {
		return failure.stage
	}
	return "unknown"
}

func (owner *textContext) selectTextBootstrapLocked() (route.ClosedBootstrapSelection, error) {
	return owner.selectTextAdjacentLocked(1, owner.source.membersSlotLocked())
}

func (owner *textContext) selectTextAdjacentLocked(domain uint8, retained **textInteriorSet) (route.ClosedBootstrapSelection, error) {
	profile, members, now, err := owner.endpoint.closedTextRoleMembers()
	if err != nil {
		return route.ClosedBootstrapSelection{}, textInteriorSelectionFailureAt("role-members-"+textRoleMemberFailureStage(err), err)
	}
	entries, err := owner.endpoint.textEntrySets()
	if err != nil {
		return route.ClosedBootstrapSelection{}, textInteriorSelectionFailureAt("entries", err)
	}
	pair, err := entries.Members(domain)
	if err != nil {
		return route.ClosedBootstrapSelection{}, textInteriorSelectionFailureAt("entry-pair", err)
	}
	if (*retained) != nil && now.Before((*retained).chosen) {
		return route.ClosedBootstrapSelection{}, textInteriorSelectionFailureAt("time", errors.New("text source time regressed"))
	}
	if (*retained) == nil || !now.Before((*retained).notAfter) {
		if owner.permission.hasPending() {
			return route.ClosedBootstrapSelection{}, errors.New("pending issuance cannot replace source set")
		}
		selected, err := chooseTextInteriorSet(members, pair, now, domain)
		if err != nil {
			return route.ClosedBootstrapSelection{}, textInteriorSelectionFailureAt("choose", err)
		}
		(*retained) = &selected
	}
	// The selected first member remains the initial route. An unavailable member
	// is a refusal here; this operation performs no automatic alternate attempt.
	first, err := entries.CurrentMember(domain, 0)
	if err != nil {
		return route.ClosedBootstrapSelection{}, textInteriorSelectionFailureAt("entry-current", err)
	}
	interior := (*retained).interior[0]
	current := false
	for _, member := range members {
		if member.subrole == 2 && member.ClosedSetMember == interior {
			current = true
		}
	}
	if !current || interiorMemberConflict(first, interior) {
		return route.ClosedBootstrapSelection{}, textInteriorSelectionFailureAt("membership", errors.New("text source member unavailable"))
	}
	return route.ClosedBootstrapSelection{ProfileDigest: profile.Digest, EntryNodeID: first.NodeID, InteriorNodeID: interior.NodeID}, nil
}

func chooseTextInteriorSet(members []textRoleMember, entries [2]entry.ClosedSetMember, now time.Time, domain uint8) (textInteriorSet, error) {
	var eligible []entry.ClosedSetMember
	for _, member := range members {
		if member.Domain == domain && member.subrole == 2 && !interiorMemberConflict(entries[0], member.ClosedSetMember) && !interiorMemberConflict(entries[1], member.ClosedSetMember) {
			eligible = append(eligible, member.ClosedSetMember)
		}
	}
	slices.SortFunc(eligible, func(first, second entry.ClosedSetMember) int {
		return bytes.Compare(first.NodeID[:], second.NodeID[:])
	})
	var pairs [][2]entry.ClosedSetMember
	for _, first := range eligible {
		for _, second := range eligible {
			if !interiorMemberConflict(first, second) {
				pairs = append(pairs, [2]entry.ClosedSetMember{first, second})
			}
		}
	}
	if len(pairs) == 0 {
		return textInteriorSet{}, errors.New("two eligible source Interior members unavailable")
	}
	// The adjacent pair is already a private, durably random installation
	// choice. Deriving the Interior pair from that retained choice keeps every
	// context and process on the same route without exposing a caller-selected
	// Node or drawing a second route after restart.
	digestInput := make([]byte, 0, 1+4*len(entries[0].NodeID))
	digestInput = append(digestInput, domain)
	for _, member := range entries {
		digestInput = append(digestInput, member.NodeID[:]...)
		digestInput = append(digestInput, member.PublicKey[:]...)
	}
	digest := sha256.Sum256(digestInput)
	selected := binary.BigEndian.Uint64(digest[:8]) % uint64(len(pairs))
	set := textInteriorSet{chosen: now, notAfter: now.Add(30 * time.Minute), interior: pairs[selected]}
	for _, member := range set.interior {
		if member.NotAfter.Before(set.notAfter) {
			set.notAfter = member.NotAfter
		}
	}
	return set, nil
}

func interiorMemberConflict(first, second entry.ClosedSetMember) bool {
	return first.NodeID == second.NodeID || first.PublicKey == second.PublicKey || first.FamilyID == second.FamilyID
}
