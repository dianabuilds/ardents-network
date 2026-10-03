package authority

import (
	"errors"
	"github.com/dianabuilds/ardents-network/internal/admission"
	"github.com/dianabuilds/ardents-network/internal/admission/receiving"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

// TokenVerifier authenticates a selected-profile role token for
// forwarding and control duties. Host reservation and spend stay with their
// respective duty owners.
func (source Source) TokenVerifier(receiver route.ClosedRoleReceiver, clock func() time.Time) route.ClosedAdmissionVerifier {
	return func(input route.ClosedAdmissionVerification) (route.ClosedAdmissionApproval, error) {
		if source.CurrentProfile == nil || clock == nil {
			return route.ClosedAdmissionApproval{}, errors.New("closed forwarding token is unavailable")
		}
		profile, available := source.CurrentProfile()
		if !available {
			return route.ClosedAdmissionApproval{}, errors.New("closed forwarding token is unavailable")
		}
		window, err := receiving.VerifyToken(profile, receiving.Receiver{NetworkID: receiver.NetworkID, StateGeneration: receiver.StateGeneration,
			StateDigest: receiver.StateDigest, ProfileDigest: receiver.ProfileDigest, NodeID: receiver.NodeID, DutyGeneration: receiver.DutyGeneration},
			admission.Class(input.Class), input.Token, clock())
		if err != nil {
			return route.ClosedAdmissionApproval{}, errors.New("closed forwarding token is unavailable")
		}
		return route.ClosedAdmissionApproval{Window: window}, nil
	}
}
