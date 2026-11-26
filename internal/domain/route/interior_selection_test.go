package route

import (
	"testing"
	"time"
)

func TestInteriorRetainsDeterministicPairAndOriginalHorizon(t *testing.T) {
	now := time.Unix(1900000000, 0).UTC()
	member := func(id byte) Member {
		return Member{NodeID: [32]byte{id}, PublicKey: [32]byte{id + 10}, FamilyID: [32]byte{id + 20}, RecordDigest: [32]byte{id + 30}, DutyGeneration: 1, Domain: 1, NotAfter: now.Add(time.Hour)}
	}
	entries := [2]Member{member(1), member(2)}
	candidates := []Member{member(3), member(4), member(5)}
	first, err := SelectInterior(candidates, entries, now, 1)
	if err != nil {
		t.Fatal(err)
	}
	second, err := SelectInterior([]Member{candidates[2], candidates[0], candidates[1]}, entries, now, 1)
	if err != nil || second != first {
		t.Fatal("candidate order changed retained derivation", err)
	}
	if err := first.Check(candidates, entries, first.NotAfter, 1); err == nil {
		t.Fatal("expired pair accepted")
	}
	if err := first.Check(candidates, entries, now.Add(-time.Second), 1); err == nil {
		t.Fatal("time regression accepted")
	}
	missing := []Member{first.Members[1]}
	if err := first.Check(missing, entries, now.Add(time.Second), 1); err == nil {
		t.Fatal("missing retained first member replaced")
	}
}

func TestPurposeAssignmentMatrix(t *testing.T) {
	for purpose := uint8(1); purpose <= 8; purpose++ {
		for domain := uint8(1); domain <= 4; domain++ {
			for subrole := uint8(1); subrole <= 6; subrole++ {
				expected := purpose == 1 && domain == 2 && subrole == 6 || (purpose == 2 || purpose == 3) && domain == 2 && subrole == 5 || (purpose == 4 || purpose == 5) && domain == 4 && subrole == 3 || purpose == 6 && domain == 2 && subrole == 4 || purpose == 7 && (domain == 1 || domain == 3 || domain == 4) && (subrole == 1 || subrole == 2)
				if PurposePermitsDuty(purpose, domain, subrole) != expected {
					t.Fatalf("purpose %d, domain %d, subrole %d", purpose, domain, subrole)
				}
			}
		}
	}
}
