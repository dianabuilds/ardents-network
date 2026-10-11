package selection

import (
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network"
	"github.com/dianabuilds/ardents-network/internal/successor/route"
)

// Supplied-fact selection tests grant no signed State, token or transport.
func TestSubmissionExactDutyPreservesSourceAndKnownExclusions(t *testing.T) {
	now := time.Unix(1900000000, 0).UTC()
	view, source, responder := rendezvousModel(t, now, func(_ *network.ProfileFacts, records []network.NodeRecord, assignments []network.Assignment) {
		assignments[8].Subrole = 5
		assignments[9].RoleDomain, assignments[9].Subrole = 4, 3
		records[9].Assignment = "introduction"
	})
	want, err := view.RetainDuty([32]byte{10}, now)
	if err != nil {
		t.Fatal(err)
	}
	if duty, err := source.SubmissionDuty(view, want.NodeID, want.RecordGeneration, nil); err != nil || duty != want {
		t.Fatal("exact Introduction unavailable", duty, err)
	}
	for _, peer := range []route.Member{{NodeID: want.NodeID}, {PublicKey: want.PublicKey}, {FamilyID: want.FamilyID}} {
		if _, err := source.SubmissionDuty(view, want.NodeID, want.RecordGeneration, []route.Member{peer}); err == nil {
			t.Fatal("explicit peer conflict ignored", peer)
		}
		changed := source
		changed.known = append(append([]route.Member(nil), source.known...), peer)
		if _, err := changed.SubmissionDuty(view, want.NodeID, want.RecordGeneration, nil); err == nil {
			t.Fatal("retained peer conflict ignored", peer)
		}
	}
	if _, err := responder.SubmissionDuty(view, want.NodeID, want.RecordGeneration, nil); err == nil {
		t.Fatal("Responder became submission Source")
	}
	for _, id := range [][32]byte{{9}, {11}, {12}, {99}} {
		if _, err := source.SubmissionDuty(view, id, 1, nil); err == nil {
			t.Fatal("wrong/absent duty or alternate accepted", id)
		}
	}
	if _, err := source.SubmissionDuty(view, want.NodeID, 2, nil); err == nil {
		t.Fatal("foreign duty generation accepted")
	}
	for _, neighbor := range []byte{9, 12} {
		conflict, _, _ := rendezvousModel(t, now, func(_ *network.ProfileFacts, records []network.NodeRecord, assignments []network.Assignment) {
			assignments[8].Subrole = 5
			assignments[9].RoleDomain, assignments[9].Subrole = 4, 3
			records[9].Assignment = "introduction"
			records[9].FamilyID = records[neighbor-1].FamilyID
		})
		if _, err := source.SubmissionDuty(conflict, want.NodeID, 1, nil); err == nil {
			t.Fatal("issuer/resolution family conflict ignored", neighbor)
		}
	}
}
