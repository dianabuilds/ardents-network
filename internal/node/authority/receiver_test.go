package authority

import (
	"encoding/hex"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/route/carrier"
)

func TestReceiverRefusesDutyOrDigestMismatch(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	generation := [32]byte{1}
	snapshot := state.NodeDuty{Generation: hex.EncodeToString(generation[:]), NetworkID: [32]byte{2}, Epoch: 3, Digest: [32]byte{4},
		EpochValidFrom: now.Add(-time.Minute), ValidUntil: now.Add(time.Hour), Profile: carrier.ClosedRouteProfile, Fresh: true,
		NodeID: [32]byte{5}, RecordGeneration: 6, RecordValidUntil: now.Add(time.Hour)}
	profile := state.ClosedProfileView{NetworkID: snapshot.NetworkID, StateGeneration: generation, StateDigest: snapshot.Digest,
		Digest: [32]byte{7}, Epoch: snapshot.Epoch, NotBefore: now.Add(-time.Second), NotAfter: now.Add(time.Minute)}
	view := state.ClosedRouteView{Profile: profile, NodeCount: 1}
	view.Nodes[0] = state.ClosedRouteNodeView{NodeID: snapshot.NodeID, RecordDigest: [32]byte{8}, RoleDomain: 2, Subrole: 6, DutyGeneration: snapshot.RecordGeneration}
	source := Source{CurrentRoute: func() (state.ClosedRouteView, error) { return view, nil }}
	receiver, available := source.Receiver(snapshot, ardp.PurposeIssuer, now)
	if !available || receiver.DutyGeneration != snapshot.RecordGeneration || receiver.RecordDigest != view.Nodes[0].RecordDigest {
		t.Fatalf("closed route receiver = %+v / %t", receiver, available)
	}
	view.Nodes[0].DutyGeneration++
	if _, available := source.Receiver(snapshot, ardp.PurposeIssuer, now); available {
		t.Fatal("accepted a mismatched closed profile duty")
	}
	view.Nodes[0].DutyGeneration = snapshot.RecordGeneration
	view.Nodes[0].RecordDigest = [32]byte{}
	if _, available := source.Receiver(snapshot, ardp.PurposeIssuer, now); available {
		t.Fatal("accepted a missing closed profile record digest")
	}
	view.Nodes[0].RecordDigest = [32]byte{8}
	view.Profile.StateDigest = [32]byte{9}
	if _, available := source.Receiver(snapshot, ardp.PurposeIssuer, now); available {
		t.Fatal("accepted a successor State digest")
	}
}
