package funding

import (
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/successor/network"
)

// These detached facts describe one observation, never a live permit.
func NetworkFacts(view network.RuntimeView) (admission.AuthorityFacts, time.Time, error) {
	profile := view.Profile()
	facts := admission.AuthorityFacts{NetworkID: profile.Network, StateGeneration: profile.Generation,
		StateDigest: profile.EpochDigest, Digest: profile.Digest, IssuanceAuthorityKey: profile.IssuanceAuthorityKey,
		IssuerNodeID: profile.IssuerNodeID, IssuerDutyGeneration: profile.IssuerDutyGeneration, Epoch: profile.Epoch,
		NotBefore: profile.NotBefore, NotAfter: profile.NotAfter, TokenKeyCount: uint8(len(profile.TokenKeys))}
	for index, key := range profile.TokenKeys {
		facts.TokenKeys[index] = admission.TokenKey{WindowStart: key.WindowStart, Class: key.Class, SPKI: key.SPKI}
	}
	now := view.ObservedAt()
	return facts, now, facts.ValidateAt(now)
}

// Validate the pure draft against its same freshly observed Network view.
// Callers must still enter the actual owners' fresh effect and handoff gates.
func ValidateIntent(view network.RuntimeView, intent stock.IssuanceIntent) error {
	facts, now, err := NetworkFacts(view)
	if err != nil {
		return err
	}
	member, err := view.Member(facts.IssuerNodeID, now)
	if err != nil || member.DutyGeneration != facts.IssuerDutyGeneration || member.RoleDomain != 2 || member.Subrole != 6 ||
		!now.Before(intent.Deadline) || intent.Deadline.After(member.NotAfter()) || intent.Deadline.After(facts.NotAfter) || intent.Selection.ProfileDigest != facts.Digest {
		return errors.Join(errors.New("network issuance selection is unavailable"), err)
	}
	for _, challenge := range intent.Challenges {
		if challenge.NetworkID != facts.NetworkID || challenge.ProfileDigest != facts.Digest || challenge.IssuerNodeID != facts.IssuerNodeID {
			return errors.New("network issuance challenge binding differs")
		}
		receiver, err := view.Member(challenge.ReceiverNodeID, now)
		if err != nil || receiver.DutyGeneration != challenge.ReceiverDutyGeneration || intent.Deadline.After(receiver.NotAfter()) {
			return errors.Join(errors.New("network issuance recipient is unavailable"), err)
		}
	}
	return nil
}
