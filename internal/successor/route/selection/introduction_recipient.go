package selection

import (
	"errors"

	"github.com/dianabuilds/ardents-network/internal/successor/network"
	"github.com/dianabuilds/ardents-network/internal/successor/route"
)

// IntroductionDuty resolves the unique current closed delivery duty, excluding
// both retained pairs and all locally known conflicting peers. It performs no
// dial, retry, fallback or new Entry/Interior selection. This is not the distinct
// general Rendezvous selection and preselected-alternative lifecycle.
func (leg Leg) IntroductionDuty(view network.RuntimeView, excluded []route.Member) (network.RetainedDuty, error) {
	if leg.EntryMember.RoleDomain != 4 || leg.Check(view, view.ObservedAt()) != nil {
		return network.RetainedDuty{}, errors.New("introduction prefix unavailable")
	}
	known := append([]route.Member(nil), leg.known...)
	known = append(known, excluded...)
	known = append(known, route.Member{NodeID: leg.EntryMember.NodeID, PublicKey: leg.EntryMember.PublicKey, FamilyID: leg.EntryMember.FamilyID}, route.Member{NodeID: leg.InteriorMember.NodeID, PublicKey: leg.InteriorMember.PublicKey, FamilyID: leg.InteriorMember.FamilyID})
	var selected network.Member
	for _, candidate := range view.Members() {
		if candidate.RoleDomain != 4 || candidate.Subrole != 3 || !candidate.Current(view.ObservedAt()) {
			continue
		}
		conflicts := false
		for _, peer := range known {
			if route.Conflict(route.Member{NodeID: candidate.NodeID, PublicKey: candidate.PublicKey, FamilyID: candidate.FamilyID}, peer) {
				conflicts = true
				break
			}
		}
		if conflicts {
			continue
		}
		if selected.NodeID != [32]byte{} {
			return network.RetainedDuty{}, errors.New("introduction delivery duty ambiguous")
		}
		selected = candidate
	}
	if selected.NodeID == [32]byte{} {
		return network.RetainedDuty{}, errors.New("introduction delivery duty absent")
	}
	return view.RetainDuty(selected.NodeID, view.ObservedAt())
}
