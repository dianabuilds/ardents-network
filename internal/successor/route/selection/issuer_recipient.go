package selection

import (
	"errors"
	"github.com/dianabuilds/ardents-network/internal/successor/network"
	"github.com/dianabuilds/ardents-network/internal/successor/route"
)

// IssuerDuty selects only the one authenticated current issuer, excluding all
// identities/families retained by this leg. It never draws a replacement.
func (leg Leg) IssuerDuty(view network.RuntimeView) (network.RetainedDuty, error) {
	now := view.ObservedAt()
	if leg.EntryMember.RoleDomain != 1 {
		return network.RetainedDuty{}, errors.New("issuer requires original Domain-1 leg")
	}
	if err := leg.Check(view, now); err != nil {
		return network.RetainedDuty{}, err
	}
	member, err := view.Member(view.Profile().IssuerNodeID, now)
	if err != nil || member.RoleDomain != 2 || member.Subrole != 6 {
		return network.RetainedDuty{}, errors.Join(errors.New("exact current issuer unavailable"), err)
	}
	known := append([]route.Member(nil), leg.known...)
	known = append(known, rendezvousPeer(leg.EntryMember), rendezvousPeer(leg.InteriorMember))
	for _, peer := range known {
		if route.Conflict(rendezvousPeer(member), peer) {
			return network.RetainedDuty{}, errors.New("issuer conflicts with known participant")
		}
	}
	return view.RetainDuty(member.NodeID, now)
}
