package selection

import (
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/route"
)

// This supplied-fact model checks local eligibility, not authenticated intake.
func TestSelectionCandidatesKeepExactDutyAndApplyEveryIdentityExclusion(t *testing.T) {
	now := time.Unix(1_900_000_000, 0).UTC()
	view, _, _ := rendezvousModel(t, now, nil)
	for _, subrole := range []uint8{1, 2} {
		selected := selectionCandidates(view, subrole, nil)
		if len(selected) != 4 {
			t.Fatal("wrong number of eligible adjacent duties", len(selected))
		}
		for _, candidate := range selected {
			member, err := view.Member(candidate.NodeID, now)
			if err != nil || member.Subrole != subrole || candidate != (ClosedSetMember{NodeID: member.NodeID, PublicKey: member.PublicKey, FamilyID: member.FamilyID, RecordDigest: member.RecordDigest, DutyGeneration: member.DutyGeneration, Domain: member.RoleDomain, NotAfter: member.NotAfter()}) {
				t.Fatal("candidate lost its exact public duty binding", err)
			}
		}
		first := selected[0]
		for _, excluded := range []route.Member{{NodeID: first.NodeID}, {PublicKey: first.PublicKey}, {FamilyID: first.FamilyID}} {
			remaining := selectionCandidates(view, subrole, []route.Member{excluded})
			if len(remaining) != 3 {
				t.Fatal("identity exclusion changed unrelated duties", len(remaining))
			}
			for _, candidate := range remaining {
				if candidate.NodeID == first.NodeID {
					t.Fatal("conflicting candidate retained")
				}
			}
		}
	}
}

func entryRuleView() ClosedSetView {
	now := time.Unix(1_800_000_000, 0).UTC()
	return ClosedSetView{NetworkID: [32]byte{1}, Now: now, Candidates: []ClosedSetMember{
		{NodeID: [32]byte{2}, PublicKey: [32]byte{12}, FamilyID: [32]byte{22}, RecordDigest: [32]byte{32}, DutyGeneration: 1, Domain: 1, NotAfter: now.Add(12 * time.Hour)},
		{NodeID: [32]byte{3}, PublicKey: [32]byte{13}, FamilyID: [32]byte{23}, RecordDigest: [32]byte{33}, DutyGeneration: 2, Domain: 1, NotAfter: now.Add(12 * time.Hour)},
	}}
}

func TestEntryPairRefusesEachIdentityConflict(t *testing.T) {
	for _, conflict := range []string{"node", "key", "family", "domain"} {
		t.Run(conflict, func(t *testing.T) {
			view := entryRuleView()
			first, second := &view.Candidates[0], &view.Candidates[1]
			switch conflict {
			case "node":
				second.NodeID = first.NodeID
			case "key":
				second.PublicKey = first.PublicKey
			case "family":
				second.FamilyID = first.FamilyID
			case "domain":
				second.Domain = 3
			}
			if _, err := chooseClosedEntryPair(view, 1); err == nil {
				t.Fatal("unavailable nonconflicting Entry pair accepted")
			}
		})
	}
}

func TestEntryPairRetainsBothMembersAndAbsoluteBound(t *testing.T) {
	for _, bound := range []time.Duration{6 * time.Hour, 45 * time.Minute} {
		t.Run(bound.String(), func(t *testing.T) {
			view := entryRuleView()
			if bound < 6*time.Hour {
				view.Candidates[1].NotAfter = view.Now.Add(bound)
			}
			selected, err := chooseClosedEntryPair(view, 1)
			if err != nil {
				t.Fatal(err)
			}
			if selected.Chosen != view.Now || selected.NotAfter != view.Now.Add(bound) {
				t.Fatalf("original bound changed: %+v", selected)
			}
			first, second := view.Candidates[0], view.Candidates[1]
			if selected.Members != [2]ClosedSetMember{first, second} && selected.Members != [2]ClosedSetMember{second, first} {
				t.Fatal("selection changed public bindings or omitted an alternative")
			}
		})
	}
}

func TestEntryViewRefusesForeignExpiredDuplicateAndMalformedBindings(t *testing.T) {
	view := entryRuleView()
	if !validClosedSetView(view, view.NetworkID) {
		t.Fatal("valid public view refused")
	}
	for _, fault := range []string{"network", "expiry", "duplicate", "generation", "reserved-domain", "subsecond-expiry"} {
		t.Run(fault, func(t *testing.T) {
			view := entryRuleView()
			network := view.NetworkID
			switch fault {
			case "network":
				network = [32]byte{9}
			case "expiry":
				view.Candidates[0].NotAfter = view.Now
			case "duplicate":
				view.Candidates[1].NodeID = view.Candidates[0].NodeID
			case "generation":
				view.Candidates[0].DutyGeneration = 0
			case "reserved-domain":
				view.Candidates[0].Domain = 2
			case "subsecond-expiry":
				view.Candidates[0].NotAfter = view.Candidates[0].NotAfter.Add(time.Nanosecond)
			}
			if validClosedSetView(view, network) {
				t.Fatal("invalid public view accepted")
			}
		})
	}
}
