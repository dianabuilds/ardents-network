package network_test

import (
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network"
)

func TestAcceptedStateRejectsMixedGenerationsAndProfiles(t *testing.T) {
	now := time.Unix(1800000000, 0).UTC()
	profile := membershipProfile(now)
	assignments, records := membershipInputs(now)
	// A current member may have a shorter interval than the independently
	// signed profile. Its work is bounded by its Record, not by rewriting the
	// profile or refusing all use before that member actually expires.
	records[0].ValidUntil = now.Add(30 * time.Second)
	membership, err := network.BindMembership(profile, assignments, records)
	if err != nil {
		t.Fatal(err)
	}
	epoch := network.EpochFacts{Network: profile.Network, Number: profile.Epoch, Digest: profile.EpochDigest, ValidFrom: profile.NotBefore, ValidUntil: profile.NotAfter}
	publicProfile := network.ProfileFacts{ProfileBinding: profile, IssuanceAuthorityKey: [32]byte{7},
		IssuerNodeID: records[0].NodeID, IssuerDutyGeneration: records[0].Generation,
		TokenKeys: []network.TokenKey{{WindowStart: now, Class: 1, SPKI: [346]byte{1}}}}
	for _, defect := range []string{"network", "generation", "Epoch digest", "Epoch number", "profile digest", "window"} {
		t.Run(defect, func(t *testing.T) {
			mixed := profile
			switch defect {
			case "network":
				mixed.Network = [32]byte{9}
			case "generation":
				mixed.Generation = [32]byte{9}
			case "Epoch digest":
				mixed.EpochDigest = [32]byte{9}
			case "Epoch number":
				mixed.Epoch++
			case "profile digest":
				mixed.Digest = [32]byte{9}
			case "window":
				mixed.NotAfter = epoch.ValidUntil.Add(time.Second)
			}
			mixedPublic := publicProfile
			mixedPublic.ProfileBinding = mixed
			if _, err := network.BindAcceptedState(epoch, profile.Generation, mixedPublic, membership); err == nil {
				t.Fatal("mixed authority accepted")
			}
		})
	}
	accepted, err := network.BindAcceptedState(epoch, profile.Generation, publicProfile, membership)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := accepted.Observe(network.TrustedTime{}); err == nil {
		t.Fatal("missing time observation accepted")
	}
	for _, instant := range []time.Time{profile.NotBefore.Add(-time.Nanosecond), profile.NotAfter} {
		observed, err := network.ConfirmTime(network.ClockEvidence{Wall: instant, Independent: instant}, 0)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := accepted.Observe(observed); err == nil {
			t.Fatal("unavailable profile observed")
		}
	}
	observed, err := network.ConfirmTime(network.ClockEvidence{Wall: now, Independent: now}, 0)
	if err != nil {
		t.Fatal(err)
	}
	view, err := accepted.Observe(observed)
	if err != nil {
		t.Fatal(err)
	}
	duty, err := view.RetainDuty(records[0].NodeID, now)
	if err != nil || duty.Epoch != epoch || duty.Generation != profile.Generation || duty.RecordGeneration != records[0].Generation {
		t.Fatalf("retained duty did not come from this observation: %+v / %v", duty, err)
	}
	if _, err := view.RetainDuty([32]byte{255}, now); err == nil {
		t.Fatal("retained unknown participant")
	}
	if _, err := (network.RuntimeView{}).RetainDuty(records[0].NodeID, now); err == nil {
		t.Fatal("retained duty from zero observation")
	}
	if err := view.MatchDuty(duty, now); err != nil {
		t.Fatalf("positive duty binding: %v", err)
	}
	for _, defect := range []string{"Epoch start", "Epoch end", "Record start", "Record end"} {
		t.Run(defect, func(t *testing.T) {
			changed := duty
			switch defect {
			case "Epoch start":
				changed.Epoch.ValidFrom = changed.Epoch.ValidFrom.Add(-time.Second)
			case "Epoch end":
				changed.Epoch.ValidUntil = changed.Epoch.ValidUntil.Add(time.Second)
			case "Record start":
				changed.RecordValidFrom = changed.RecordValidFrom.Add(-time.Second)
			case "Record end":
				changed.RecordValidUntil = changed.RecordValidUntil.Add(time.Second)
			}
			if err := view.MatchDuty(changed, now); err == nil {
				t.Fatal("caller altered an authenticated interval")
			}
		})
	}
	if err := view.MatchDuty(duty, records[0].ValidUntil); err == nil {
		t.Fatal("expired member retained duty authority under a live profile")
	}
	duty.RecordGeneration++
	if err := view.MatchDuty(duty, now); err == nil {
		t.Fatal("old assignment accepted")
	}
	if err := view.Check(profile.NotAfter); err == nil {
		t.Fatal("observation became a permanent lease")
	}
	if err := (network.RuntimeView{}).Check(now); err == nil {
		t.Fatal("zero observation accepted")
	}
}

