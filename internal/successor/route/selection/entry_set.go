package selection

import (
	"crypto/rand"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/successor/network"
	"github.com/dianabuilds/ardents-network/internal/successor/route"
	"math/big"
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

type closedDomainSet struct {
	Chosen   time.Time          `json:"chosen"`
	NotAfter time.Time          `json:"not_after"`
	Members  [2]ClosedSetMember `json:"members"`
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

func selectionCandidates(view network.RuntimeView, subrole uint8, exclusions []route.Member) []ClosedSetMember {
	var candidates []ClosedSetMember
	now := view.ObservedAt()
	for _, member := range view.Members() {
		if member.Subrole != subrole || !route.PurposePermitsDuty(7, member.RoleDomain, member.Subrole) || !member.Current(now) {
			continue
		}
		candidate := ClosedSetMember{NodeID: member.NodeID, PublicKey: member.PublicKey, FamilyID: member.FamilyID, RecordDigest: member.RecordDigest, DutyGeneration: member.DutyGeneration, Domain: member.RoleDomain, NotAfter: member.NotAfter()}
		excluded := false
		for _, known := range exclusions {
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
