//go:build linux

package receiver

import (
	"errors"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network"
	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/successor/route/role"
)

// Supplied-fact boundary controls, not authenticated intake or installed proof.
// The native Descriptor driver separately supplies actual signed State/Startup.
func descriptorOuterView(t *testing.T, now time.Time, damage string) network.RuntimeView {
	t.Helper()
	b := network.ProfileBinding{Network: [32]byte{101}, Generation: [32]byte{102}, EpochDigest: [32]byte{103}, Digest: [32]byte{104}, Epoch: 1, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour)}
	if damage == "profile" {
		b.Digest[0]++
	}
	p := network.ProfileFacts{ProfileBinding: b, IssuanceAuthorityKey: [32]byte{106}, IssuerNodeID: [32]byte{2}, IssuerDutyGeneration: 1, TokenKeys: []network.TokenKey{{WindowStart: now.Truncate(time.Hour), Class: 1, SPKI: [346]byte{1}}}}
	var records []network.NodeRecord
	var assignments []network.Assignment
	for id := byte(1); id <= 3; id++ {
		domain, subrole, name := uint8(2), uint8(5), "rendezvous"
		if id == 2 {
			subrole = 6
		}
		if id == 3 {
			domain, subrole, name = 1, 2, "initiator"
		}
		r := network.NodeRecord{NodeID: [32]byte{id}, RecordDigest: [32]byte{id + 60}, PublicKey: [32]byte{id + 20}, FamilyID: [32]byte{id + 40}, Generation: 1, Assignment: name, CarrierProfile: "ardents-carrier-tcp-tls-v2", Endpoint: "127.0.0.1:2000", Capacity: 1, ValidFrom: b.NotBefore, ValidUntil: b.NotAfter, AssignmentNotAfter: b.NotAfter}
		if id == 1 && damage == "local-generation" {
			r.Generation++
		}
		if id == 3 {
			switch damage {
			case "peer-generation":
				r.Generation++
			case "peer-record":
				r.RecordDigest[0]++
			case "peer-key":
				r.PublicKey[0]++
			case "peer-family":
				r.FamilyID = [32]byte{41}
			case "peer-duty":
				subrole = 1
			case "peer-carrier":
				r.CarrierProfile = "ardents-carrier-quic-v2"
			}
		}
		records = append(records, r)
		assignments = append(assignments, network.Assignment{NodeID: r.NodeID, RecordDigest: r.RecordDigest, Generation: r.Generation, RoleDomain: domain, Subrole: subrole})
	}
	membership, err := network.BindMembership(b, assignments, records)
	if err != nil {
		t.Fatal(err)
	}
	accepted, err := network.BindAcceptedState(network.EpochFacts{Network: b.Network, Number: b.Epoch, Digest: b.EpochDigest, ValidFrom: b.NotBefore, ValidUntil: b.NotAfter}, b.Generation, p, membership)
	if err != nil {
		t.Fatal(err)
	}
	clock, err := network.ConfirmTime(network.ClockEvidence{Wall: now, Independent: now}, 0)
	if err != nil {
		t.Fatal(err)
	}
	view, err := accepted.Observe(clock)
	if err != nil {
		t.Fatal(err)
	}
	return view
}

func TestDescriptorOuterPeerUsesOneCoherentFreshViewPerFrame(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	original := descriptorOuterView(t, now, "")
	duty, err := original.RetainDuty([32]byte{1}, now)
	if err != nil {
		t.Fatal(err)
	}
	local, err := original.Member(duty.NodeID, now)
	if err != nil {
		t.Fatal(err)
	}
	peer, err := original.Member([32]byte{3}, now)
	if err != nil {
		t.Fatal(err)
	}
	current, calls := original, 0
	var observationError error
	r := &Receiver{config: ReceiverConfig{Authority: role.Authority{Duty: duty, Profile: original.Profile().ProfileBinding, Current: func() (network.RuntimeView, error) {
		calls++
		return current, observationError
	}}}}
	h := ardp.Hello{NetworkID: original.Profile().Network, StateGeneration: original.Profile().Generation, StateDigest: original.Profile().EpochDigest, ProfileDigest: original.Profile().Digest, RecipientNodeID: local.NodeID, RecipientDutyGeneration: local.DutyGeneration, Purpose: ardp.PurposeForwarding, Deadline: now.Add(time.Minute)}
	for frame := range 2 {
		if err := r.checkOuterPeer(h, local, peer, peer.PublicKey); err != nil || calls != frame+1 {
			t.Fatal("one coherent observation required at each fresh frame boundary", calls, err)
		}
	}
	lost := errors.New("original State observation lost after the preceding frame")
	for _, damage := range []string{"profile", "local-generation", "peer-generation", "peer-record", "peer-key", "peer-family", "peer-duty", "peer-carrier", "observer", "hello"} {
		current, observationError = descriptorOuterView(t, now, damage), nil
		request := h
		if damage == "observer" {
			observationError = lost
		}
		if damage == "hello" {
			request.RecipientDutyGeneration++
		}
		before := calls
		err := r.checkOuterPeer(request, local, peer, peer.PublicKey)
		if err == nil || calls != before+1 || observationError != nil && !errors.Is(err, lost) {
			t.Fatal("a preceding accepting frame must not authorize changed original facts", damage, calls-before, err)
		}
	}
}
