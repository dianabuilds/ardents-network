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
	"github.com/dianabuilds/ardents-network/internal/route/client"
)

// Each Source, Introduction, and Responder lifecycle retains its own Interior
// Set. Both members are fixed before dialing; transport failure and worker loss
// do not resample the retained choice.
type interiorSet struct {
	chosen, notAfter time.Time
	interior         [2]entry.ClosedSetMember
}

type interiorSelectionFailure struct {
	stage string
	cause error
}

func (failure *interiorSelectionFailure) Error() string { return failure.cause.Error() }

func (failure *interiorSelectionFailure) Unwrap() error { return failure.cause }

func interiorSelectionFailureAt(stage string, cause error) error {
	return &interiorSelectionFailure{stage: stage, cause: cause}
}

func interiorSelectionFailureStage(cause error) string {
	var failure *interiorSelectionFailure
	if errors.As(cause, &failure) && failure.stage != "" {
		return failure.stage
	}
	return "unknown"
}

func (owner *textContext) selectBootstrapLocked() (client.ClosedBootstrapSelection, error) {
	return owner.selectAdjacentLocked(1, owner.source.membersSlotLocked())
}

func (owner *textContext) selectAdjacentLocked(domain uint8, retained **interiorSet) (client.ClosedBootstrapSelection, error) {
	profile, members, now, err := owner.endpoint.closedRoleMembers()
	if err != nil {
		return client.ClosedBootstrapSelection{}, interiorSelectionFailureAt("role-members-"+roleMemberFailureStage(err), err)
	}
	entries, err := owner.endpoint.textEntrySets()
	if err != nil {
		return client.ClosedBootstrapSelection{}, interiorSelectionFailureAt("entries", err)
	}
	pair, err := entries.Members(domain)
	if err != nil {
		return client.ClosedBootstrapSelection{}, interiorSelectionFailureAt("entry-pair", err)
	}
	if (*retained) != nil && now.Before((*retained).chosen) {
		return client.ClosedBootstrapSelection{}, interiorSelectionFailureAt("time", errors.New("text source time regressed"))
	}
	if (*retained) == nil || !now.Before((*retained).notAfter) {
		if owner.tokens.permission.hasPending() {
			return client.ClosedBootstrapSelection{}, errors.New("pending issuance cannot replace source set")
		}
		selected, err := chooseInteriorSet(members, pair, now, domain)
		if err != nil {
			return client.ClosedBootstrapSelection{}, interiorSelectionFailureAt("choose", err)
		}
		(*retained) = &selected
	}
	// The selected first member remains the initial route. An unavailable member
	// is a refusal here; this operation performs no automatic alternate attempt.
	first, err := entries.CurrentMember(domain, 0)
	if err != nil {
		return client.ClosedBootstrapSelection{}, interiorSelectionFailureAt("entry-current", err)
	}
	interior := (*retained).interior[0]
	current := false
	for _, member := range members {
		if member.subrole == 2 && member.ClosedSetMember == interior {
			current = true
		}
	}
	if !current || interiorMemberConflict(first, interior) {
		return client.ClosedBootstrapSelection{}, interiorSelectionFailureAt("membership", errors.New("text source member unavailable"))
	}
	return client.ClosedBootstrapSelection{ProfileDigest: profile.Digest, EntryNodeID: first.NodeID, InteriorNodeID: interior.NodeID}, nil
}

func chooseInteriorSet(members []roleMember, entries [2]entry.ClosedSetMember, now time.Time, domain uint8) (interiorSet, error) {
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
		return interiorSet{}, errors.New("two eligible source Interior members unavailable")
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
	set := interiorSet{chosen: now, notAfter: now.Add(30 * time.Minute), interior: pairs[selected]}
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
