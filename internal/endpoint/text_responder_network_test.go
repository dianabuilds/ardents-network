//go:build linux

package endpoint

import (
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route"
	"testing"
)

// Distinct real listeners/keys are populated by startTextRoleNetwork before
// any Node starts. Accepted State itself remains the explicit test seam.
func addTextResponderPrefixState(source *textSourceStateFixture) {
	source.view.NodeCount, source.snapshot.CandidateCount = 15, 15
	for index := 11; index < 15; index++ {
		subrole := uint8(1)
		if index >= 13 {
			subrole = 2
		}
		id, record := fixtureID(byte(10+index)), fixtureID(byte(30+index))
		source.view.Nodes[index] = state.ClosedRouteNodeView{NodeID: id, RecordDigest: record, RoleDomain: 3, Subrole: subrole, DutyGeneration: uint64(index + 1)}
		candidate := source.snapshot.Candidates[4]
		candidate.NodeID, candidate.RecordDigest = id, record
		source.snapshot.Candidates[index] = candidate
	}
}

func TestTextResponderRejectsKnownIntroductionFamiliesBeforeIssuance(t *testing.T) {
	for _, carrier := range []route.CarrierProfile{route.ClosedCarrierTCP, route.ClosedCarrierQUIC} {
		t.Run(string(carrier), func(t *testing.T) {
			_, owner, source := startTextRoleNetwork(t, carrier, true, true)
			if _, err := owner.openTextPrefix(t.Context()); err != nil {
				t.Fatal(err)
			}
			if _, err := owner.openTextIntroductionPrefix(t.Context()); err != nil {
				t.Fatal(err)
			}
			source.mu.Lock()
			// Each Responder member stays distinct from its own pair and issuer.
			// Only the forbidden cross-domain observation has been introduced.
			for i := 11; i < 15; i++ {
				source.snapshot.Candidates[i].FamilyID = source.snapshot.Candidates[i-4].FamilyID
			}
			source.mu.Unlock()
			owner.mu.Lock()
			before := owner.permission.reserved
			owner.mu.Unlock()
			opened, err := owner.openTextPublisherPrefix(t.Context(), &owner.responder, 3)
			if err == nil || opened != nil {
				t.Error("Responder admitted a known Introduction family")
			}
			owner.mu.Lock()
			after := owner.permission.reserved
			owner.mu.Unlock()
			if before != after {
				t.Error("forbidden role consumed issuance before refusal")
			}
		})
	}
}
