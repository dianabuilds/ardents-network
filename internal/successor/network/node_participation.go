package network

import (
	"crypto/sha256"
	"errors"
	"time"
)

// NodeParticipationFacts describe an already authenticated assignment. Local
// restriction derivation grants no assignment and cannot extend either bound.
type NodeParticipationFacts struct {
	Identity [32]byte
	// FamilyID is the authenticated public family digest when the retained
	// membership does not expose its original declaration. Supplying both
	// requires exact agreement, never hashing the digest a second time.
	FamilyID                [32]byte
	Family, Assignment      string
	EpochUntil, RecordUntil time.Time
}

func (facts NodeParticipationFacts) Restriction(phase string) (LocalRestriction, error) {
	if facts.Family == "" && facts.FamilyID == [32]byte{} || facts.Assignment == "" || facts.EpochUntil.IsZero() || facts.RecordUntil.IsZero() ||
		(phase != "prepared" && phase != "quarantined" && phase != "live") {
		return LocalRestriction{}, errors.New("local Node participation is invalid")
	}
	family := facts.FamilyID
	if facts.Family != "" {
		declared := sha256.Sum256([]byte(facts.Family))
		if family != [32]byte{} && family != declared {
			return LocalRestriction{}, errors.New("local Node family declaration differs")
		}
		family = declared
	}
	class := "node-duty"
	switch facts.Assignment {
	case "rendezvous":
		class = "route-rendezvous"
	case "introduction":
		class = "route-introduction"
	case "transit-issuance":
		class = "transit-issuance"
	}
	until := facts.EpochUntil
	if facts.RecordUntil.Before(until) {
		until = facts.RecordUntil
	}
	return LocalRestriction{Identity: facts.Identity, Family: family, Class: class, Phase: phase, NotAfter: until}, nil
}
