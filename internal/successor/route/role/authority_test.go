package role

import (
	"errors"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network"
	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
)

// These supplied-fact observations exercise channel binding rules. Genuine
// authenticated intake and admitted Carrier operations belong to command tests.
func TestAuthorityReobservesWithoutRebinding(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	view := authorityObservation(t, now, 1, [32]byte{104})
	duty, err := view.RetainDuty([32]byte{1}, now)
	if err != nil {
		t.Fatal(err)
	}
	current := view
	calls := 0
	lost := errors.New("original observation unavailable")
	var failure error
	a := Authority{Duty: duty, Profile: view.Profile().ProfileBinding, Current: func() (network.RuntimeView, error) {
		calls++
		return current, failure
	}}
	if _, err := a.Member(); err != nil {
		t.Fatal(err)
	}
	for _, change := range []string{"profile", "duty", "observation"} {
		t.Run(change, func(t *testing.T) {
			current, failure = view, nil
			switch change {
			case "profile":
				current = authorityObservation(t, now, 1, [32]byte{105})
			case "duty":
				current = authorityObservation(t, now, 2, [32]byte{104})
			case "observation":
				failure = lost
			}
			before := calls
			if _, err := a.Member(); err == nil || (failure != nil && !errors.Is(err, lost)) {
				t.Fatal("original authority loss accepted or hidden", err)
			}
			if calls != before+1 || a.Duty != duty || a.Profile != view.Profile().ProfileBinding {
				t.Fatal("authority cached or rebound")
			}
		})
	}
	if _, err := (Authority{}).Member(); err == nil {
		t.Fatal("missing observer accepted")
	}
}

func TestHelloRejectsChangedBindingPurposeAndBounds(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	view := authorityObservation(t, now, 1, [32]byte{104})
	duty, err := view.RetainDuty([32]byte{1}, now)
	if err != nil {
		t.Fatal(err)
	}
	p := view.Profile().ProfileBinding
	a := Authority{Current: func() (network.RuntimeView, error) { return view, nil }, Duty: duty, Profile: p}
	h := ardp.Hello{NetworkID: p.Network, StateGeneration: p.Generation, StateDigest: p.EpochDigest, ProfileDigest: p.Digest, RecipientNodeID: duty.NodeID, RecipientDutyGeneration: duty.RecordGeneration, Purpose: ardp.PurposeForwarding, Deadline: now.Add(time.Minute)}
	for _, outer := range []bool{false, true} {
		if _, err := a.Hello(h, outer); err != nil {
			t.Fatal("original forwarding binding refused", err)
		}
		for _, damage := range []string{"network", "generation", "state", "profile", "recipient", "duty", "purpose", "expired", "horizon"} {
			t.Run(damage+map[bool]string{false: "/inner", true: "/outer"}[outer], func(t *testing.T) {
				changed := h
				switch damage {
				case "network":
					changed.NetworkID[0]++
				case "generation":
					changed.StateGeneration[0]++
				case "state":
					changed.StateDigest[0]++
				case "profile":
					changed.ProfileDigest[0]++
				case "recipient":
					changed.RecipientNodeID[0]++
				case "duty":
					changed.RecipientDutyGeneration++
				case "purpose":
					changed.Purpose = ardp.PurposeDataJoin
				case "expired":
					changed.Deadline = now
				case "horizon":
					changed.Deadline = p.NotAfter.Add(time.Second)
				}
				if _, err := a.Hello(changed, outer); err == nil {
					t.Fatal("changed channel accepted")
				}
			})
		}
	}
}

