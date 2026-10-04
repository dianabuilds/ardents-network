package network_test

import (
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network"
)

func membershipInputs(now time.Time) ([]network.Assignment, []network.NodeRecord) {
	record := network.NodeRecord{NodeID: [32]byte{1}, PublicKey: [32]byte{2}, FamilyID: [32]byte{3}, RecordDigest: [32]byte{4},
		Generation: 1, Capacity: 1, Assignment: "initiator", CarrierProfile: "ardents-carrier-tcp-tls-v2",
		ValidFrom: now.Add(-time.Minute), ValidUntil: now.Add(time.Minute), AssignmentNotAfter: now.Add(time.Minute)}
	return []network.Assignment{{NodeID: record.NodeID, RecordDigest: record.RecordDigest, Generation: 1, RoleDomain: 1, Subrole: 1}}, []network.NodeRecord{record}
}

func TestMembershipKeyAmbiguitySurvivesRoleAndExpiryFiltering(t *testing.T) {
	now := time.Unix(1800000000, 0).UTC()
	for _, expired := range []bool{false, true} {
		assignments, records := membershipInputs(now)
		other := records[0]
		other.NodeID, other.Assignment = [32]byte{5}, "rendezvous"
		if expired {
			other.ValidUntil = now
		}
		records = append(records, other)
		assignments = append(assignments, network.Assignment{NodeID: other.NodeID, RecordDigest: other.RecordDigest, Generation: 1, RoleDomain: 2, Subrole: 6})
		bound, err := network.BindMembership(membershipProfile(now), assignments, records)
		if err != nil {
			t.Fatal(err)
		}
		bound = bound.Observe(now)
		if _, err := bound.MemberByKey(records[0].PublicKey, now); err == nil {
			t.Fatal("role or expiry hid ambiguous key")
		}
		if _, err := bound.Member(records[0].NodeID, now); err != nil {
			t.Fatalf("unrelated member became unavailable: %v", err)
		}
	}
}

func TestMembershipOwnsBindingAndCopiedObservation(t *testing.T) {
	now := time.Unix(1800000000, 0).UTC()
	assignments, records := membershipInputs(now)
	bound, err := network.BindMembership(membershipProfile(now), assignments, records)
	if err != nil {
		t.Fatal(err)
	}
	id, key := records[0].NodeID, records[0].PublicKey
	if _, err := bound.Member(id, now); err == nil {
		t.Fatal("unobserved facts became current")
	}
	observed := bound.Observe(now)
	member, err := observed.Member(id, now)
	if err != nil {
		t.Fatal(err)
	}
	byKey, err := observed.MemberByKey(key, now)
	if err != nil || byKey != member {
		t.Fatalf("identity and key disagree: %v", err)
	}
	assignments[0].NodeID, records[0].PublicKey = [32]byte{}, [32]byte{}
	inventory := observed.Members()
	inventory[0].PublicKey = [32]byte{}
	if unchanged, err := observed.MemberByKey(key, now); err != nil || unchanged != member {
		t.Fatal("caller mutation changed membership")
	}
	expired := bound.Observe(member.NotAfter())
	if _, err := expired.Member(id, now); err == nil {
		t.Fatal("earlier caller time revived expired assignment")
	}
	for _, missing := range [][32]byte{{}, {255}} {
		if _, err := observed.Member(missing, now); err == nil {
			t.Fatal("missing identity selected a fallback")
		}
		if _, err := observed.MemberByKey(missing, now); err == nil {
			t.Fatal("missing key selected a fallback")
		}
	}
}

func TestMembershipRefusesMismatchedAndAmbiguousEvidence(t *testing.T) {
	now := time.Unix(1800000000, 0).UTC()
	for _, defect := range []string{"digest", "generation", "role", "carrier", "missing", "duplicate record", "duplicate assignment"} {
		t.Run(defect, func(t *testing.T) {
			assignments, records := membershipInputs(now)
			switch defect {
			case "digest":
				assignments[0].RecordDigest = [32]byte{99}
			case "generation":
				assignments[0].Generation++
			case "role":
				assignments[0].RoleDomain = 4
			case "carrier":
				records[0].CarrierProfile = "ardents-carrier-tcp-tls-v1"
			case "missing":
				records = nil
			case "duplicate record":
				records = append(records, records[0])
			case "duplicate assignment":
				assignments = append(assignments, assignments[0])
			}
			if _, err := network.BindMembership(membershipProfile(now), assignments, records); err == nil {
				t.Fatal("invalid membership accepted")
			}
		})
	}
}

func membershipProfile(now time.Time) network.ProfileBinding {
	return network.ProfileBinding{Network: [32]byte{1}, Generation: [32]byte{2}, EpochDigest: [32]byte{3}, Digest: [32]byte{4},
		Epoch: 1, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Minute)}
}
