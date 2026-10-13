package network

import (
	"crypto/sha256"
	"testing"
	"time"
)

func TestNodeRestrictionRetainsAuthenticatedFamilyDigest(t *testing.T) {
	end := time.Unix(1800000000, 0).UTC()
	family := sha256.Sum256([]byte("original-family"))
	facts := NodeParticipationFacts{Identity: [32]byte{1}, FamilyID: family, Assignment: "initiator", EpochUntil: end, RecordUntil: end.Add(time.Minute)}
	restriction, err := facts.Restriction("prepared")
	if err != nil || restriction.Family != family || restriction.Class != "node-duty" || !restriction.NotAfter.Equal(end) {
		t.Fatalf("authenticated family changed: %+v %v", restriction, err)
	}
	facts.Family = "another-family"
	if _, err := facts.Restriction("live"); err == nil {
		t.Fatal("mismatched declaration substituted authenticated family")
	}
	facts.Family = "original-family"
	restriction, err = facts.Restriction("live")
	if err != nil || restriction.Family != family {
		t.Fatal("exact family agreement refused", err)
	}
}