func TestFreshHelloCannotPublishAfterObservationLoss(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	view := authorityObservation(t, now, 1, [32]byte{104})
	duty, err := view.RetainDuty([32]byte{1}, now)
	if err != nil {
		t.Fatal(err)
	}
	lost := errors.New("observation lost before HELLO handoff")
	calls := 0
	a := Authority{Duty: duty, Profile: view.Profile().ProfileBinding, Current: func() (network.RuntimeView, error) {
		calls++
		if calls == 2 {
			return network.RuntimeView{}, lost
		}
		return view, nil
	}}
	h, err := a.FreshHello(now.Add(time.Minute), ardp.PurposeForwarding, false)
	if !errors.Is(err, lost) || h != (ardp.Hello{}) || calls != 2 {
		t.Fatal("HELLO published after authority loss", h, err, calls)
	}
	a.Current = func() (network.RuntimeView, error) { return view, nil }
	end := now.Add(time.Minute)
	first, err := a.FreshHello(end, ardp.PurposeForwarding, false)
	if err != nil {
		t.Fatal(err)
	}
	second, err := a.FreshHello(end, ardp.PurposeForwarding, false)
	if err != nil {
		t.Fatal(err)
	}
	if first.ChannelNonce == [32]byte{} || first.ChannelNonce == second.ChannelNonce || first.Deadline != end || second.Deadline != end || first.RecipientNodeID != duty.NodeID || second.RecipientNodeID != duty.NodeID {
		t.Fatal("fresh nonce changed original recipient/bounds or was reused")
	}
}

func TestParticipantExclusionKeepsNodeKeyAndFamilyDistinct(t *testing.T) {
	first := network.Member{NodeID: [32]byte{1}, PublicKey: [32]byte{2}, FamilyID: [32]byte{3}}
	other := network.Member{NodeID: [32]byte{4}, PublicKey: [32]byte{5}, FamilyID: [32]byte{6}}
	if Conflicting(first, other) {
		t.Fatal("distinct participants excluded")
	}
	for _, field := range []string{"Node", "key", "family"} {
		conflict := other
		switch field {
		case "Node":
			conflict.NodeID = first.NodeID
		case "key":
			conflict.PublicKey = first.PublicKey
		case "family":
			conflict.FamilyID = first.FamilyID
		}
		if !Conflicting(first, conflict) {
			t.Fatal("participant conflict ignored", field)
		}
	}
}

func authorityObservation(t *testing.T, now time.Time, generation uint64, digest [32]byte) network.RuntimeView {
	t.Helper()
	binding := network.ProfileBinding{Network: [32]byte{101}, Generation: [32]byte{102}, EpochDigest: [32]byte{103}, Digest: digest, Epoch: 1, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour)}
	profile := network.ProfileFacts{ProfileBinding: binding, IssuanceAuthorityKey: [32]byte{106}, IssuerNodeID: [32]byte{2}, IssuerDutyGeneration: 1, TokenKeys: []network.TokenKey{{WindowStart: now.Truncate(time.Hour), Class: 2, SPKI: [346]byte{1}}}}
	var records []network.NodeRecord
	var assignments []network.Assignment
	for id := byte(1); id <= 2; id++ {
		domain, subrole, name, recordGeneration := uint8(1), uint8(1), "initiator", generation
		if id == 2 {
			domain, subrole, name, recordGeneration = 2, 6, "rendezvous", 1
		}
		record := network.NodeRecord{NodeID: [32]byte{id}, RecordDigest: [32]byte{id + 60}, PublicKey: [32]byte{id + 20}, FamilyID: [32]byte{id + 40}, Generation: recordGeneration, Assignment: name, CarrierProfile: "ardents-carrier-tcp-tls-v2", Endpoint: "127.0.0.1:2000", Capacity: 1, ValidFrom: binding.NotBefore, ValidUntil: binding.NotAfter, AssignmentNotAfter: binding.NotAfter}
		records = append(records, record)
		assignments = append(assignments, network.Assignment{NodeID: record.NodeID, RecordDigest: record.RecordDigest, Generation: recordGeneration, RoleDomain: domain, Subrole: subrole})
	}
	membership, err := network.BindMembership(binding, assignments, records)
	if err != nil {
		t.Fatal(err)
	}
	accepted, err := network.BindAcceptedState(network.EpochFacts{Network: binding.Network, Number: binding.Epoch, Digest: binding.EpochDigest, ValidFrom: binding.NotBefore, ValidUntil: binding.NotAfter}, binding.Generation, profile, membership)
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
