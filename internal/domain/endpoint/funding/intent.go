package funding

import (
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/admission"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/domain/network"
)

// NetworkFacts returns detached facts for one observation, never a live permit.
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

// ValidateIntent checks the pure draft against its same freshly observed Network view.
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

// PresentationGuard validates a presentation against one fresh Network view.
func PresentationGuard(current func() (network.RuntimeView, error)) func(stock.Presentation) error {
	return func(p stock.Presentation) error {
		view, err := current()
		if err != nil {
			return err
		}
		facts, now, err := NetworkFacts(view)
		if err != nil {
			return err
		}
		member, err := view.Member(p.RecipientNodeID, now)
		if err != nil || member.DutyGeneration != p.RecipientDutyGeneration || facts.NetworkID != p.NetworkID || facts.StateGeneration != p.StateGeneration || facts.StateDigest != p.StateDigest || facts.Digest != p.ProfileDigest || !now.Before(p.Deadline) || p.Deadline.After(member.NotAfter()) || p.Deadline.After(facts.NotAfter) {
			return errors.Join(errors.New("network presentation binding is unavailable"), err)
		}
		return nil
	}
}
