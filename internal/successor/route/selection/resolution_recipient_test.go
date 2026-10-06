package selection

import (
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network"
	"github.com/dianabuilds/ardents-network/internal/successor/route"
)

// Supplied-fact models isolate Route selection. Signed State and actual
// resolution admission remain required in command composition.
func TestResolutionRecipientRequiresSoleCurrentUnconflictedSourceDuty(t *testing.T) {
	now := time.Unix(1900000000, 0).UTC()
	model := func(second bool) (network.RuntimeView, Leg, Leg) {
		return rendezvousModel(t, now, func(_ *network.ProfileFacts, _ []network.NodeRecord, assignments []network.Assignment) {
			assignments[8].Subrole = 5
			if second {
				assignments[9].Subrole = 5
			}
		})
	}
	view, leg, responder := model(false)
	duty, err := leg.ResolutionDuty(view, nil)
	if err != nil || duty.NodeID != [32]byte{9} {
		t.Fatal("sole resolution recipient differs", duty, err)
	}
	member, err := view.Member(duty.NodeID, now)
	if err != nil {
		t.Fatal(err)
	}
	for _, peer := range []route.Member{{NodeID: member.NodeID}, {PublicKey: member.PublicKey}, {FamilyID: member.FamilyID}} {
		if _, err := leg.ResolutionDuty(view, []route.Member{peer}); err == nil {
			t.Fatal("explicit conflict ignored", peer)
		}
		changed := leg
		changed.known = append(append([]route.Member(nil), leg.known...), peer)
		if _, err := changed.ResolutionDuty(view, nil); err == nil {
			t.Fatal("retained conflict ignored", peer)
		}
	}
	if _, err := responder.ResolutionDuty(view, nil); err == nil {
		t.Fatal("Responder became resolution Source")
	}
	ambiguous, _, _ := model(true)
	if _, err := leg.ResolutionDuty(ambiguous, nil); err == nil {
		t.Fatal("two eligible resolution duties silently selected")
	}
	absent, _, _ := rendezvousModel(t, now, nil)
	if _, err := leg.ResolutionDuty(absent, nil); err == nil {
		t.Fatal("Rendezvous/issuer substituted for missing resolution")
	}
	expired := leg
	expired.NotAfter = now
	if _, err := expired.ResolutionDuty(view, nil); err == nil {
		t.Fatal("expired original Source accepted")
	}
	if again, err := leg.ResolutionDuty(view, nil); err != nil || again != duty {
		t.Fatal("refusal mutated original selection", err)
	}
}
