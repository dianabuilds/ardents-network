package duty

import (
	"errors"
	networkdomain "github.com/dianabuilds/ardents-network/internal/successor/network"
	"time"
)

func participationHistory(records []dutyRecord) (networkdomain.LocalParticipation, error) {
	facts := make([]networkdomain.ParticipationFact, len(records))
	for index, record := range records {
		facts[index] = networkdomain.ParticipationFact{Producer: record.Producer, Restriction: networkdomain.LocalRestriction{
			Identity: record.Identity, Family: record.Family, Class: record.Class, Phase: record.State, NotAfter: time.Unix(record.NotAfter, 0).UTC()}}
	}
	history, err := networkdomain.RestoreLocalParticipation(facts)
	return history, participationError(err)
}

func proposeParticipation(records []dutyRecord, producer [32]byte, duties []Duty, now time.Time) ([]dutyRecord, error) {
	history, err := participationHistory(records)
	if err != nil {
		return nil, err
	}
	restrictions := make([]networkdomain.LocalRestriction, len(duties))
	for index, duty := range duties {
		restrictions[index] = networkdomain.LocalRestriction{Identity: duty.Identity, Family: duty.Family, Class: duty.Class, Phase: duty.State, NotAfter: duty.NotAfter}
	}
	next, err := history.Replace(producer, restrictions, now)
	if err != nil {
		return nil, participationError(err)
	}
	facts := next.Records()
	projected := make([]dutyRecord, len(facts))
	for index, fact := range facts {
		projected[index] = dutyRecord{Producer: fact.Producer, Identity: fact.Restriction.Identity,
			Family: fact.Restriction.Family, Class: fact.Restriction.Class, State: fact.Restriction.Phase, NotAfter: fact.Restriction.NotAfter.Unix()}
	}
	return projected, nil
}

func participationConflict(records []dutyRecord, identity, family [32]byte, now time.Time) (bool, error) {
	history, err := participationHistory(records)
	if err != nil {
		return false, err
	}
	return history.Conflicts(identity, family, now), nil
}

func participationError(err error) error {
	switch {
	case errors.Is(err, networkdomain.ErrParticipationRecordLimit):
		return ErrLocalRoleRecordLimit
	case errors.Is(err, networkdomain.ErrParticipationProducerLimit):
		return ErrLocalRoleProducerLimit
	case errors.Is(err, networkdomain.ErrParticipationConflict):
		return ErrLocalRoleConflict
	case errors.Is(err, networkdomain.ErrSourceExposureExhausted):
		return ErrInstallationSourceExhausted
	default:
		return err
	}
}

func nodeParticipationRestriction(participation NodeParticipation, phase NodePhase) (Duty, error) {
	facts := networkdomain.NodeParticipationFacts{Identity: participation.Identity, Family: participation.Family, Assignment: participation.Assignment,
		EpochUntil: participation.EpochUntil, RecordUntil: participation.RecordUntil}
	restriction, err := facts.Restriction(string(phase))
	if err != nil {
		return Duty{}, err
	}
	return Duty{Identity: restriction.Identity, Family: restriction.Family, Class: restriction.Class, State: restriction.Phase, NotAfter: restriction.NotAfter}, nil
}
