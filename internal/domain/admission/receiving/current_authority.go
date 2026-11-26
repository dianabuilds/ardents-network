package receiving

import (
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/admission"
	"github.com/dianabuilds/ardents-network/internal/domain/network"
)

// ObserveCurrentAuthority binds the receiver duty to an observed Network view.
func ObserveCurrentAuthority(view network.RuntimeView, facts admission.AuthorityFacts, now time.Time, receiver Receiver, end time.Time) (Observation, error) {
	member, err := view.Member(receiver.NodeID, now)
	if err != nil || member.DutyGeneration != receiver.DutyGeneration || facts.NetworkID != receiver.NetworkID ||
		facts.StateGeneration != receiver.StateGeneration || facts.StateDigest != receiver.StateDigest || facts.Digest != receiver.ProfileDigest {
		return Observation{}, errors.Join(errors.New("network receiver duty is unavailable"), err)
	}
	if member.NotAfter().Before(end) {
		end = member.NotAfter()
	}
	if facts.NotAfter.Before(end) {
		end = facts.NotAfter
	}
	return Observation{Profile: facts, Receiver: receiver, Now: now, NotAfter: end}, nil
}
