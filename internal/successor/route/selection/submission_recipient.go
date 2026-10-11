package selection

import (
	"errors"

	"github.com/dianabuilds/ardents-network/internal/successor/network"
	"github.com/dianabuilds/ardents-network/internal/successor/route"
)

// SubmissionDuty checks the exact Introduction named by a caller's separately
// verified proof through this original Source. It chooses no alternative and
// accepts no supplied address, key, freshness flag or role authority.
func (leg Leg) SubmissionDuty(view network.RuntimeView, node [32]byte, generation uint64, excluded []route.Member) (network.RetainedDuty, error) {
	if leg.EntryMember.RoleDomain != 1 {
		return network.RetainedDuty{}, errors.New("submission requires original Source")
	}
	if err := leg.Check(view, view.ObservedAt()); err != nil {
		return network.RetainedDuty{}, err
	}
	member, err := view.Member(node, view.ObservedAt())
	if err != nil || member.DutyGeneration != generation || !route.PurposePermitsDuty(5, member.RoleDomain, member.Subrole) {
		return network.RetainedDuty{}, errors.Join(errors.New("exact Introduction submission duty unavailable"), err)
	}
	byKey, err := view.MemberByKey(member.PublicKey, view.ObservedAt())
	if err != nil || byKey.NodeID != member.NodeID {
		return network.RetainedDuty{}, errors.Join(errors.New("submission recipient key ambiguous"), err)
	}
	issuer, err := leg.IssuerDuty(view)
	if err != nil {
		return network.RetainedDuty{}, err
	}
	resolution, err := leg.ResolutionDuty(view, excluded)
	if err != nil {
		return network.RetainedDuty{}, err
	}
	known := append(append([]route.Member(nil), leg.known...), excluded...)
	known = append(known, rendezvousPeer(leg.EntryMember), rendezvousPeer(leg.InteriorMember),
		route.Member{NodeID: issuer.NodeID, PublicKey: issuer.PublicKey, FamilyID: issuer.FamilyID},
		route.Member{NodeID: resolution.NodeID, PublicKey: resolution.PublicKey, FamilyID: resolution.FamilyID})
	for _, peer := range known {
		if route.Conflict(rendezvousPeer(member), peer) {
			return network.RetainedDuty{}, errors.New("submission conflicts with known participant")
		}
	}
	return view.RetainDuty(member.NodeID, view.ObservedAt())
}
