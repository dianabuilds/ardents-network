package selection

import (
	"errors"

	"github.com/dianabuilds/ardents-network/internal/successor/network"
	"github.com/dianabuilds/ardents-network/internal/successor/route"
)

// ResolutionDuty selects the sole current purpose-3 recipient for the original
// Domain-1 Source. The signed member inventory owns its identity; neither a
// Descriptor nor the caller can supply a replacement Node or literal endpoint.
func (leg Leg) ResolutionDuty(view network.RuntimeView, excluded []route.Member) (network.RetainedDuty, error) {
	if leg.EntryMember.RoleDomain != 1 {
		return network.RetainedDuty{}, errors.New("resolution requires original Domain-1 Source")
	}
	now := view.ObservedAt()
	if err := leg.Check(view, now); err != nil {
		return network.RetainedDuty{}, err
	}
	known := append([]route.Member(nil), leg.known...)
	known = append(known, excluded...)
	known = append(known, rendezvousPeer(leg.EntryMember), rendezvousPeer(leg.InteriorMember))
	var selected network.Member
	for _, member := range view.Members() {
		if !route.PurposePermitsDuty(3, member.RoleDomain, member.Subrole) || !member.Current(now) {
			continue
		}
		conflicting := false
		for _, peer := range known {
			if route.Conflict(rendezvousPeer(member), peer) {
				conflicting = true
				break
			}
		}
		if conflicting {
			continue
		}
		if selected.NodeID != [32]byte{} {
			return network.RetainedDuty{}, errors.New("resolution duty ambiguous")
		}
		selected = member
	}
	if selected.NodeID == [32]byte{} {
		return network.RetainedDuty{}, errors.New("resolution duty absent")
	}
	return view.RetainDuty(selected.NodeID, now)
}
