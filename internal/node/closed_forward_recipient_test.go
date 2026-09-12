package node

import (
	"encoding/hex"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route"
)

func TestClosedForwardRecipientRequiresExactStateRecipientAndRecord(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	generation := [32]byte{1}
	target, recordDigest, public := [32]byte{2}, [32]byte{3}, [32]byte{4}
	snapshot := dutyFacts{Generation: hex.EncodeToString(generation[:]), NetworkID: [32]byte{5}, Epoch: 6, Digest: [32]byte{7},
		EpochValidFrom: now.Add(-time.Minute), ValidUntil: now.Add(time.Hour), Profile: route.ClosedRouteProfile, Fresh: true,
		RecordValidUntil: now.Add(time.Hour), CandidateCount: 1,
		Candidates: [64]dutyCandidate{{NodeID: target, RecordDigest: recordDigest, PublicKey: public, Endpoint: "127.0.0.1:41000",
			CarrierProfile: string(route.ClosedCarrierTCP), ValidUntil: now.Add(time.Hour), AssignmentNotAfter: now.Add(time.Hour)}}}
	profile := state.ClosedProfileView{NetworkID: snapshot.NetworkID, StateGeneration: generation, StateDigest: snapshot.Digest,
		Digest: [32]byte{8}, Epoch: snapshot.Epoch, NotBefore: now.Add(-time.Second), NotAfter: now.Add(time.Minute)}
	view := state.ClosedRouteView{Profile: profile, NodeCount: 1}
	view.Nodes[0] = state.ClosedRouteNodeView{NodeID: target, RecordDigest: recordDigest, RoleDomain: 1, Subrole: 1, DutyGeneration: 9}
	config := runtimeConfig{Config: Config{CurrentClosedRoute: func() (state.ClosedRouteView, bool) { return view, true }}}
	open := route.ClosedOpen{NextNodeID: target, NextDutyGeneration: 9, Purpose: route.ClosedPurposeForwarding, Deadline: now.Add(time.Second)}
	candidate, err := closedForwardRecipient(config, snapshot, open, now)
	if err != nil || candidate != snapshot.Candidates[0] {
		t.Fatalf("closed forward recipient = %+v / %v", candidate, err)
	}
	open.NextDutyGeneration++
	if _, err := closedForwardRecipient(config, snapshot, open, now); err == nil {
		t.Fatal("accepted a forwarding OPEN with mismatched duty")
	}
	open.NextDutyGeneration--
	view.Nodes[0].RecordDigest[0]++
	if _, err := closedForwardRecipient(config, snapshot, open, now); err == nil {
		t.Fatal("accepted a forwarding OPEN with mismatched record digest")
	}
}
