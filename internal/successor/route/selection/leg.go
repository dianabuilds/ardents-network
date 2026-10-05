package selection

import (
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network"
	"github.com/dianabuilds/ardents-network/internal/successor/route"
)

// Leg binds a selected initial pair to this exact authenticated observation.
// Its alternatives are retained but this slice performs no automatic retry.
type Leg struct {
	Entry, Interior             network.RetainedDuty
	EntryMember, InteriorMember network.Member
	Profile                     network.ProfileBinding
	NotAfter                    time.Time
	known                       []route.Member
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

func minTime(first time.Time, rest ...time.Time) time.Time {
	for _, value := range rest {
		if value.Before(first) {
			first = value
		}
	}
	return first
}
