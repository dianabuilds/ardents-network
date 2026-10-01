package forwarding

import (
	"encoding/hex"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/node/authority"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/route/carrier"
)

func TestClosedForwardRecipientRequiresExactStateRecipientAndRecord(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	generation := [32]byte{1}
	target, recordDigest, public := [32]byte{2}, [32]byte{3}, [32]byte{4}
	snapshot := state.NodeDuty{Generation: hex.EncodeToString(generation[:]), NetworkID: [32]byte{5}, Epoch: 6, Digest: [32]byte{7},
		EpochValidFrom: now.Add(-time.Minute), ValidUntil: now.Add(time.Hour), Profile: carrier.ClosedRouteProfile, Fresh: true,
		RecordValidUntil: now.Add(time.Hour), NodeID: [32]byte{10}, RecordGeneration: 1, CandidateCount: 1,
		Candidates: [64]state.NodeDutyCandidate{{NodeID: target, RecordDigest: recordDigest, PublicKey: public, Endpoint: "127.0.0.1:41000",
			CarrierProfile: string(carrier.ClosedCarrierTCP), ValidUntil: now.Add(time.Hour), AssignmentNotAfter: now.Add(time.Hour)}}}
	profile := state.ClosedProfileView{NetworkID: snapshot.NetworkID, StateGeneration: generation, StateDigest: snapshot.Digest,
		Digest: [32]byte{8}, Epoch: snapshot.Epoch, NotBefore: now.Add(-time.Second), NotAfter: now.Add(time.Minute)}
	view := state.ClosedRouteView{Profile: profile, NodeCount: 1}
	view.Nodes[0] = state.ClosedRouteNodeView{NodeID: target, RecordDigest: recordDigest, RoleDomain: 1, Subrole: 1, DutyGeneration: 9}
	source := authority.Source{CurrentRoute: func() (state.ClosedRouteView, error) { return view, nil }}
	open := route.ClosedOpen{NextNodeID: target, NextDutyGeneration: 9, Purpose: ardp.PurposeForwarding, Deadline: now.Add(time.Second)}
	candidate, err := recipient(source, snapshot, open, now, ValidCarrierEndpoint)
	if err != nil || candidate != snapshot.Candidates[0] {
		t.Fatalf("closed forward recipient = %+v / %v", candidate, err)
	}
	open.NextDutyGeneration++
	if _, err := recipient(source, snapshot, open, now, ValidCarrierEndpoint); err == nil {
		t.Fatal("accepted a forwarding OPEN with mismatched duty")
	}
	open.NextDutyGeneration--
	view.Nodes[0].RecordDigest[0]++
	if _, err := recipient(source, snapshot, open, now, ValidCarrierEndpoint); err == nil {
		t.Fatal("accepted a forwarding OPEN with mismatched record digest")
	}
	stateErr := errors.New("injected closed Route State failure")
	source.CurrentRoute = func() (state.ClosedRouteView, error) { return state.ClosedRouteView{}, stateErr }
	if _, err := recipient(source, snapshot, open, now, ValidCarrierEndpoint); err == nil {
		t.Fatal("accepted a forwarding OPEN without a current closed Route")
	}
}

func TestClosedForwardingUsesTransparentCarrierRelayWithoutChangingStatePeer(t *testing.T) {
	got, err := dialAddress("203.0.113.24:48127", "198.51.100.7:49128")
	if err != nil || got != "198.51.100.7:49128" {
		t.Fatalf("closed Carrier relay = %q / %v", got, err)
	}
	got, err = dialAddress("203.0.113.24:48127", "")
	if err != nil || got != "203.0.113.24:48127" {
		t.Fatalf("closed direct Carrier = %q / %v", got, err)
	}
}

func TestClosedForwardingRejectsInvalidCarrierRelayEndpoint(t *testing.T) {
	for _, input := range []struct{ advertised, relay string }{
		{"hostname.test:48127", "198.51.100.7:49128"},
		{"0.0.0.0:48127", "198.51.100.7:49128"},
		{"203.0.113.24:0", "198.51.100.7:49128"},
		{"203.0.113.24:48127", "relay.test:49128"},
		{"203.0.113.24:48127", "0.0.0.0:49128"},
		{"203.0.113.24:48127", "198.51.100.7:0"},
	} {
		if _, err := dialAddress(input.advertised, input.relay); err == nil || !strings.Contains(err.Error(), "Carrier relay endpoint") {
			t.Fatalf("closed Carrier relay %q -> %q error = %v", input.advertised, input.relay, err)
		}
	}
}
