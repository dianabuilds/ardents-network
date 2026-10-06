package selection

import (
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network"
	"github.com/dianabuilds/ardents-network/internal/successor/route"
)

func TestIssuerSelectionRetainsExactProfileAndAllKnownExclusions(t *testing.T) {
	now := time.Unix(1900000000, 0).UTC()
	view, leg, responder := rendezvousModel(t, now, nil)
	duty, err := leg.IssuerDuty(view)
	if err != nil || duty.NodeID != [32]byte{12} {
		t.Fatal("exact current issuer differs", err, duty.NodeID)
	}
	member, err := view.Member(duty.NodeID, now)
	if err != nil {
		t.Fatal(err)
	}
	for _, known := range []route.Member{{NodeID: member.NodeID}, {PublicKey: member.PublicKey}, {FamilyID: member.FamilyID}} {
		changed := leg
		changed.known = append(append([]route.Member(nil), leg.known...), known)
		if _, err := changed.IssuerDuty(view); err == nil {
			t.Fatal("issuer ignored retained conflict", known)
		}
	}
	if _, err := responder.IssuerDuty(view); err == nil {
		t.Fatal("Responder became issuer Source")
	}
	expired := leg
	expired.NotAfter = now
	if _, err := expired.IssuerDuty(view); err == nil {
		t.Fatal("expired leg accepted issuer")
	}
	if again, err := leg.IssuerDuty(view); err != nil || again != duty {
		t.Fatal("refusals changed original issuer", err)
	}
}

// These local rules use the supplied-fact Network model. They do not qualify
// authenticated intake, durable selection or an integrated Route operation.
func TestRetainedLegRejectsChangedAuthorityWithoutRebinding(t *testing.T) {
	now := time.Unix(1900000000, 0).UTC()
	view, original, _ := rendezvousModel(t, now, nil)
	if err := original.Check(view, now); err != nil {
		t.Fatal(err)
	}
	for _, loss := range []string{"profile", "Entry", "Interior", "expiry", "observation past expiry"} {
		t.Run(loss, func(t *testing.T) {
			observed := now
			leg := original
			if loss == "expiry" {
				leg.NotAfter = now
			}
			if loss == "observation past expiry" {
				observed = now.Add(time.Minute)
				leg.NotAfter = observed
			}
			changed, _, _ := rendezvousModelAt(t, now, observed, func(profile *network.ProfileFacts, records []network.NodeRecord, assignments []network.Assignment) {
				switch loss {
				case "profile":
					profile.Digest = [32]byte{200}
				case "Entry":
					records[0].Generation++
					assignments[0].Generation++
				case "Interior":
					records[2].Generation++
					assignments[2].Generation++
				}
			})
			// An older supplied time cannot undo the observation's expiry floor.
			if err := leg.Check(changed, now); err == nil {
				t.Fatal("changed or expired original authority accepted")
			}
			if err := original.Check(view, now); err != nil {
				t.Fatal("checking another observation mutated the retained leg", err)
			}
		})
	}
}
