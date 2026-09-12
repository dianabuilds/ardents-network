//go:build linux

package route

import (
	"encoding/hex"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
)

type resolutionSelectionState struct {
	snapshot state.Snapshot
	view     state.ClosedRouteView
}

func (source *resolutionSelectionState) Current() (state.Snapshot, error) {
	return source.snapshot, nil
}
func (source *resolutionSelectionState) CurrentClosedRoute() (state.ClosedRouteView, error) {
	return source.view, nil
}

func sourceResolutionSelectionFixture(t *testing.T) (*ClosedSourcePrefix, *resolutionSelectionState) {
	t.Helper()
	now := time.Now().UTC()
	profile := state.ClosedProfileView{NetworkID: [32]byte{1}, StateGeneration: [32]byte{2}, StateDigest: [32]byte{3}, Digest: [32]byte{4},
		IssuerNodeID: [32]byte{12}, IssuerDutyGeneration: 3, Epoch: 1, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour)}
	source := &resolutionSelectionState{view: state.ClosedRouteView{Profile: profile, NodeCount: 4},
		snapshot: state.Snapshot{NetworkID: profile.NetworkID, Generation: hex.EncodeToString(profile.StateGeneration[:]), Digest: profile.StateDigest,
			Epoch: profile.Epoch, Profile: ClosedRouteProfile, Freshness: "fresh", EpochValidFrom: profile.NotBefore, ValidUntil: profile.NotAfter, CandidateCount: 4}}
	for index := 0; index < 4; index++ {
		domain, subrole := uint8(1), uint8(index+1)
		if index >= 2 {
			domain, subrole = 2, uint8(8-index)
		}
		node, record := [32]byte{byte(10 + index)}, [32]byte{byte(20 + index)}
		source.view.Nodes[index] = state.ClosedRouteNodeView{NodeID: node, RecordDigest: record, RoleDomain: domain, Subrole: subrole, DutyGeneration: uint64(index + 1)}
		peer := &source.snapshot.Candidates[index]
		peer.NodeID, peer.RecordDigest, peer.PublicKey, peer.FamilyID = node, record, [32]byte{byte(30 + index)}, [32]byte{byte(40 + index)}
		peer.ValidFrom, peer.ValidUntil, peer.AssignmentNotAfter = profile.NotBefore, profile.NotAfter, profile.NotAfter
		peer.CarrierProfile, peer.Endpoint, peer.Capacity = string(ClosedCarrierTCP), "127.0.0.1:1", 1
	}
	selection := ClosedBootstrapSelection{ProfileDigest: profile.Digest, EntryNodeID: source.view.Nodes[0].NodeID, InteriorNodeID: source.view.Nodes[1].NodeID}
	plan, err := prepareClosedBootstrap(source, selection, now)
	if err != nil {
		t.Fatal(err)
	}
	// No transport exists in this selection test; invalid State must stop before
	// any OPEN or presenter. Actual networking has its separate Endpoint test.
	return &ClosedSourcePrefix{source: source, selection: selection, plan: plan, channels: &closedSourceChannels{}}, source
}

func TestClosedSourceResolutionRequiresUniqueCurrentStateRecipient(t *testing.T) {
	prefix, source := sourceResolutionSelectionFixture(t)
	if node, err := prefix.ResolutionRecipient(); err != nil || node != source.view.Nodes[3].NodeID {
		t.Fatalf("selected recipient: %x %v", node, err)
	}
	for name, mutate := range map[string]func(*resolutionSelectionState){
		"missing": func(source *resolutionSelectionState) { source.view.NodeCount = 3 },
		"duplicate duty": func(source *resolutionSelectionState) {
			source.view.Nodes[4] = source.view.Nodes[3]
			source.view.NodeCount = 5
		},
		"wrong domain":        func(source *resolutionSelectionState) { source.view.Nodes[3].RoleDomain = 4 },
		"zero duty":           func(source *resolutionSelectionState) { source.view.Nodes[3].DutyGeneration = 0 },
		"record substitution": func(source *resolutionSelectionState) { source.snapshot.Candidates[3].RecordDigest[0]++ },
		"expired assignment": func(source *resolutionSelectionState) {
			source.snapshot.Candidates[3].AssignmentNotAfter = time.Now().Add(-time.Second)
		},
		"source family": func(source *resolutionSelectionState) {
			source.snapshot.Candidates[3].FamilyID = source.snapshot.Candidates[0].FamilyID
		},
		"source key": func(source *resolutionSelectionState) {
			source.snapshot.Candidates[3].PublicKey = source.snapshot.Candidates[1].PublicKey
		},
		"caller address": func(source *resolutionSelectionState) {
			source.snapshot.Candidates[3].Endpoint = "https://example.invalid"
		},
		"duplicate record": func(source *resolutionSelectionState) {
			source.snapshot.Candidates[4] = source.snapshot.Candidates[3]
			source.snapshot.CandidateCount = 5
		},
		"State conflict": func(source *resolutionSelectionState) { source.snapshot.Conflicting = true },
	} {
		t.Run(name, func(t *testing.T) {
			prefix, source := sourceResolutionSelectionFixture(t)
			mutate(source)
			if _, err := prefix.ResolutionRecipient(); err == nil {
				t.Fatal("invalid State recipient selected")
			}
			called := false
			_, _, err := prefix.ExchangeDescriptor(t.Context(), func(ClosedHello, uint8) ([]byte, error) { called = true; return nil, nil }, [32]byte{91}, nil)
			if err == nil || called {
				t.Fatal("invalid recipient reached network/presenter")
			}
		})
	}
}
