package network

import (
	"errors"
	"time"
)

var (
	ErrParticipationRecordLimit   = errors.New("local role record limit exceeded")
	ErrParticipationProducerLimit = errors.New("local role producer limit exceeded")
	ErrParticipationConflict      = errors.New("local role duty conflicts with retained state")
	ErrSourceExposureExhausted    = errors.New("direct-source exposure set is full")
)

// LocalRestriction is an installation-local exclusion, not global membership.
// Class and Phase retain the persisted vocabulary, including retired classes
// needed to recover old roots. A live Source requires producer-owned release.
type LocalRestriction struct {
	Identity, Family [32]byte
	Class, Phase     string
	NotAfter         time.Time
}

type ParticipationFact struct {
	Producer    [32]byte
	Restriction LocalRestriction
}

// LocalParticipation is a separate consistency owner from accepted Epochs.
// Replace produces a copied proposal; only its application's durable commit
// installs it. Neither this value nor the root journal grants Node authority.
type LocalParticipation struct{ records []ParticipationFact }

func RestoreLocalParticipation(records []ParticipationFact) (LocalParticipation, error) {
	if err := validateParticipation(records); err != nil {
		return LocalParticipation{}, err
	}
	return LocalParticipation{records: append([]ParticipationFact(nil), records...)}, nil
}

func (participation LocalParticipation) Records() []ParticipationFact {
	return append([]ParticipationFact(nil), participation.records...)
}

// Replace changes exactly one producer, pruning only expired time-held facts
// of other producers. Another producer can never expire or remove a live Source.
// The application calls release/replacement only after dependent work joins.
func (participation LocalParticipation) Replace(producer [32]byte, restrictions []LocalRestriction, now time.Time) (LocalParticipation, error) {
	if producer == [32]byte{} {
		return LocalParticipation{}, errors.New("local role replacement is invalid")
	}
	if len(restrictions) > 64 {
		return LocalParticipation{}, ErrParticipationRecordLimit
	}
	next := make([]ParticipationFact, 0, len(participation.records)+len(restrictions))
	for _, retained := range participation.records {
		if retained.Producer != producer && retained.Restriction.effective(now) {
			next = append(next, retained)
		}
	}
	for _, restriction := range restrictions {
		if !validRestriction(restriction) || !restriction.effective(now) {
			return LocalParticipation{}, errors.New("local role duty is invalid")
		}
		next = append(next, ParticipationFact{Producer: producer, Restriction: restriction})
	}
	var sources int
	for _, fact := range next {
		if fact.Restriction.Class == "direct-source" {
			sources++
		}
	}
	if sources > 64 {
		return LocalParticipation{}, ErrSourceExposureExhausted
	}
	return RestoreLocalParticipation(next)
}

func (participation LocalParticipation) Conflicts(identity, family [32]byte, now time.Time) bool {
	for _, fact := range participation.records {
		restriction := fact.Restriction
		if restriction.effective(now) && restriction.Class != "ordinary-initiator" &&
			(identity != [32]byte{} && restriction.Identity == identity || family != [32]byte{} && restriction.Family == family) {
			return true
		}
	}
	return false
}

func (restriction LocalRestriction) effective(now time.Time) bool {
	return restriction.Class == "direct-source" && restriction.Phase == "live" || now.Unix() < restriction.NotAfter.Unix()
}

func validRestriction(restriction LocalRestriction) bool {
	return restriction.Identity != [32]byte{} && restriction.Family != [32]byte{} && restriction.NotAfter.Unix() > 0 &&
		validRestrictionClass(restriction.Class) && validRestrictionPhase(restriction.Phase)
}

func validRestrictionClass(class string) bool {
	switch class {
	case "ordinary-initiator", "direct-source", "route-interior", "route-rendezvous", "route-responder", "route-introduction", "destination-resolution", "transit-issuance", "node-duty":
		return true
	default:
		return false
	}
}

func validRestrictionPhase(phase string) bool {
	return phase == "exposed" || phase == "prepared" || phase == "quarantined" || phase == "live"
}

func validateParticipation(records []ParticipationFact) error {
	if len(records) > 64 {
		return ErrParticipationRecordLimit
	}
	producers, seen := make(map[[32]byte]bool), make(map[[3][32]byte]bool)
	for _, fact := range records {
		if fact.Producer == [32]byte{} || !validRestriction(fact.Restriction) {
			return errors.New("local role record is invalid")
		}
		producers[fact.Producer] = true
		key := [3][32]byte{fact.Producer, fact.Restriction.Identity, fact.Restriction.Family}
		if seen[key] {
			return errors.New("local role record is duplicated")
		}
		seen[key] = true
	}
	if len(producers) > 16 {
		return ErrParticipationProducerLimit
	}
	for first, a := range records {
		if a.Restriction.Class == "ordinary-initiator" {
			continue
		}
		for _, b := range records[first+1:] {
			if b.Restriction.Class == "ordinary-initiator" {
				continue
			}
			if a.Restriction.Identity == b.Restriction.Identity || a.Restriction.Family == b.Restriction.Family {
				if a.Producer == b.Producer && a.Restriction.Class == "direct-source" && b.Restriction.Class == "direct-source" {
					continue
				}
				return ErrParticipationConflict
			}
		}
	}
	return nil
}
