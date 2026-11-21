package network

import (
	"errors"
	"time"
)

// RetainedDuty identifies the assignment held by a local execution owner. It
// cannot make that assignment current or select a replacement participant.
type RetainedDuty struct {
	Epoch                                EpochFacts
	Generation                           [32]byte
	NodeID, PublicKey, FamilyID          [32]byte
	RecordGeneration                     uint64
	RecordValidFrom, RecordValidUntil    time.Time
	Assignment, Endpoint, CarrierProfile string
}

// RetainDuty copies one live authenticated assignment from this observation.
// Consumers retain this binding and recheck it against future observations;
// they never reconstruct an Epoch from a separate diagnostic read.
func (view RuntimeView) RetainDuty(id [32]byte, now time.Time) (RetainedDuty, error) {
	member, err := view.Member(id, now)
	if err != nil {
		return RetainedDuty{}, err
	}
	return RetainedDuty{Epoch: view.accepted.epoch, Generation: view.accepted.profile.Generation,
		NodeID: member.NodeID, PublicKey: member.PublicKey, FamilyID: member.FamilyID,
		RecordGeneration: member.DutyGeneration, RecordValidFrom: member.ValidFrom, RecordValidUntil: member.ValidUntil,
		Assignment: member.Assignment, Endpoint: member.Endpoint, CarrierProfile: member.CarrierProfile}, nil
}

func (view RuntimeView) MatchDuty(duty RetainedDuty, now time.Time) error {
	if err := view.Check(now); err != nil {
		return err
	}
	if now.Before(view.ObservedAt()) {
		now = view.ObservedAt()
	}
	profile := view.accepted.profile
	if duty.Generation != profile.Generation || duty.Epoch.Network != profile.Network || duty.Epoch.Digest != profile.EpochDigest ||
		duty.Epoch.Number != profile.Epoch ||
		now.Before(duty.Epoch.ValidFrom) || !now.Before(duty.Epoch.ValidUntil) || now.Before(duty.RecordValidFrom) || !now.Before(duty.RecordValidUntil) ||
		!duty.Epoch.ValidFrom.Equal(view.accepted.epoch.ValidFrom) || !duty.Epoch.ValidUntil.Equal(view.accepted.epoch.ValidUntil) {
		return errors.New("network observation does not match retained duty")
	}
	member, err := view.Member(duty.NodeID, now)
	if err != nil || member.DutyGeneration != duty.RecordGeneration || member.PublicKey != duty.PublicKey || member.FamilyID != duty.FamilyID ||
		member.Assignment != duty.Assignment || member.Endpoint != duty.Endpoint || member.CarrierProfile != duty.CarrierProfile ||
		!member.ValidFrom.Equal(duty.RecordValidFrom) || !member.ValidUntil.Equal(duty.RecordValidUntil) {
		return errors.New("network local duty changed")
	}
	return nil
}