func TestRuntimeProfileOwnsCopiedInventoryAndExactIssuerBinding(t *testing.T) {
	now := time.Unix(1800000000, 0).UTC()
	binding := membershipProfile(now)
	assignments, records := membershipInputs(now)
	otherRecord := records[0]
	otherRecord.NodeID, otherRecord.RecordDigest, otherRecord.PublicKey = [32]byte{20}, [32]byte{21}, [32]byte{22}
	otherAssignment := assignments[0]
	otherAssignment.NodeID, otherAssignment.RecordDigest = otherRecord.NodeID, otherRecord.RecordDigest
	assignments = append(assignments, otherAssignment)
	records = append(records, otherRecord)
	// Profile authority survives one unavailable member. Issuance checks that
	// issuer's live assignment separately at its effect points.
	records[0].ValidUntil = now.Add(-time.Second)
	membership, err := network.BindMembership(binding, assignments, records)
	if err != nil {
		t.Fatal(err)
	}
	epoch := network.EpochFacts{Network: binding.Network, Number: binding.Epoch, Digest: binding.EpochDigest,
		ValidFrom: binding.NotBefore, ValidUntil: binding.NotAfter}
	profile := network.ProfileFacts{ProfileBinding: binding, IssuanceAuthorityKey: [32]byte{7},
		IssuerNodeID: records[0].NodeID, IssuerDutyGeneration: records[0].Generation,
		TokenKeys: []network.TokenKey{{WindowStart: now, Class: 1, SPKI: [346]byte{1}}}}
	for _, defect := range []string{"issuer", "duty", "authority", "missing keys", "unbounded keys"} {
		t.Run(defect, func(t *testing.T) {
			invalid := profile
			switch defect {
			case "issuer":
				invalid.IssuerNodeID = [32]byte{99}
			case "duty":
				invalid.IssuerDutyGeneration++
			case "authority":
				invalid.IssuanceAuthorityKey = [32]byte{}
			case "missing keys":
				invalid.TokenKeys = nil
			case "unbounded keys":
				invalid.TokenKeys = make([]network.TokenKey, 19)
			}
			if _, err := network.BindAcceptedState(epoch, binding.Generation, invalid, membership); err == nil {
				t.Fatal("unbound public profile accepted")
			}
		})
	}
	accepted, err := network.BindAcceptedState(epoch, binding.Generation, profile, membership)
	if err != nil {
		t.Fatal(err)
	}
	profile.TokenKeys[0].SPKI[0] = 99
	profile.TokenKeys[0].Class = 3
	observed, err := network.ConfirmTime(network.ClockEvidence{Wall: now, Independent: now}, 0)
	if err != nil {
		t.Fatal(err)
	}
	view, err := accepted.Observe(observed)
	if err != nil {
		t.Fatal(err)
	}
	public := view.Profile()
	if public.ProfileBinding != binding || public.IssuerNodeID != records[0].NodeID ||
		public.IssuerDutyGeneration != records[0].Generation || public.TokenKeys[0].SPKI[0] != 1 || public.TokenKeys[0].Class != 1 {
		t.Fatal("caller mutation changed accepted profile or its signed interval")
	}
	public.TokenKeys[0].SPKI[0] = 77
	public.TokenKeys = append(public.TokenKeys, network.TokenKey{})
	if again := view.Profile(); len(again.TokenKeys) != 1 || again.TokenKeys[0].SPKI[0] != 1 {
		t.Fatal("returned inventory aliases accepted authority")
	}
	if _, err := view.Member(profile.IssuerNodeID, now); err == nil {
		t.Fatal("expired issuer became a live member")
	}
	if _, err := view.Member(records[1].NodeID, now); err != nil {
		t.Fatalf("issuer expiry retired independent member: %v", err)
	}
}
